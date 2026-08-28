package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/query"
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
		ID:        model.PostID(row.ID),
		ThreadID:  model.ThreadID(row.ThreadID),
		UserID:    typedAuthorID(row.UserID),
		Number:    int(row.Number),
		Body:      row.Body,
		CreatedAt: time.Time(row.CreatedAt),
		UpdatedAt: time.Time(row.UpdatedAt),
	}
}
