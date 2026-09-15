package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/query"
)

// PostReferenceRepositoryはsqlc生成のクエリ経由でpost_referencesを読み書き
// します。
type PostReferenceRepository struct {
	reader *query.Queries
	writer *query.Queries
}

// NewPostReferenceRepositoryは、データベースの読み取り用プールで読み、書き込み用
// プールで書くPostReferenceRepositoryを生成します。
func NewPostReferenceRepository(db *database.DB) *PostReferenceRepository {
	return &PostReferenceRepository{reader: query.New(db.Reader), writer: query.New(db.Writer)}
}

// WithTxはqueriesをtx内で実行する新しいPostReferenceRepositoryを返し、
// UseCaseが本リポジトリを自身のトランザクションに参加させられるようにします。
// レシーバ自身は変更しません。
func (r *PostReferenceRepository) WithTx(tx *sql.Tx) *PostReferenceRepository {
	q := r.writer.WithTx(tx)
	return &PostReferenceRepository{reader: q, writer: q}
}

// ListByReferencedPostIDsは指定したいずれかの投稿を指す参照を返し、スレッドを描く
// 呼び出し元が、これから表示する各投稿に後続のどの投稿が返信したかを1クエリで知れる
// ようにします。idを1つずつではなくまとめて取るのは、そうしなければ最大1000件の
// 投稿を載せるページで投稿1件につき1クエリになるためです。
//
// 行は指し先の投稿ごとにまとまり、その中では参照した投稿が書かれた順で返るため、
// 呼び出し元は並べ替えずに描画できます。順序をエンジンに委ねず明示するのは、エンジンが
// たまたま採った計画の順で行を返してよいためです。
//
// 空のidスライスに対してはクエリを発行せず空のスライスを返します。参照を引く手がかりが
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

// CreatePostReferenceInputは参照の作成に必要な属性を保持します。idとタイムスタンプは
// DB側で採番されます。
type CreatePostReferenceInput struct {
	PostID           model.PostID
	ReferencedPostID model.PostID
}

// Createは参照を挿入し、DBが採番したidとタイムスタンプを設定した状態で返します。
// 同じ >>Nを2度書いた本文が生む参照は1つのため、呼び出し元は抽出したものを重複除去
// してから書き込みます。UNIQUE (post_id, referenced_post_id) は2度目のINSERTを拒否
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

// CreatePostReferencesInputは、本文を読み取った投稿と、その本文が参照するレス番号を
// 保持します。投稿自身のスレッドと番号をidと一緒に運ぶのは、それらが番号を解決する
// 相手であるためです。レス番号は1つのスレッドの中でしか意味を持たず、返信できるのは
// その投稿自身の番号より下の番号だけです。
type CreatePostReferencesInput struct {
	PostID            model.PostID
	ThreadID          model.ThreadID
	Number            int
	ReferencedNumbers []int
}

// CreateAllByReferencedNumbersは投稿の本文が参照するものを記録します。レス番号から
// 投稿への解決はINSERT自身の中で行います。1番号につき1回引き当てるのではなく1つの文に
// するのは、本文がスレッドの持つ数だけ番号を名指しうるうえ、その引き当てのどれもが投稿を
// 書き込むトランザクション、すなわち次の書き手が待っているトランザクションの中に入るため
// です。
//
// 行を書くのは、同じスレッドの投稿が持つ番号のうち、参照した投稿自身の番号より下のものに
// 対してだけです。どの投稿も持たない番号・その投稿自身の番号・それより先の番号は、いずれも
// 行を作りません。レス番号はスレッド内の1つの投稿を指すため、>>5を2度書いた本文も、
// UNIQUE (post_id, referenced_post_id) が認める1つの関係だけを記録します。
//
// 番号が1つも無ければ、クエリを発行せず何も書きません。どの投稿も参照しない本文には、
// 記録する参照がないためです。
func (r *PostReferenceRepository) CreateAllByReferencedNumbers(ctx context.Context, input CreatePostReferencesInput) error {
	if len(input.ReferencedNumbers) == 0 {
		return nil
	}

	numbers := make([]int64, len(input.ReferencedNumbers))
	for i, number := range input.ReferencedNumbers {
		numbers[i] = int64(number)
	}

	return r.writer.CreatePostReferencesByNumbers(ctx, query.CreatePostReferencesByNumbersParams{
		PostID:   int64(input.PostID),
		ThreadID: int64(input.ThreadID),
		Number:   int64(input.Number),
		Numbers:  numbers,
	})
}

// toModelはquery.PostReferenceをmodel.PostReferenceに変換し、リポジトリの境界で
// 生のidを型付きの形に、保存書式の時刻をtime.Timeにキャストします。
func (r *PostReferenceRepository) toModel(row query.PostReference) *model.PostReference {
	return &model.PostReference{
		ID:               model.PostReferenceID(row.ID),
		PostID:           model.PostID(row.PostID),
		ReferencedPostID: model.PostID(row.ReferencedPostID),
		CreatedAt:        time.Time(row.CreatedAt),
		UpdatedAt:        time.Time(row.UpdatedAt),
	}
}
