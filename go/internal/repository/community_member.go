package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/query"
)

// CommunityMemberRepository reads and writes community memberships through
// sqlc-generated queries.
//
// [Ja] CommunityMemberRepository は sqlc 生成のクエリ経由で community_members を
// 読み書きします。
type CommunityMemberRepository struct {
	q *query.Queries
}

// NewCommunityMemberRepository creates a CommunityMemberRepository backed by the
// given queries.
//
// [Ja] NewCommunityMemberRepository は与えられた queries を使う
// CommunityMemberRepository を生成します。
func NewCommunityMemberRepository(q *query.Queries) *CommunityMemberRepository {
	return &CommunityMemberRepository{q: q}
}

// WithTx returns a new CommunityMemberRepository whose queries run inside tx, so
// a UseCase can enlist this repository in its transaction. The receiver is left
// unchanged.
//
// [Ja] WithTx は queries を tx 内で実行する新しい CommunityMemberRepository を返し、
// UseCase が本リポジトリを自身のトランザクションに参加させられるようにします。
// レシーバ自身は変更しません。
func (r *CommunityMemberRepository) WithTx(tx pgx.Tx) *CommunityMemberRepository {
	return &CommunityMemberRepository{q: r.q.WithTx(tx)}
}

// CreateCommunityMemberInput holds the attributes needed to create a community
// membership. id and the timestamps are assigned by the database.
//
// [Ja] CreateCommunityMemberInput はコミュニティのメンバーシップ作成に必要な属性を
// 保持します。id とタイムスタンプは DB 側で採番されます。
type CreateCommunityMemberInput struct {
	CommunityID model.CommunityID
	UserID      model.UserID
}

// Create inserts a community membership and returns it with the
// database-assigned id and timestamps populated. A user may belong to a
// community only once, so creating a membership for a user who already joined
// fails with a UNIQUE-violation error the caller must handle rather than
// silently creating a second membership.
//
// [Ja] Create はコミュニティのメンバーシップを挿入し、DB が採番した id とタイムスタンプを
// 設定した状態で返します。ユーザーが 1 つのコミュニティに所属できるのは 1 度だけのため、
// 既に参加しているユーザーのメンバーシップを作ろうとすると、2 つ目を黙って作らずに
// UNIQUE 制約違反のエラーで失敗します。呼び出し側はこれを扱う必要があります。
func (r *CommunityMemberRepository) Create(ctx context.Context, input CreateCommunityMemberInput) (*model.CommunityMember, error) {
	row, err := r.q.CreateCommunityMember(ctx, query.CreateCommunityMemberParams{
		CommunityID: uuid.UUID(input.CommunityID),
		UserID:      uuid.UUID(input.UserID),
	})
	if err != nil {
		return nil, err
	}
	return r.toModel(row), nil
}

// toModel converts a query.CommunityMember row into a model.CommunityMember,
// casting the raw uuids into the typed IDs at the repository boundary.
//
// [Ja] toModel は query.CommunityMember を model.CommunityMember に変換し、リポジトリの
// 境界で生の uuid を型付きの ID にキャストします。
func (r *CommunityMemberRepository) toModel(row query.CommunityMember) *model.CommunityMember {
	return &model.CommunityMember{
		ID:          model.CommunityMemberID(row.ID),
		CommunityID: model.CommunityID(row.CommunityID),
		UserID:      model.UserID(row.UserID),
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
}
