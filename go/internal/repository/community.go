package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/query"
)

// CommunityRepository reads and writes communities through sqlc-generated
// queries.
//
// [Ja] CommunityRepository は sqlc 生成のクエリ経由で communities を読み書きします。
type CommunityRepository struct {
	q *query.Queries
}

// NewCommunityRepository creates a CommunityRepository backed by the given
// queries.
//
// [Ja] NewCommunityRepository は与えられた queries を使う CommunityRepository を
// 生成します。
func NewCommunityRepository(q *query.Queries) *CommunityRepository {
	return &CommunityRepository{q: q}
}

// WithTx returns a new CommunityRepository whose queries run inside tx, so a
// UseCase can enlist this repository in its transaction. The receiver is left
// unchanged.
//
// [Ja] WithTx は queries を tx 内で実行する新しい CommunityRepository を返し、UseCase が
// 本リポジトリを自身のトランザクションに参加させられるようにします。レシーバ自身は
// 変更しません。
func (r *CommunityRepository) WithTx(tx pgx.Tx) *CommunityRepository {
	return &CommunityRepository{q: r.q.WithTx(tx)}
}

// FindByIdentifier returns the community with the given identifier, or
// (nil, nil) when none exists. The identifier column is citext, so the match
// ignores letter case (the same casing rule the identifier UNIQUE constraint
// enforces). Absence is a normal lookup outcome, not an error; the caller
// decides whether to treat it as a business-level failure.
//
// [Ja] FindByIdentifier は指定 identifier のコミュニティを返し、存在しない場合は
// (nil, nil) を返します。identifier 列は citext のため照合は大文字小文字を無視します
// (identifier の UNIQUE 制約が強制するのと同じ大小の規則)。未存在は正常なルックアップ
// 結果でありエラーではありません。業務上の失敗として扱うかは呼び出し側が判断します。
func (r *CommunityRepository) FindByIdentifier(ctx context.Context, identifier string) (*model.Community, error) {
	row, err := r.q.GetCommunityByIdentifier(ctx, identifier)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return r.toModel(row), nil
}

// CreateCommunityInput holds the attributes needed to create a community. id and
// the timestamps are assigned by the database.
//
// [Ja] CreateCommunityInput はコミュニティ作成に必要な属性を保持します。id と
// タイムスタンプは DB 側で採番されます。
type CreateCommunityInput struct {
	Name       string
	Identifier string
}

// Create inserts a community and returns it with the database-assigned id and
// timestamps populated. The identifier column is citext + UNIQUE, so if another
// community has claimed the same identifier between validation and this insert,
// the write fails with a UNIQUE-violation error the caller must handle rather
// than silently creating a duplicate.
//
// [Ja] Create はコミュニティを挿入し、DB が採番した id とタイムスタンプを設定した状態で
// 返します。identifier 列は citext + UNIQUE のため、検証からこの挿入までの間に別の
// コミュニティが同じ identifier を取得していた場合、この書き込みは重複を黙って作らずに
// UNIQUE 制約違反のエラーで失敗します。呼び出し側はこれを扱う必要があります。
func (r *CommunityRepository) Create(ctx context.Context, input CreateCommunityInput) (*model.Community, error) {
	row, err := r.q.CreateCommunity(ctx, query.CreateCommunityParams{
		Name:       input.Name,
		Identifier: input.Identifier,
	})
	if err != nil {
		return nil, err
	}
	return r.toModel(row), nil
}

// toModel converts a query.Community row into a model.Community, casting the raw
// uuid into the typed CommunityID at the repository boundary.
//
// [Ja] toModel は query.Community を model.Community に変換し、リポジトリの境界で生の
// uuid を型付きの CommunityID にキャストします。
func (r *CommunityRepository) toModel(row query.Community) *model.Community {
	return &model.Community{
		ID:         model.CommunityID(row.ID),
		Name:       row.Name,
		Identifier: row.Identifier,
		CreatedAt:  row.CreatedAt,
		UpdatedAt:  row.UpdatedAt,
	}
}
