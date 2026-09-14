package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/query"
	"github.com/groobb/groobb/go/internal/sqlitetime"
)

// PostRepository reads and writes posts through sqlc-generated queries.
//
// [Ja] PostRepository は sqlc 生成のクエリ経由で posts を読み書きします。
type PostRepository struct {
	reader *query.Queries
	writer *query.Queries
}

// NewPostRepository creates a PostRepository that reads through the database's
// read pool and writes through its write pool.
//
// [Ja] NewPostRepository は、データベースの読み取り用プールで読み、書き込み用プールで
// 書く PostRepository を生成します。
func NewPostRepository(db *database.DB) *PostRepository {
	return &PostRepository{reader: query.New(db.Reader), writer: query.New(db.Writer)}
}

// WithTx returns a new PostRepository whose queries run inside tx, so a UseCase
// can enlist this repository in its transaction. The receiver is left unchanged.
//
// [Ja] WithTx は queries を tx 内で実行する新しい PostRepository を返し、UseCase が
// 本リポジトリを自身のトランザクションに参加させられるようにします。レシーバ自身は
// 変更しません。
func (r *PostRepository) WithTx(tx *sql.Tx) *PostRepository {
	q := r.writer.WithTx(tx)
	return &PostRepository{reader: q, writer: q}
}

// ListByThreadID returns every post in the thread in reply-number order, which
// is the order they are displayed in. There is no limit and no offset: a thread
// is capped at 1000 posts and its post list is not paginated (ADR 0009), so the
// whole thread is always what a caller asks for.
//
// No row is left out, and none will be: muting (M3) marks a post rather than
// dropping it, because hiding a reply outright breaks the thread of a
// conversation that quotes it. The mark is one more column on these same rows,
// not a second query.
//
// [Ja] ListByThreadID はスレッドのすべての投稿をレス番号順、すなわち表示される順で
// 返します。上限もオフセットもありません。スレッドの投稿数は 1000 件が上限で投稿一覧は
// ページ分割しない (ADR 0009) ため、呼び出し元が求めるのは常にスレッド全体です。
//
// 行を 1 つも落としませんし、今後も落としません。ミュート (M3) は投稿を落とすのではなく
// 印を付けます。返信をそのまま隠すと、それを引用した会話の繋がりが壊れるためです。その印は
// 同じ行に足す 1 列であって、2 本目のクエリではありません。
func (r *PostRepository) ListByThreadID(ctx context.Context, threadID model.ThreadID) ([]*model.Post, error) {
	rows, err := r.reader.ListPostsByThreadID(ctx, int64(threadID))
	if err != nil {
		return nil, err
	}

	posts := make([]*model.Post, len(rows))
	for i, row := range rows {
		posts[i] = r.toModel(row)
	}
	return posts, nil
}

// FindLatestByUserID returns the most recent post the user wrote anywhere in
// the instance, or (nil, nil) when they have written none. It is the row the
// interval between one person's posts is measured from, so the search is
// narrowed to neither a board, a thread nor a session: the interval belongs to
// the person, and writing in another thread does not start a fresh one.
//
// Posts sharing a timestamp are ordered by id, the same tie-break the index
// carries, so the row that comes back is the same one on every call.
//
// [Ja] FindLatestByUserID は、そのユーザーがインスタンスのどこかに書いた最新の投稿を
// 返し、1 件も書いていない場合は (nil, nil) を返します。これは 1 人の投稿の間隔を測る
// 起点となる行であるため、掲示板でもスレッドでもセッションでも絞り込みません。間隔は
// その人に属するものであり、別のスレッドへ書いても新しく始まることはありません。
//
// 時刻が同じ投稿は id で順序を決めます。索引が持つのと同じ同着の解き方であり、返る行は
// どの呼び出しでも同じものになります。
func (r *PostRepository) FindLatestByUserID(ctx context.Context, userID model.UserID) (*model.Post, error) {
	raw := int64(userID)

	row, err := r.reader.GetLatestPostByUserID(ctx, &raw)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return r.toModel(row), nil
}

// CreatePostInput holds the attributes needed to create a post. id and the
// timestamps are assigned by the database. Number is passed in rather than
// derived here because it is decided together with the thread's posts_count, in
// the transaction that writes both.
//
// [Ja] CreatePostInput は投稿の作成に必要な属性を保持します。id とタイムスタンプは DB 側で
// 採番されます。Number をここで導かずに受け取るのは、それがスレッドの posts_count と
// 同時に、両方を書き込むトランザクションの中で決まるためです。
type CreatePostInput struct {
	ThreadID model.ThreadID
	UserID   *model.UserID
	Number   int
	Body     string
}

// Create inserts a post and returns it with the database-assigned id and
// timestamps populated.
//
// [Ja] Create は投稿を挿入し、DB が採番した id とタイムスタンプを設定した状態で返します。
func (r *PostRepository) Create(ctx context.Context, input CreatePostInput) (*model.Post, error) {
	row, err := r.writer.CreatePost(ctx, query.CreatePostParams{
		ThreadID: int64(input.ThreadID),
		UserID:   rawAuthorID(input.UserID),
		Number:   int64(input.Number),
		Body:     input.Body,
	})
	if err != nil {
		return nil, err
	}
	return r.toModel(row), nil
}

