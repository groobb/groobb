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

// PostRepositoryはsqlc生成のクエリ経由でpostsを読み書きします。
type PostRepository struct {
	reader *query.Queries
	writer *query.Queries
}

// NewPostRepositoryは、データベースの読み取り用プールで読み、書き込み用プールで
// 書くPostRepositoryを生成します。
func NewPostRepository(db *database.DB) *PostRepository {
	return &PostRepository{reader: query.New(db.Reader), writer: query.New(db.Writer)}
}

// WithTxはqueriesをtx内で実行する新しいPostRepositoryを返し、UseCaseが
// 本リポジトリを自身のトランザクションに参加させられるようにします。レシーバ自身は
// 変更しません。
func (r *PostRepository) WithTx(tx *sql.Tx) *PostRepository {
	q := r.writer.WithTx(tx)
	return &PostRepository{reader: q, writer: q}
}

// ListByThreadIDはスレッドのすべての投稿をレス番号順、すなわち表示される順で
// 返します。上限もオフセットもありません。スレッドの投稿数は1000件が上限で投稿一覧は
// ページ分割しない (ADR 0009) ため、呼び出し元が求めるのは常にスレッド全体です。
//
// 行を1つも落としませんし、今後も落としません。ミュート (M3) は投稿を落とすのではなく
// 印を付けます。返信をそのまま隠すと、それを引用した会話の繋がりが壊れるためです。その印は
// 同じ行に足す1列であって、2本目のクエリではありません。
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

// FindLatestByUserIDは、そのユーザーがインスタンスのどこかに書いた最新の投稿を
// 返し、1件も書いていない場合は (nil, nil) を返します。これは1人の投稿の間隔を測る
// 起点となる行であるため、掲示板でもスレッドでもセッションでも絞り込みません。間隔は
// その人に属するものであり、別のスレッドへ書いても新しく始まることはありません。
//
// 時刻が同じ投稿はidで順序を決めます。索引が持つのと同じ同着の解き方であり、返る行は
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

// CreatePostInputは投稿の作成に必要な属性を保持します。idとタイムスタンプはDB側で
// 採番されます。Numberをここで導かずに受け取るのは、それがスレッドのposts_countと
// 同時に、両方を書き込むトランザクションの中で決まるためです。
type CreatePostInput struct {
	ThreadID model.ThreadID
	UserID   *model.UserID
	Number   int
	Body     string
}

// Createは投稿を挿入し、DBが採番したidとタイムスタンプを設定した状態で返します。
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

// FindByThreadIDAndNumberはスレッドの指定したレス番号の投稿を返し、その番号の投稿が
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

// ListByIDsは指定したidのうち、まだ存在する投稿をid順で返し、idの集合を持つ
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

// Unpublishは投稿に管理者による非公開の時刻を打刻します。触れるのはこの1行だけで
// あり、スレッドのposts_count・last_post_id・last_posted_atはそのままです。件数は表示
// されている本文の数ではなく発行したレス番号の数であり、上限に達したスレッドは、その投稿の
// 1つが非公開になっても書き込みを再開しないためです。
//
// 時刻はスレッドのモデレーションの時刻やmoderation_logs.created_atと同じく、
// データベースの時計を使います。
func (r *PostRepository) Unpublish(ctx context.Context, id model.PostID) error {
	return r.writer.UnpublishPost(ctx, int64(id))
}

// toModelはquery.Postをmodel.Postに変換し、リポジトリの境界で生のidを型付きの
// 形に、保存書式の時刻をtime.Timeにキャストします。UserIDは作者のアカウント行が物理削除
// された後にだけnilになり、論理退会では投稿行にidが残ります。
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
