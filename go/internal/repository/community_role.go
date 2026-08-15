package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/query"
)

// CommunityRoleRepository reads and writes community roles through
// sqlc-generated queries.
//
// [Ja] CommunityRoleRepository は sqlc 生成のクエリ経由で community_roles を
// 読み書きします。
type CommunityRoleRepository struct {
	q *query.Queries
}

// NewCommunityRoleRepository creates a CommunityRoleRepository backed by the
// given queries.
//
// [Ja] NewCommunityRoleRepository は与えられた queries を使う
// CommunityRoleRepository を生成します。
func NewCommunityRoleRepository(q *query.Queries) *CommunityRoleRepository {
	return &CommunityRoleRepository{q: q}
}

// WithTx returns a new CommunityRoleRepository whose queries run inside tx, so a
// UseCase can enlist this repository in its transaction. The receiver is left
// unchanged.
//
// [Ja] WithTx は queries を tx 内で実行する新しい CommunityRoleRepository を返し、
// UseCase が本リポジトリを自身のトランザクションに参加させられるようにします。
// レシーバ自身は変更しません。
func (r *CommunityRoleRepository) WithTx(tx pgx.Tx) *CommunityRoleRepository {
	return &CommunityRoleRepository{q: r.q.WithTx(tx)}
}

// CreateCommunityRoleInput holds the attributes needed to create a community
// role. id and the timestamps are assigned by the database.
//
// [Ja] CreateCommunityRoleInput はコミュニティロールの作成に必要な属性を保持します。
// id とタイムスタンプは DB 側で採番されます。
type CreateCommunityRoleInput struct {
	CommunityID model.CommunityID
	Name        string
}

// Create inserts a community role and returns it with the database-assigned id
// and timestamps populated. Role names are UNIQUE per community, so creating a
// role whose name the community already uses fails with a UNIQUE-violation error
// the caller must handle rather than silently creating a duplicate.
//
// [Ja] Create はコミュニティロールを挿入し、DB が採番した id とタイムスタンプを
// 設定した状態で返します。ロール名はコミュニティごとに UNIQUE のため、そのコミュニティが
// 既に使っている名前のロールを作ろうとすると、重複を黙って作らずに UNIQUE 制約違反の
// エラーで失敗します。呼び出し側はこれを扱う必要があります。
func (r *CommunityRoleRepository) Create(ctx context.Context, input CreateCommunityRoleInput) (*model.CommunityRole, error) {
	row, err := r.q.CreateCommunityRole(ctx, query.CreateCommunityRoleParams{
		CommunityID: uuid.UUID(input.CommunityID),
		Name:        input.Name,
	})
	if err != nil {
		return nil, err
	}
	return r.toModel(row), nil
}

// toModel converts a query.CommunityRole row into a model.CommunityRole, casting
// the raw uuids into the typed IDs at the repository boundary.
//
// [Ja] toModel は query.CommunityRole を model.CommunityRole に変換し、リポジトリの
// 境界で生の uuid を型付きの ID にキャストします。
func (r *CommunityRoleRepository) toModel(row query.CommunityRole) *model.CommunityRole {
	return &model.CommunityRole{
		ID:          model.CommunityRoleID(row.ID),
		CommunityID: model.CommunityID(row.CommunityID),
		Name:        row.Name,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
}
