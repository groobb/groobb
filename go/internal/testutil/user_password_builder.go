package testutil

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/auth"
	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/model"
)

// DefaultBuilderPasswordはUserPasswordBuilderが未設定時にハッシュ化する平文
// パスワードです。資格情報を用意するだけのテストが、この既知の値でサインインできる
// ようにします。
const DefaultBuilderPassword = "password123"

// UserPasswordBuilderはテスト用のuser_passwords行をfluent APIで組み立てます。
// パスワードは常に既存ユーザーに属するため、所有ユーザーは必須で既定値はありません。
// パスワードはDefaultBuilderPasswordを既定としBuild時にbcryptハッシュ化するため、
// テストは自前でハッシュ化せずに有効な資格情報を投入できます。
type UserPasswordBuilder struct {
	t        *testing.T
	db       *database.DB
	userID   model.UserID
	password string
}

// NewUserPasswordBuilderは既定の平文パスワードを持つUserPasswordBuilderを
// 生成します。
func NewUserPasswordBuilder(t *testing.T, db *database.DB) *UserPasswordBuilder {
	t.Helper()
	return &UserPasswordBuilder{
		t:        t,
		db:       db,
		password: DefaultBuilderPassword,
	}
}

// WithUserIDは所有ユーザーを設定します。
func (b *UserPasswordBuilder) WithUserID(userID model.UserID) *UserPasswordBuilder {
	b.userID = userID
	return b
}

// WithPasswordはハッシュ化する平文パスワードを設定します。特定の既知の
// パスワードでサインインする必要があるテストで使います。
func (b *UserPasswordBuilder) WithPassword(password string) *UserPasswordBuilder {
	b.password = password
	return b
}

// Buildはパスワードをハッシュ化して資格情報を挿入し、DBが採番したIDを
// 返します。エラー時はテストを失敗させます。idとタイムスタンプはDBの既定値に
// 任せます。user_idはNOT NULLのため、ユーザーが未設定の場合はテストを失敗させます。
func (b *UserPasswordBuilder) Build() model.UserPasswordID {
	b.t.Helper()

	if b.userID == 0 {
		b.t.Fatal("UserPasswordBuilderにはユーザーIDが必要です (WithUserIDで設定してください)")
	}

	digest, err := auth.HashPassword(b.password)
	if err != nil {
		b.t.Fatalf("テスト用パスワードのハッシュ化に失敗: %v", err)
	}

	var id int64
	err = b.db.Writer.QueryRowContext(context.Background(),
		`INSERT INTO user_passwords (user_id, password_digest) VALUES (?, ?) RETURNING id`,
		int64(b.userID), digest,
	).Scan(&id)
	if err != nil {
		b.t.Fatalf("テスト用ユーザーパスワードの作成に失敗: %v", err)
	}

	return model.UserPasswordID(id)
}
