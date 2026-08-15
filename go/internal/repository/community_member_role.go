package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/query"
)

// CommunityMemberRoleRepository reads and writes role assignments through
// sqlc-generated queries.
//
// [Ja] CommunityMemberRoleRepository は sqlc 生成のクエリ経由で
// community_member_roles を読み書きします。
type CommunityMemberRoleRepository struct {
	q *query.Queries
}

// NewCommunityMemberRoleRepository creates a CommunityMemberRoleRepository
// backed by the given queries.
//
// [Ja] NewCommunityMemberRoleRepository は与えられた queries を使う
// CommunityMemberRoleRepository を生成します。
func NewCommunityMemberRoleRepository(q *query.Queries) *CommunityMemberRoleRepository {
	return &CommunityMemberRoleRepository{q: q}
}

// WithTx returns a new CommunityMemberRoleRepository whose queries run inside tx,
// so a UseCase can enlist this repository in its transaction. The receiver is
// left unchanged.
//
// [Ja] WithTx は queries を tx 内で実行する新しい CommunityMemberRoleRepository を返し、
// UseCase が本リポジトリを自身のトランザクションに参加させられるようにします。
// レシーバ自身は変更しません。
func (r *CommunityMemberRoleRepository) WithTx(tx pgx.Tx) *CommunityMemberRoleRepository {
	return &CommunityMemberRoleRepository{q: r.q.WithTx(tx)}
}

// CreateCommunityMemberRoleInput holds the attributes needed to assign a role to
// a member. id and the timestamps are assigned by the database.
//
// CommunityID is taken from the caller instead of being derived from the member,
// because it is the column both composite foreign keys share: passing the
// community the assignment is meant for is what lets the database compare it
// against the member's and the role's own community.
//
// [Ja] CreateCommunityMemberRoleInput はメンバーへのロール割当に必要な属性を保持します。
// id とタイムスタンプは DB 側で採番されます。
//
// CommunityID をメンバーから導出せず呼び出し側から受け取るのは、これが 2 本の複合外部キーが
// 共有する列だからです。その割当が属するコミュニティを渡すことで初めて、DB がメンバー側・
// ロール側それぞれのコミュニティと突き合わせられます。
type CreateCommunityMemberRoleInput struct {
	CommunityID       model.CommunityID
	CommunityMemberID model.CommunityMemberID
	CommunityRoleID   model.CommunityRoleID
}

// Create inserts a role assignment and returns it with the database-assigned id
// and timestamps populated. The composite foreign keys reject an assignment
// whose member and role do not both belong to the given community, and the
// member-role pair is UNIQUE, so both mismatched and duplicate assignments fail
// with an error the caller must handle.
//
// [Ja] Create はロール割当を挿入し、DB が採番した id とタイムスタンプを設定した状態で
// 返します。複合外部キーは、メンバーとロールの双方が指定コミュニティに属していない割当を
// 拒否し、メンバーとロールの組は UNIQUE のため、コミュニティ不一致の割当も重複した割当も
// エラーで失敗します。呼び出し側はこれを扱う必要があります。
func (r *CommunityMemberRoleRepository) Create(ctx context.Context, input CreateCommunityMemberRoleInput) (*model.CommunityMemberRole, error) {
	row, err := r.q.CreateCommunityMemberRole(ctx, query.CreateCommunityMemberRoleParams{
		CommunityID:       uuid.UUID(input.CommunityID),
		CommunityMemberID: uuid.UUID(input.CommunityMemberID),
		CommunityRoleID:   uuid.UUID(input.CommunityRoleID),
	})
	if err != nil {
		return nil, err
	}
	return r.toModel(row), nil
}

// toModel converts a query.CommunityMemberRole row into a
// model.CommunityMemberRole, casting the raw uuids into the typed IDs at the
// repository boundary.
//
// [Ja] toModel は query.CommunityMemberRole を model.CommunityMemberRole に変換し、
// リポジトリの境界で生の uuid を型付きの ID にキャストします。
func (r *CommunityMemberRoleRepository) toModel(row query.CommunityMemberRole) *model.CommunityMemberRole {
	return &model.CommunityMemberRole{
		ID:                model.CommunityMemberRoleID(row.ID),
		CommunityID:       model.CommunityID(row.CommunityID),
		CommunityMemberID: model.CommunityMemberID(row.CommunityMemberID),
		CommunityRoleID:   model.CommunityRoleID(row.CommunityRoleID),
		CreatedAt:         row.CreatedAt,
		UpdatedAt:         row.UpdatedAt,
	}
}
