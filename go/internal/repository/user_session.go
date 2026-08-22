package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/query"
)

// UserSessionRepository reads and writes user_sessions through sqlc-generated
// queries.
//
// [Ja] UserSessionRepository は sqlc 生成のクエリ経由で user_sessions を読み書き
// します。
type UserSessionRepository struct {
	reader *query.Queries
	writer *query.Queries
}

// NewUserSessionRepository creates a UserSessionRepository that reads through the database's read pool
// and writes through its write pool.
//
// [Ja] NewUserSessionRepository は、データベースの読み取り用プールで読み、書き込み用プールで書く
// UserSessionRepository を生成します。
func NewUserSessionRepository(db *database.DB) *UserSessionRepository {
	return &UserSessionRepository{reader: query.New(db.Reader), writer: query.New(db.Writer)}
}

// WithTx returns a new UserSessionRepository whose queries run inside tx, so a
// UseCase can enlist this repository in its transaction. The receiver is left
// unchanged.
//
// [Ja] WithTx は queries を tx 内で実行する新しい UserSessionRepository を返し、
// UseCase が本リポジトリを自身のトランザクションに参加させられるようにします。
// レシーバ自身は変更しません。
func (r *UserSessionRepository) WithTx(tx *sql.Tx) *UserSessionRepository {
	q := r.writer.WithTx(tx)
	return &UserSessionRepository{reader: q, writer: q}
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
	row, err := r.reader.GetUserSessionByToken(ctx, token)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
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
	row, err := r.writer.CreateUserSession(ctx, query.CreateUserSessionParams{
		UserID:    int64(input.UserID),
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
	return r.writer.DeleteUserSessionByToken(ctx, token)
}

// DeleteByUserID removes every session owned by the given user, signing them out
// on all devices at once. It is used by the withdrawal flow so a withdrawn user's
// live sessions cannot keep resolving to their account. Deleting when the user has
// no sessions is not an error, so callers need not check first.
//
// [Ja] DeleteByUserID は指定ユーザーが所有する全セッションを削除し、全端末で一括
// サインアウトさせます。退会フローで使い、退会済みユーザーの有効なセッションが
// アカウントに解決し続けないようにします。ユーザーがセッションを持たないときに削除しても
// エラーにならないため、呼び出し側で事前確認は不要です。
func (r *UserSessionRepository) DeleteByUserID(ctx context.Context, userID model.UserID) error {
	return r.writer.DeleteUserSessionsByUserID(ctx, int64(userID))
}

// toModel converts a query.UserSession row into a model.UserSession, casting the
// raw ids into the typed IDs at the repository boundary.
//
// [Ja] toModel は query.UserSession を model.UserSession に変換し、リポジトリの
// 境界で生の id を型付き ID にキャストします。
func (r *UserSessionRepository) toModel(row query.UserSession) *model.UserSession {
	return &model.UserSession{
		ID:         model.UserSessionID(row.ID),
		UserID:     model.UserID(row.UserID),
		Token:      row.Token,
		IPAddress:  row.IpAddress,
		UserAgent:  row.UserAgent,
		SignedInAt: time.Time(row.SignedInAt),
		CreatedAt:  time.Time(row.CreatedAt),
		UpdatedAt:  time.Time(row.UpdatedAt),
	}
}
