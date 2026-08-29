package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/query"
	"github.com/groobb/groobb/go/internal/sqlitetime"
)

// ThreadRepository reads and writes threads through sqlc-generated queries.
//
// [Ja] ThreadRepository は sqlc 生成のクエリ経由で threads を読み書きします。
type ThreadRepository struct {
	reader *query.Queries
	writer *query.Queries
}

// NewThreadRepository creates a ThreadRepository that reads through the
// database's read pool and writes through its write pool.
//
// [Ja] NewThreadRepository は、データベースの読み取り用プールで読み、書き込み用プールで
// 書く ThreadRepository を生成します。
func NewThreadRepository(db *database.DB) *ThreadRepository {
	return &ThreadRepository{reader: query.New(db.Reader), writer: query.New(db.Writer)}
}

// WithTx returns a new ThreadRepository whose queries run inside tx, so a
// UseCase can enlist this repository in its transaction. The receiver is left
// unchanged.
//
// [Ja] WithTx は queries を tx 内で実行する新しい ThreadRepository を返し、UseCase が
// 本リポジトリを自身のトランザクションに参加させられるようにします。レシーバ自身は
// 変更しません。
func (r *ThreadRepository) WithTx(tx *sql.Tx) *ThreadRepository {
	q := r.writer.WithTx(tx)
	return &ThreadRepository{reader: q, writer: q}
}

