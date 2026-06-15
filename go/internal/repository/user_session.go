package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/query"
)

// UserSessionRepository reads and writes user_sessions through sqlc-generated
// queries.
//
// [Ja] UserSessionRepository は sqlc 生成のクエリ経由で user_sessions を読み書き
// します。
type UserSessionRepository struct {
	q *query.Queries
}

// NewUserSessionRepository creates a UserSessionRepository backed by the given
// queries.
//
// [Ja] NewUserSessionRepository は与えられた queries を使う UserSessionRepository を
// 生成します。
func NewUserSessionRepository(q *query.Queries) *UserSessionRepository {
	return &UserSessionRepository{q: q}
}

// WithTx returns a new UserSessionRepository whose queries run inside tx, so a
// UseCase can enlist this repository in its transaction. The receiver is left
// unchanged.
//
// [Ja] WithTx は queries を tx 内で実行する新しい UserSessionRepository を返し、
// UseCase が本リポジトリを自身のトランザクションに参加させられるようにします。
// レシーバ自身は変更しません。
func (r *UserSessionRepository) WithTx(tx pgx.Tx) *UserSessionRepository {
	return &UserSessionRepository{q: r.q.WithTx(tx)}
}

// FindByToken returns the session with the given token, or (nil, nil) when none
// exists. A missing session is a normal lookup outcome (e.g. a stale or forged
// cookie), not an error; the caller treats it as "not signed in".
//
// [Ja] FindByToken は指定 token のセッションを返し、存在しない場合は (nil, nil) を
// 返します。セッションの未存在は (例: 失効した / 偽造された Cookie など) 正常な
// ルックアップ結果でありエラーではありません。呼び出し側は「未サインイン」として
// 扱います。
func (r *UserSessionRepository) FindByToken(ctx context.Context, token string) (*model.UserSession, error) {
	row, err := r.q.GetUserSessionByToken(ctx, token)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return r.toModel(row), nil
}

// CreateUserSessionInput holds the attributes needed to create a session. id and
// the timestamps (signed_in_at, created_at, updated_at) are assigned by the
// database.
//
// [Ja] CreateUserSessionInput はセッション作成に必要な属性を保持します。id と
// タイムスタンプ (signed_in_at / created_at / updated_at) は DB 側で採番されます。
type CreateUserSessionInput struct {
	UserID    model.UserID
	Token     string
	IPAddress string
	UserAgent string
}

// Create inserts a session and returns it with the database-assigned id and
// timestamps populated.
//
// [Ja] Create はセッションを挿入し、DB が採番した id とタイムスタンプを設定した
// 状態で返します。
func (r *UserSessionRepository) Create(ctx context.Context, input CreateUserSessionInput) (*model.UserSession, error) {
	row, err := r.q.CreateUserSession(ctx, query.CreateUserSessionParams{
		UserID:    uuid.UUID(input.UserID),
		Token:     input.Token,
		IpAddress: input.IPAddress,
		UserAgent: input.UserAgent,
	})
	if err != nil {
		return nil, err
	}
	return r.toModel(row), nil
}

// DeleteByToken removes the session identified by token. Deleting a token that
// no longer exists is not an error, so signing out twice is harmless.
//
// [Ja] DeleteByToken は token で識別されるセッションを削除します。既に存在しない
// token の削除はエラーにならないため、二重サインアウトも無害です。
func (r *UserSessionRepository) DeleteByToken(ctx context.Context, token string) error {
	return r.q.DeleteUserSessionByToken(ctx, token)
}

// toModel converts a query.UserSession row into a model.UserSession, casting the
// raw uuids into the typed IDs at the repository boundary.
//
// [Ja] toModel は query.UserSession を model.UserSession に変換し、リポジトリの
// 境界で生の uuid を型付き ID にキャストします。
func (r *UserSessionRepository) toModel(row query.UserSession) *model.UserSession {
	return &model.UserSession{
		ID:         model.UserSessionID(row.ID),
		UserID:     model.UserID(row.UserID),
		Token:      row.Token,
		IPAddress:  row.IpAddress,
		UserAgent:  row.UserAgent,
		SignedInAt: row.SignedInAt,
		CreatedAt:  row.CreatedAt,
		UpdatedAt:  row.UpdatedAt,
	}
}
