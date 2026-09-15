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

// ThreadRepositoryはsqlc生成のクエリ経由でthreadsを読み書きします。
type ThreadRepository struct {
	reader *query.Queries
	writer *query.Queries
}

// NewThreadRepositoryは、データベースの読み取り用プールで読み、書き込み用プールで
// 書くThreadRepositoryを生成します。
func NewThreadRepository(db *database.DB) *ThreadRepository {
	return &ThreadRepository{reader: query.New(db.Reader), writer: query.New(db.Writer)}
}

// WithTxはqueriesをtx内で実行する新しいThreadRepositoryを返し、UseCaseが
// 本リポジトリを自身のトランザクションに参加させられるようにします。レシーバ自身は
// 変更しません。
func (r *ThreadRepository) WithTx(tx *sql.Tx) *ThreadRepository {
	q := r.writer.WithTx(tx)
	return &ThreadRepository{reader: q, writer: q}
}

// FindByIDは指定idのスレッドを返し、存在しない場合は (nil, nil) を返します。
// slugではなくidで引くのは、タイトルが編集されうるためです。未存在は正常な
// ルックアップ結果であり — /t/{id} が404を返すと判断する手立てです — エラーでは
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

// ListByBoardIDは掲示板のスレッドを、最後に投稿されたものから順に返します
// (時刻が同じスレッドも順序が固定されるようidで同着を解き、後のものを先に置きます)。
// 並び順は非正規化されたlast_posted_atから得るため、一覧の1行はpostsにまったく
// 触れずに描けます。
//
// 管理者が非公開にしたスレッドは落とします。掲示板を見て回る訪問者が出会うべきで
// ないためです。idを既に持っている呼び出し元はFindByIDやListByIDsから引き続き
// 届きます。
//
// 一覧全体を1つのSELECTにしているのは、落とす行を同じ文の中で落とすためです。
// 非公開のスレッドは既にそうしており、ミュート (M3) もスレッドと人をSQLで除外し、
// ページネーション (M4) はその除外の後ろにLIMITを置きます。複数のクエリを束ねて
// Go側で絞り込む形にすると、そのうち何件が落ちたかによって1ページの件数が
// 揺れてしまいます。
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

// ListRecentPerBoardは、各掲示板について最後に投稿されたものからperBoard件の
// スレッドを返します。掲示板はコミュニティが並べた順、各掲示板のスレッドは最後に投稿
// されたものから順に並びます。まだ誰も書き込んでいない掲示板は1行も持たないため、
// 呼び出し側は描きたい掲示板の一覧と突き合わせるのであって、結果から掲示板の集合を
// 読み取るのではありません。
//
// 管理者が非公開にしたスレッドは落とします。落とすのはperBoard件を切り出した後では
// なく前であるため、掲示板は隠れたスレッドに枠を取られることなく、公開されている
// 最新のperBoard件を出します。
//
// この一覧はすべての掲示板を1つの文で取得します。コミュニティのホームは掲示板ごとに
// 数件のスレッドを見せるため、掲示板ごとにクエリを投げる形はサイドバーが1つの行の集合から
// 描いている一覧に比例して増えていきます。各掲示板の検索では、最新の公開スレッドへ
// 到達するまでに非公開の候補も読み取ります。
// 先にすべてのスレッドへ順位を付けて上位だけを残す形 (threadsに対する窓関数) では、
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

// CreateThreadInputはスレッドの作成に必要な属性を保持します。idとタイムスタンプは
// DB側で採番され、最終投稿の非正規化列は既定値で始まります。行を挿入する時点では
// スレッドにまだ投稿が無いため、最初の投稿ができた時点でUpdateLastPostがそれらを
// 埋めます。
type CreateThreadInput struct {
	BoardID  model.BoardID
	UserID   *model.UserID
	Title    string
	Language model.ThreadLanguage
}

// Createはスレッドを挿入し、DBが採番したidとタイムスタンプを設定した状態で
// 返します。
//
// 挿入の前に、言語がスレッドを書ける言語の集合に含まれることを検査します。理由は
// model.ThreadLanguage.IsValidが記すとおりで、この列自身は値を列挙しないため、集合の外の
// 値を拒否するのがこの書き込みになります。集合外の値はThreadLanguageOtherと同じく
// 表示言語に解決しないため、Presentation層では両者を区別できず、実際の値に合うバッジと
// lang属性ではなく「その他」を与えることになります。
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

