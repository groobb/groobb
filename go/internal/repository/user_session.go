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

// UserSessionRepositoryはsqlc生成のクエリ経由でuser_sessionsを読み書き
// します。
type UserSessionRepository struct {
	reader *query.Queries
	writer *query.Queries
}

// NewUserSessionRepositoryは、データベースの読み取り用プールで読み、書き込み用プールで書く
// UserSessionRepositoryを生成します。
func NewUserSessionRepository(db *database.DB) *UserSessionRepository {
	return &UserSessionRepository{reader: query.New(db.Reader), writer: query.New(db.Writer)}
}

// WithTxはqueriesをtx内で実行する新しいUserSessionRepositoryを返し、
// UseCaseが本リポジトリを自身のトランザクションに参加させられるようにします。
// レシーバ自身は変更しません。
func (r *UserSessionRepository) WithTx(tx *sql.Tx) *UserSessionRepository {
	q := r.writer.WithTx(tx)
	return &UserSessionRepository{reader: q, writer: q}
}

// FindByTokenは指定tokenのセッションを返し、存在しない場合は (nil, nil) を
// 返します。セッションの未存在は (例: 失効した / 偽造されたCookieなど) 正常な
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

// CreateUserSessionInputはセッション作成に必要な属性を保持します。idと
// タイムスタンプ (signed_in_at / created_at / updated_at) はDB側で採番されます。
type CreateUserSessionInput struct {
	UserID    model.UserID
	Token     string
	IPAddress string
	UserAgent string
}

// Createはセッションを挿入し、DBが採番したidとタイムスタンプを設定した
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

// DeleteByTokenはtokenで識別されるセッションを削除します。既に存在しない
// tokenの削除はエラーにならないため、二重サインアウトも無害です。
func (r *UserSessionRepository) DeleteByToken(ctx context.Context, token string) error {
	return r.writer.DeleteUserSessionByToken(ctx, token)
}

// DeleteByUserIDは指定ユーザーが所有する全セッションを削除し、全端末で一括
// サインアウトさせます。退会フローで使い、退会済みユーザーの有効なセッションが
// アカウントに解決し続けないようにします。ユーザーがセッションを持たないときに削除しても
// エラーにならないため、呼び出し側で事前確認は不要です。
func (r *UserSessionRepository) DeleteByUserID(ctx context.Context, userID model.UserID) error {
	return r.writer.DeleteUserSessionsByUserID(ctx, int64(userID))
}

// toModelはquery.UserSessionをmodel.UserSessionに変換し、リポジトリの
// 境界で生のidを型付きIDにキャストします。
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