// FindByID returns the thread with the given id, or (nil, nil) when none
// exists. A thread is looked up by id rather than by a slug because its title
// can be edited. Absence is a normal lookup outcome — it is how /t/{id} learns
// to answer 404 — not an error.
//
// [Ja] FindByID は指定 id のスレッドを返し、存在しない場合は (nil, nil) を返します。
// slug ではなく id で引くのは、タイトルが編集されうるためです。未存在は正常な
// ルックアップ結果であり — /t/{id} が 404 を返すと判断する手立てです — エラーでは
// ありません。
func (r *ThreadRepository) FindByID(ctx context.Context, id model.ThreadID) (*model.Thread, error) {
	row, err := r.reader.GetThreadByID(ctx, int64(id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return r.toModel(row), nil
}

// ListByBoardID returns the board's threads with the most recently posted-in
// first (id breaking a tie so threads sharing a timestamp still come back in a
// fixed order, the more recent one first). The order comes from the
// denormalized last_posted_at, so a row of the list is rendered without
// touching posts at all.
//
// The whole list is one SELECT, and the rows it drops will be dropped inside
// that same statement: muting (M3) excludes threads and people in SQL, and
// pagination (M4) puts a LIMIT after that exclusion. Assembling the list from
// several queries and filtering it in Go would let the number of rows on a page
// vary with how many of them turned out to be muted.
//
// [Ja] ListByBoardID は掲示板のスレッドを、最後に投稿されたものから順に返します
// (時刻が同じスレッドも順序が固定されるよう id で同着を解き、後のものを先に置きます)。
// 並び順は非正規化された last_posted_at から得るため、一覧の 1 行は posts にまったく
// 触れずに描けます。
//
// 一覧全体を 1 つの SELECT にしているのは、落とす行を同じ文の中で落とすためです。
// ミュート (M3) はスレッドと人を SQL で除外し、ページネーション (M4) はその除外の後ろに
// LIMIT を置きます。複数のクエリを束ねて Go 側で絞り込む形にすると、そのうち何件が
// ミュート対象だったかによって 1 ページの件数が揺れてしまいます。
func (r *ThreadRepository) ListByBoardID(ctx context.Context, boardID model.BoardID) ([]*model.Thread, error) {
	rows, err := r.reader.ListThreadsByBoardID(ctx, int64(boardID))
	if err != nil {
		return nil, err
	}

	threads := make([]*model.Thread, len(rows))
	for i, row := range rows {
		threads[i] = r.toModel(row)
	}
	return threads, nil
}

// ListRecentPerBoard returns the perBoard most recently posted-in threads of
// every board, the boards in the order the community placed them and each
// board's threads with the most recently posted-in first. A board nobody has
// posted in yet contributes no row, so the caller pairs the result with the
// boards it means to draw rather than reading the set of boards out of it.
//
// The listing is one statement whose cost is set by the number of boards rather
// than the number of threads: the community's home page shows a few threads of
// each board, and a query per board would grow with a listing the sidebar
// already draws from a single row set. Ranking every thread first and keeping
// the top few (a window function over threads) would instead read the whole
// table on every visit to the page a signed-in visitor lands on.
//
// [Ja] ListRecentPerBoard は、各掲示板について最後に投稿されたものから perBoard 件の
// スレッドを返します。掲示板はコミュニティが並べた順、各掲示板のスレッドは最後に投稿
// されたものから順に並びます。まだ誰も書き込んでいない掲示板は 1 行も持たないため、
// 呼び出し側は描きたい掲示板の一覧と突き合わせるのであって、結果から掲示板の集合を
// 読み取るのではありません。
//
// この一覧は 1 つの文であり、その費用はスレッドの件数ではなく掲示板の数で決まります。
// コミュニティのホームは掲示板ごとに数件のスレッドを見せるため、掲示板ごとにクエリを
// 投げる形はサイドバーが 1 つの行の集合から描いている一覧に比例して増えていきます。
// 先にすべてのスレッドへ順位を付けて上位だけを残す形 (threads に対する窓関数) では、
// サインイン済みの訪問者が着地するページを開くたびにテーブル全体を読むことになります。
func (r *ThreadRepository) ListRecentPerBoard(ctx context.Context, perBoard int) ([]*model.Thread, error) {
	rows, err := r.reader.ListRecentThreadsPerBoard(ctx, int64(perBoard))
	if err != nil {
		return nil, err
	}

	threads := make([]*model.Thread, len(rows))
	for i, row := range rows {
		threads[i] = r.toModel(row)
	}
	return threads, nil
}

// CreateThreadInput holds the attributes needed to create a thread. id and the
// timestamps are assigned by the database, and the denormalized last-post
// columns start at their defaults: a thread has no post yet at the moment the
// row is inserted, so UpdateLastPost fills them in once the first post exists.
//
// [Ja] CreateThreadInput はスレッドの作成に必要な属性を保持します。id とタイムスタンプは
// DB 側で採番され、最終投稿の非正規化列は既定値で始まります。行を挿入する時点では
// スレッドにまだ投稿が無いため、最初の投稿ができた時点で UpdateLastPost がそれらを
// 埋めます。
type CreateThreadInput struct {
	BoardID  model.BoardID
	UserID   *model.UserID
	Title    string
	Language model.ThreadLanguage
}

// Create inserts a thread and returns it with the database-assigned id and
// timestamps populated.
//
// The language is checked against the set a thread may be written in before the
// insert, for the reason model.ThreadLanguage.IsValid documents: the column
// enumerates no values of its own, so this is the write that refuses one outside
// the set. A value outside it resolves to no display language just as
// ThreadLanguageOther does, so presentation could not distinguish the two and
// would label the value as other instead of giving it a truthful badge and lang
// attribute.
//
// [Ja] Create はスレッドを挿入し、DB が採番した id とタイムスタンプを設定した状態で
// 返します。
//
// 挿入の前に、言語がスレッドを書ける言語の集合に含まれることを検査します。理由は
// model.ThreadLanguage.IsValid が記すとおりで、この列自身は値を列挙しないため、集合の外の
// 値を拒否するのがこの書き込みになります。集合外の値は ThreadLanguageOther と同じく
// 表示言語に解決しないため、Presentation 層では両者を区別できず、実際の値に合うバッジと
// lang 属性ではなく「その他」を与えることになります。
func (r *ThreadRepository) Create(ctx context.Context, input CreateThreadInput) (*model.Thread, error) {
	if !input.Language.IsValid() {
		return nil, fmt.Errorf("スレッドの主言語が不正: language=%q", input.Language)
	}

	row, err := r.writer.CreateThread(ctx, query.CreateThreadParams{
		BoardID:  int64(input.BoardID),
		UserID:   rawAuthorID(input.UserID),
		Title:    input.Title,
		Language: string(input.Language),
	})
	if err != nil {
		return nil, err
	}
	return r.toModel(row), nil
}

// UpdateThreadLastPostInput holds the thread's denormalized view of its posts.
// LastPostID is not a pointer because the update is what a post being written
// triggers, and that post exists: the column is nullable only so that deleting
// the last post can clear it.
//
// [Ja] UpdateThreadLastPostInput はスレッドが持つ投稿の非正規化された姿を保持します。
// LastPostID がポインタでないのは、この更新が投稿の書き込みによって起きるもので、その
// 投稿は存在するためです。列が nullable なのは、最終投稿の削除がそれを解除できるように
// するためだけです。
type UpdateThreadLastPostInput struct {
	PostsCount   int
	LastPostID   model.PostID
	LastPostedAt time.Time
}

// UpdateLastPost writes the three columns a thread keeps about its posts. They
// are set together because they describe one fact — which post is the thread's
// latest, and how many there are — and a caller that updated only some of them
// would leave the board's thread list showing a count and a time that disagree.
//
// [Ja] UpdateLastPost は、スレッドが自身の投稿について保持する 3 つの列を書き込みます。
// 3 つをまとめて設定するのは、それらが 1 つの事実 — どの投稿がそのスレッドの最新で、
// 何件あるか — を表すためです。一部だけを更新する呼び出し元は、掲示板のスレッド一覧に
// 件数と時刻が食い違った行を残すことになります。
func (r *ThreadRepository) UpdateLastPost(ctx context.Context, id model.ThreadID, input UpdateThreadLastPostInput) error {
	lastPostID := int64(input.LastPostID)
	return r.writer.UpdateThreadLastPost(ctx, query.UpdateThreadLastPostParams{
		PostsCount:   int64(input.PostsCount),
		LastPostID:   &lastPostID,
		LastPostedAt: sqlitetime.Time(input.LastPostedAt),
		ID:           int64(id),
	})
}

// toModel converts a query.Thread row into a model.Thread, casting the raw ids
// into their typed forms and the stored timestamps back into time.Time at the
// repository boundary. UserID is nil only after the author's account row has
// been physically deleted; a logical withdrawal leaves the id in the thread
// row. LastPostID is nil once the thread's latest post has been deleted.
//
// [Ja] toModel は query.Thread を model.Thread に変換し、リポジトリの境界で生の id を
// 型付きの形に、保存書式の時刻を time.Time にキャストします。UserID は作者のアカウント行が
// 物理削除された後にだけ nil になり、論理退会ではスレッド行に id が残ります。LastPostID は
// スレッドの最新の投稿が削除されると nil になります。
func (r *ThreadRepository) toModel(row query.Thread) *model.Thread {
	var lastPostID *model.PostID
	if row.LastPostID != nil {
		id := model.PostID(*row.LastPostID)
		lastPostID = &id
	}
	return &model.Thread{
		ID:           model.ThreadID(row.ID),
		BoardID:      model.BoardID(row.BoardID),
		UserID:       typedAuthorID(row.UserID),
		Title:        row.Title,
		Language:     model.ThreadLanguage(row.Language),
		PostsCount:   int(row.PostsCount),
		LastPostID:   lastPostID,
		LastPostedAt: time.Time(row.LastPostedAt),
		CreatedAt:    time.Time(row.CreatedAt),
		UpdatedAt:    time.Time(row.UpdatedAt),
	}
}