// UpdateThreadLastPostInputはスレッドが持つ投稿の非正規化された姿を保持します。
// LastPostIDがポインタでないのは、この更新が投稿の書き込みによって起きるもので、その
// 投稿は存在するためです。列がnullableなのは、最終投稿の削除がそれを解除できるように
// するためだけです。
type UpdateThreadLastPostInput struct {
	PostsCount   int
	LastPostID   model.PostID
	LastPostedAt time.Time
}

// UpdateLastPostは、スレッドが自身の投稿について保持する3つの列を書き込みます。
// 3つをまとめて設定するのは、それらが1つの事実 — どの投稿がそのスレッドの最新で、
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

// ListByIDsは指定したidのうち、まだ存在するスレッドをid順で返し、idの集合を持つ
// 呼び出し元が1クエリでそれらを解決できるようにします。
//
// 非公開のスレッドも返します。一覧がそれらを落とすのは、コミュニティを見て回る訪問者が
// 出会うべきでないためですが、スレッドのidを既に持っている呼び出し元が見ているのは、その
// スレッドについての記録 — 操作の対象となったスレッドを名指す操作履歴 — であり、何も返って
// こない行はその記録を読めなくします。公開と非公開の区別はUnpublishedAtから呼び出し元が
// 行います。
//
// 空のidスライスに対してはクエリを発行せず空のスライスを返します。スレッドを引く手がかりが
// 無いためです。
func (r *ThreadRepository) ListByIDs(ctx context.Context, ids []model.ThreadID) ([]*model.Thread, error) {
	if len(ids) == 0 {
		return []*model.Thread{}, nil
	}

	rawIDs := make([]int64, len(ids))
	for i, id := range ids {
		rawIDs[i] = int64(id)
	}

	rows, err := r.reader.ListThreadsByIDs(ctx, rawIDs)
	if err != nil {
		return nil, err
	}

	threads := make([]*model.Thread, len(rows))
	for i, row := range rows {
		threads[i] = r.toModel(row)
	}
	return threads, nil
}

// Lockはスレッドに管理者によるロックの時刻を打刻します。列を無条件に書くため、既に
// ある打刻を上書きする呼び出し元は、先に読んでいない呼び出し元です。モデレーションの操作が
// 対象とする状態は、それをコミットする書き込みトランザクションの中で読み、既にロック
// されているスレッドはそこで答えられ、本メソッドには届きません。
//
// 時刻はmoderation_logs.created_atと同じく、データベースの時計を使います。
func (r *ThreadRepository) Lock(ctx context.Context, id model.ThreadID) error {
	return r.writer.LockThread(ctx, int64(id))
}

// Unlockは管理者によるロックを外します。外すのはその列だけであり、上限到達
// (model.Thread.LockReasonsが列からではなくPostsCountから導くもの) はそのまま成り立ち
// ます。併せて満杯になっているスレッドは、解除された後もその理由で閉じたままです。
func (r *ThreadRepository) Unlock(ctx context.Context, id model.ThreadID) error {
	return r.writer.UnlockThread(ctx, int64(id))
}

// Unpublishはスレッドに管理者による非公開の時刻を打刻します。行はタイトルと投稿を
// 保ち、配下の投稿には1件ずつ印を付けません。スレッドを一覧から外し /t/{id} に答えるのは
// スレッド自身の印であるため、印を外せばスレッドは丸ごと元のとおりに戻ります。
func (r *ThreadRepository) Unpublish(ctx context.Context, id model.ThreadID) error {
	return r.writer.UnpublishThread(ctx, int64(id))
}

// toModelはquery.Threadをmodel.Threadに変換し、リポジトリの境界で生のidを
// 型付きの形に、保存書式の時刻をtime.Timeにキャストします。UserIDは作者のアカウント行が
// 物理削除された後にだけnilになり、論理退会ではスレッド行にidが残ります。LastPostIDは
// スレッドの最新の投稿が削除されるとnilになります。
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

		LockedAt:      sqlitetime.TimePtr(row.LockedAt),
		UnpublishedAt: sqlitetime.TimePtr(row.UnpublishedAt),

		CreatedAt: time.Time(row.CreatedAt),
		UpdatedAt: time.Time(row.UpdatedAt),
	}
}
