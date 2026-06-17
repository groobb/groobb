package testutil

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/groobb/groobb/go/internal/model"
)

// PasswordResetTokenBuilder builds a password_reset_tokens row for tests via a
// fluent API, applying sensible defaults so a test only sets the fields it cares
// about. The owning user is required and has no default, since a token always
// belongs to an existing user.
//
// [Ja] PasswordResetTokenBuilder はテスト用の password_reset_tokens 行を fluent API で
// 組み立てます。妥当な既定値を適用するため、テストは関心のあるフィールドだけを設定すれば
// 済みます。トークンは常に既存ユーザーに属するため、所有ユーザーは必須で既定値は
// ありません。
type PasswordResetTokenBuilder struct {
	t           *testing.T
	tx          pgx.Tx
	userID      model.UserID
	tokenDigest string
	expiresAt   time.Time
	usedAt      *time.Time
}

// NewPasswordResetTokenBuilder creates a PasswordResetTokenBuilder. The default
// digest embeds a random UUID so concurrent tests (t.Parallel) do not collide on
// the token_digest UNIQUE constraint, and the default expiry is one validity
// window into the future (a freshly issued, usable token).
//
// [Ja] NewPasswordResetTokenBuilder は PasswordResetTokenBuilder を生成します。
// 既定のダイジェストにはランダムな UUID を埋め込み、並行テスト (t.Parallel) が
// token_digest の UNIQUE 制約で衝突しないようにします。既定の有効期限は有効期間 1 つ分
// 先 (発行直後の使えるトークン) です。
func NewPasswordResetTokenBuilder(t *testing.T, tx pgx.Tx) *PasswordResetTokenBuilder {
	t.Helper()
	return &PasswordResetTokenBuilder{
		t:           t,
		tx:          tx,
		tokenDigest: "digest-" + uuid.NewString(),
		expiresAt:   time.Now().Add(model.PasswordResetTokenExpirationDuration),
	}
}

// WithUserID sets the owning user.
//
// [Ja] WithUserID は所有ユーザーを設定します。
func (b *PasswordResetTokenBuilder) WithUserID(userID model.UserID) *PasswordResetTokenBuilder {
	b.userID = userID
	return b
}

// WithTokenDigest sets the stored hash, for tests that need to look the token up
// by a specific digest (e.g. the digest of a known plaintext reset token).
//
// [Ja] WithTokenDigest は保存ハッシュを設定します。特定のダイジェスト (例: 既知の平文
// リセットトークンのダイジェスト) でトークンを引く必要があるテストで使います。
func (b *PasswordResetTokenBuilder) WithTokenDigest(tokenDigest string) *PasswordResetTokenBuilder {
	b.tokenDigest = tokenDigest
	return b
}

// WithExpiresAt overrides expires_at. A test sets this to a past time to build an
// expired token; left unset, it defaults to one validity window in the future.
//
// [Ja] WithExpiresAt は expires_at を上書きします。テストは過去の時刻を渡して期限切れの
// トークンを作ります。未設定なら有効期間 1 つ分先を既定とします。
func (b *PasswordResetTokenBuilder) WithExpiresAt(expiresAt time.Time) *PasswordResetTokenBuilder {
	b.expiresAt = expiresAt
	return b
}

// WithUsedAt stamps used_at to build an already-spent token; left unset, used_at
// is NULL (an unused token).
//
// [Ja] WithUsedAt は used_at を打刻して消費済みのトークンを作ります。未設定なら used_at は
// NULL (未使用トークン) です。
func (b *PasswordResetTokenBuilder) WithUsedAt(usedAt time.Time) *PasswordResetTokenBuilder {
	b.usedAt = &usedAt
	return b
}

// Build inserts the token and returns its database-assigned ID, failing the test
// on error. id and the timestamps are left to the database defaults. It fails the
// test when no user has been set, since user_id is NOT NULL.
//
// [Ja] Build はトークンを挿入し、DB が採番した ID を返します。エラー時はテストを失敗
// させます。id とタイムスタンプは DB の既定値に任せます。user_id は NOT NULL のため、
// ユーザーが未設定の場合はテストを失敗させます。
func (b *PasswordResetTokenBuilder) Build() model.PasswordResetTokenID {
	b.t.Helper()

	if b.userID == (model.UserID{}) {
		b.t.Fatal("PasswordResetTokenBuilder にはユーザー ID が必要です (WithUserID で設定してください)")
	}

	var id uuid.UUID
	err := b.tx.QueryRow(context.Background(),
		`INSERT INTO password_reset_tokens (user_id, token_digest, expires_at, used_at) VALUES ($1, $2, $3, $4) RETURNING id`,
		uuid.UUID(b.userID), b.tokenDigest, b.expiresAt, b.usedAt,
	).Scan(&id)
	if err != nil {
		b.t.Fatalf("テスト用パスワードリセットトークンの作成に失敗: %v", err)
	}

	return model.PasswordResetTokenID(id)
}
