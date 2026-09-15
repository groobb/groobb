package testutil

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/sqlitetime"
)

// PasswordResetTokenBuilderはテスト用のpassword_reset_tokens行をfluent APIで
// 組み立てます。妥当な既定値を適用するため、テストは関心のあるフィールドだけを設定すれば
// 済みます。トークンは常に既存ユーザーに属するため、所有ユーザーは必須で既定値は
// ありません。
type PasswordResetTokenBuilder struct {
	t           *testing.T
	db          *database.DB
	userID      model.UserID
	tokenDigest string
	expiresAt   time.Time
	usedAt      *time.Time
}

// NewPasswordResetTokenBuilderはPasswordResetTokenBuilderを生成します。
// 既定のダイジェストはそのテストの次の連番を持つため、複数のトークンを作るテストが
// token_digestのUNIQUE制約で互いを区別するためにダイジェストを一つずつ決める必要は
// ありません。既定の有効期限は有効期間1つ分先 (発行直後の使えるトークン) です。
func NewPasswordResetTokenBuilder(t *testing.T, db *database.DB) *PasswordResetTokenBuilder {
	t.Helper()
	return &PasswordResetTokenBuilder{
		t:           t,
		db:          db,
		tokenDigest: fmt.Sprintf("digest-%d", nextSequence(db)),
		expiresAt:   time.Now().Add(model.PasswordResetTokenExpirationDuration),
	}
}

// WithUserIDは所有ユーザーを設定します。
func (b *PasswordResetTokenBuilder) WithUserID(userID model.UserID) *PasswordResetTokenBuilder {
	b.userID = userID
	return b
}

// WithTokenDigestは保存ハッシュを設定します。特定のダイジェスト (例: 既知の平文
// リセットトークンのダイジェスト) でトークンを引く必要があるテストで使います。
func (b *PasswordResetTokenBuilder) WithTokenDigest(tokenDigest string) *PasswordResetTokenBuilder {
	b.tokenDigest = tokenDigest
	return b
}

// WithExpiresAtはexpires_atを上書きします。テストは過去の時刻を渡して期限切れの
// トークンを作ります。未設定なら有効期間1つ分先を既定とします。
func (b *PasswordResetTokenBuilder) WithExpiresAt(expiresAt time.Time) *PasswordResetTokenBuilder {
	b.expiresAt = expiresAt
	return b
}

// WithUsedAtはused_atを打刻して消費済みのトークンを作ります。未設定ならused_atは
// NULL (未使用トークン) です。
func (b *PasswordResetTokenBuilder) WithUsedAt(usedAt time.Time) *PasswordResetTokenBuilder {
	b.usedAt = &usedAt
	return b
}

// Buildはトークンを挿入し、DBが採番したIDを返します。エラー時はテストを失敗
// させます。idとタイムスタンプはDBの既定値に任せます。user_idはNOT NULLのため、
// ユーザーが未設定の場合はテストを失敗させます。
func (b *PasswordResetTokenBuilder) Build() model.PasswordResetTokenID {
	b.t.Helper()

	if b.userID == 0 {
		b.t.Fatal("PasswordResetTokenBuilderにはユーザーIDが必要です (WithUserIDで設定してください)")
	}

	var id int64
	err := b.db.Writer.QueryRowContext(context.Background(),
		`INSERT INTO password_reset_tokens (user_id, token_digest, expires_at, used_at) VALUES (?, ?, ?, ?) RETURNING id`,
		int64(b.userID), b.tokenDigest, sqlitetime.Time(b.expiresAt), sqlitetime.Ptr(b.usedAt),
	).Scan(&id)
	if err != nil {
		b.t.Fatalf("テスト用パスワードリセットトークンの作成に失敗: %v", err)
	}

	return model.PasswordResetTokenID(id)
}
