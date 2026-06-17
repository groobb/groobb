package testutil

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/groobb/groobb/go/internal/auth"
	"github.com/groobb/groobb/go/internal/model"
)

// DefaultBuilderPassword is the plaintext password a UserPasswordBuilder hashes
// when none is set, so a test that just needs a credential can sign in with this
// known value.
//
// [Ja] DefaultBuilderPassword は UserPasswordBuilder が未設定時にハッシュ化する平文
// パスワードです。資格情報を用意するだけのテストが、この既知の値でサインインできる
// ようにします。
const DefaultBuilderPassword = "password123"

// UserPasswordBuilder builds a user_passwords row for tests via a fluent API. The
// owning user is required and has no default, since a password always belongs to
// an existing user. The password defaults to DefaultBuilderPassword and is
// bcrypt-hashed on Build, so tests seed a working credential without hashing it
// themselves.
//
// [Ja] UserPasswordBuilder はテスト用の user_passwords 行を fluent API で組み立てます。
// パスワードは常に既存ユーザーに属するため、所有ユーザーは必須で既定値はありません。
// パスワードは DefaultBuilderPassword を既定とし Build 時に bcrypt ハッシュ化するため、
// テストは自前でハッシュ化せずに有効な資格情報を投入できます。
type UserPasswordBuilder struct {
	t        *testing.T
	tx       pgx.Tx
	userID   model.UserID
	password string
}

// NewUserPasswordBuilder creates a UserPasswordBuilder with the default plaintext
// password.
//
// [Ja] NewUserPasswordBuilder は既定の平文パスワードを持つ UserPasswordBuilder を
// 生成します。
func NewUserPasswordBuilder(t *testing.T, tx pgx.Tx) *UserPasswordBuilder {
	t.Helper()
	return &UserPasswordBuilder{
		t:        t,
		tx:       tx,
		password: DefaultBuilderPassword,
	}
}

// WithUserID sets the owning user.
//
// [Ja] WithUserID は所有ユーザーを設定します。
func (b *UserPasswordBuilder) WithUserID(userID model.UserID) *UserPasswordBuilder {
	b.userID = userID
	return b
}

// WithPassword sets the plaintext password to hash, for tests that need to sign
// in with a specific known password.
//
// [Ja] WithPassword はハッシュ化する平文パスワードを設定します。特定の既知の
// パスワードでサインインする必要があるテストで使います。
func (b *UserPasswordBuilder) WithPassword(password string) *UserPasswordBuilder {
	b.password = password
	return b
}

// Build hashes the password, inserts the credential, and returns its
// database-assigned ID, failing the test on error. id and timestamps are left to
// the database defaults. It fails the test when no user has been set, since
// user_id is NOT NULL.
//
// [Ja] Build はパスワードをハッシュ化して資格情報を挿入し、DB が採番した ID を
// 返します。エラー時はテストを失敗させます。id とタイムスタンプは DB の既定値に
// 任せます。user_id は NOT NULL のため、ユーザーが未設定の場合はテストを失敗させます。
func (b *UserPasswordBuilder) Build() model.UserPasswordID {
	b.t.Helper()

	if b.userID == (model.UserID{}) {
		b.t.Fatal("UserPasswordBuilder にはユーザー ID が必要です (WithUserID で設定してください)")
	}

	digest, err := auth.HashPassword(b.password)
	if err != nil {
		b.t.Fatalf("テスト用パスワードのハッシュ化に失敗: %v", err)
	}

	var id uuid.UUID
	err = b.tx.QueryRow(context.Background(),
		`INSERT INTO user_passwords (user_id, password_digest) VALUES ($1, $2) RETURNING id`,
		uuid.UUID(b.userID), digest,
	).Scan(&id)
	if err != nil {
		b.t.Fatalf("テスト用ユーザーパスワードの作成に失敗: %v", err)
	}

	return model.UserPasswordID(id)
}
