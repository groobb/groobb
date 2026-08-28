package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/query"
)

// PostReferenceRepository reads and writes post references through
// sqlc-generated queries.
//
// [Ja] PostReferenceRepository は sqlc 生成のクエリ経由で post_references を読み書き
// します。
type PostReferenceRepository struct {
	reader *query.Queries
	writer *query.Queries
}

// NewPostReferenceRepository creates a PostReferenceRepository that reads
// through the database's read pool and writes through its write pool.
//
// [Ja] NewPostReferenceRepository は、データベースの読み取り用プールで読み、書き込み用
// プールで書く PostReferenceRepository を生成します。
func NewPostReferenceRepository(db *database.DB) *PostReferenceRepository {
	return &PostReferenceRepository{reader: query.New(db.Reader), writer: query.New(db.Writer)}
}

// WithTx returns a new PostReferenceRepository whose queries run inside tx, so a
// UseCase can enlist this repository in its transaction. The receiver is left
// unchanged.
//
// [Ja] WithTx は queries を tx 内で実行する新しい PostReferenceRepository を返し、
// UseCase が本リポジトリを自身のトランザクションに参加させられるようにします。
// レシーバ自身は変更しません。
func (r *PostReferenceRepository) WithTx(tx *sql.Tx) *PostReferenceRepository {
	q := r.writer.WithTx(tx)
	return &PostReferenceRepository{reader: q, writer: q}
}

// ListByReferencedPostIDs returns the references pointing at any of the given
// posts, so a caller rendering a thread learns in one query which later posts
// replied to each of the posts it is about to display. It takes the ids
// together rather than one at a time because the alternative is one query per
// post on a page that carries up to a thousand of them.
//
// The rows come back grouped by the post they point at and, within a group, in
// the order the referring posts were written, so a caller renders them without
// sorting. Ordering is stated rather than left to the engine, which is free to
// return the rows in whichever order the plan happens to produce.
//
// An empty slice of ids returns an empty slice without querying: there is
// nothing to look references up by.
//
// [Ja] ListByReferencedPostIDs は指定したいずれかの投稿を指す参照を返し、スレッドを描く
// 呼び出し元が、これから表示する各投稿に後続のどの投稿が返信したかを 1 クエリで知れる
// ようにします。id を 1 つずつではなくまとめて取るのは、そうしなければ最大 1000 件の
// 投稿を載せるページで投稿 1 件につき 1 クエリになるためです。
//
// 行は指し先の投稿ごとにまとまり、その中では参照した投稿が書かれた順で返るため、
// 呼び出し元は並べ替えずに描画できます。順序をエンジンに委ねず明示するのは、エンジンが
// たまたま採った計画の順で行を返してよいためです。
//
// 空の id スライスに対してはクエリを発行せず空のスライスを返します。参照を引く手がかりが
// 無いためです。
func (r *PostReferenceRepository) ListByReferencedPostIDs(ctx context.Context, referencedPostIDs []model.PostID) ([]*model.PostReference, error) {
	if len(referencedPostIDs) == 0 {
		return []*model.PostReference{}, nil
	}

	rawIDs := make([]int64, len(referencedPostIDs))
	for i, id := range referencedPostIDs {
		rawIDs[i] = int64(id)
	}

	rows, err := r.reader.ListPostReferencesByReferencedPostIDs(ctx, rawIDs)
	if err != nil {
		return nil, err
	}

	references := make([]*model.PostReference, len(rows))
	for i, row := range rows {
		references[i] = r.toModel(row)
	}
	return references, nil
}

// CreatePostReferenceInput holds the attributes needed to create a reference. id
// and the timestamps are assigned by the database.
//
// [Ja] CreatePostReferenceInput は参照の作成に必要な属性を保持します。id とタイムスタンプは
// DB 側で採番されます。
type CreatePostReferenceInput struct {
	PostID           model.PostID
	ReferencedPostID model.PostID
}

// Create inserts a reference and returns it with the database-assigned id and
// timestamps populated. A body that writes the same >>N twice yields one
// reference, so the caller deduplicates what it extracted before writing it:
// UNIQUE (post_id, referenced_post_id) rejects the second insert.
//
// [Ja] Create は参照を挿入し、DB が採番した id とタイムスタンプを設定した状態で返します。
// 同じ >>N を 2 度書いた本文が生む参照は 1 つのため、呼び出し元は抽出したものを重複除去
// してから書き込みます。UNIQUE (post_id, referenced_post_id) は 2 度目の INSERT を拒否
// します。
func (r *PostReferenceRepository) Create(ctx context.Context, input CreatePostReferenceInput) (*model.PostReference, error) {
	row, err := r.writer.CreatePostReference(ctx, query.CreatePostReferenceParams{
		PostID:           int64(input.PostID),
		ReferencedPostID: int64(input.ReferencedPostID),
	})
	if err != nil {
		return nil, err
	}
	return r.toModel(row), nil
}

// toModel converts a query.PostReference row into a model.PostReference,
// casting the raw ids into their typed forms and the stored timestamps back
// into time.Time at the repository boundary.
//
// [Ja] toModel は query.PostReference を model.PostReference に変換し、リポジトリの境界で
// 生の id を型付きの形に、保存書式の時刻を time.Time にキャストします。
func (r *PostReferenceRepository) toModel(row query.PostReference) *model.PostReference {
	return &model.PostReference{
		ID:               model.PostReferenceID(row.ID),
		PostID:           model.PostID(row.PostID),
		ReferencedPostID: model.PostID(row.ReferencedPostID),
		CreatedAt:        time.Time(row.CreatedAt),
		UpdatedAt:        time.Time(row.UpdatedAt),
	}
}