// FindByThreadIDAndNumber returns the post at the given reply number of the
// thread, or (nil, nil) when the thread has no post with that number. The pair
// is what addresses a post everywhere it is referred to — a >>N in a body, the
// #p{number} anchor, a URL shared elsewhere — so it is what a caller acting on
// one post of a thread holds, rather than the post's own id.
//
// An unpublished post is returned. What a caller does with it depends on why it
// asked: the answer for a moderator aiming at a post is not the answer for the
// listing, and neither is decided here.
//
// [Ja] FindByThreadIDAndNumberはスレッドの指定したレス番号の投稿を返し、その番号の投稿が
// 無い場合は (nil, nil) を返します。この組は、投稿が参照されるあらゆる場所 — 本文中の
// >>N、アンカーの #p{number}、外部で共有されたURL — で投稿を指すものであるため、スレッドの
// 1つの投稿を対象とする呼び出し元が持つのは、投稿自身のidではなくこの組になります。
//
// 非公開の投稿も返します。それをどう扱うかは、何のために引いたかによります。投稿を対象と
// する管理者への答えは一覧への答えとは異なり、そのどちらもここでは決めません。
func (r *PostRepository) FindByThreadIDAndNumber(ctx context.Context, threadID model.ThreadID, number int) (*model.Post, error) {
	row, err := r.reader.GetPostByThreadIDAndNumber(ctx, query.GetPostByThreadIDAndNumberParams{
		ThreadID: int64(threadID),
		Number:   int64(number),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return r.toModel(row), nil
}

// ListByIDs returns the posts among the given ids that still exist, in id
// order, so a caller holding a set of ids resolves them in one query.
//
// Unpublished posts are returned, as they are in ListByThreadID: the mark hides
// a post's body from the thread's readers, not the row from the callers that
// name it. The moderation log's entry for an unpublished post is about that very
// post, and a row that came back with nothing would leave the entry unreadable.
//
// An empty slice of ids returns an empty slice without querying: there is
// nothing to look posts up by.
//
// [Ja] ListByIDsは指定したidのうち、まだ存在する投稿をid順で返し、idの集合を持つ
// 呼び出し元が1クエリでそれらを解決できるようにします。
//
// 非公開の投稿も、ListByThreadIDと同じく返します。印が隠すのはスレッドの読み手に対する
// 本文であって、その投稿を名指す呼び出し元に対する行ではありません。非公開にされた投稿に
// ついての操作履歴の記録は、まさにその投稿についてのものであり、何も返ってこない行はその
// 記録を読めなくします。
//
// 空のidスライスに対してはクエリを発行せず空のスライスを返します。投稿を引く手がかりが
// 無いためです。
func (r *PostRepository) ListByIDs(ctx context.Context, ids []model.PostID) ([]*model.Post, error) {
	if len(ids) == 0 {
		return []*model.Post{}, nil
	}

	rawIDs := make([]int64, len(ids))
	for i, id := range ids {
		rawIDs[i] = int64(id)
	}

	rows, err := r.reader.ListPostsByIDs(ctx, rawIDs)
	if err != nil {
		return nil, err
	}

	posts := make([]*model.Post, len(rows))
	for i, row := range rows {
		posts[i] = r.toModel(row)
	}
	return posts, nil
}

// Unpublish stamps the post as unpublished by an administrator. It touches this
// one row and nothing else: the thread's posts_count, last_post_id and
// last_posted_at stay as they are, because the count is the number of reply
// numbers issued rather than the number of bodies on display, and a thread that
// reached the cap does not reopen by having one of its posts unpublished.
//
// The timestamp uses the database clock, as do thread moderation timestamps
// and moderation_logs.created_at.
//
// [Ja] Unpublishは投稿に管理者による非公開の時刻を打刻します。触れるのはこの1行だけで
// あり、スレッドのposts_count・last_post_id・last_posted_atはそのままです。件数は表示
// されている本文の数ではなく発行したレス番号の数であり、上限に達したスレッドは、その投稿の
// 1つが非公開になっても書き込みを再開しないためです。
//
// 時刻はスレッドのモデレーションの時刻やmoderation_logs.created_atと同じく、
// データベースの時計を使います。
func (r *PostRepository) Unpublish(ctx context.Context, id model.PostID) error {
	return r.writer.UnpublishPost(ctx, int64(id))
}

// toModel converts a query.Post row into a model.Post, casting the raw ids into
// their typed forms and the stored timestamps back into time.Time at the
// repository boundary. UserID is nil only after the author's account row has
// been physically deleted; a logical withdrawal leaves the id in the post row.
//
// [Ja] toModel は query.Post を model.Post に変換し、リポジトリの境界で生の id を型付きの
// 形に、保存書式の時刻を time.Time にキャストします。UserID は作者のアカウント行が物理削除
// された後にだけ nil になり、論理退会では投稿行に id が残ります。
func (r *PostRepository) toModel(row query.Post) *model.Post {
	return &model.Post{
		ID:       model.PostID(row.ID),
		ThreadID: model.ThreadID(row.ThreadID),
		UserID:   typedAuthorID(row.UserID),
		Number:   int(row.Number),
		Body:     row.Body,

		UnpublishedAt: sqlitetime.TimePtr(row.UnpublishedAt),

		CreatedAt: time.Time(row.CreatedAt),
		UpdatedAt: time.Time(row.UpdatedAt),
	}
}
