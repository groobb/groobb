package testutil

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/groobb/groobb/go/internal/model"
)

// UserBuilder builds a users row for tests via a fluent API, applying sensible
// defaults so a test only sets the fields it cares about.
//
// [Ja] UserBuilder はテスト用の users 行を fluent API で組み立てます。妥当な既定値を
// 適用するため、テストは関心のあるフィールドだけを設定すれば済みます。
type UserBuilder struct {
	t        *testing.T
	tx       pgx.Tx
	email    string
	locale   string
	timeZone string
}

// NewUserBuilder creates a UserBuilder. The default email embeds a random UUID
// so concurrent tests (t.Parallel) do not collide on the users.email UNIQUE
// constraint without each test having to pick a distinct address.
//
// [Ja] NewUserBuilder は UserBuilder を生成します。既定の email にはランダムな UUID を
// 埋め込み、並行テスト (t.Parallel) が各自で別アドレスを選ばずとも users.email の
// UNIQUE 制約で衝突しないようにします。
func NewUserBuilder(t *testing.T, tx pgx.Tx) *UserBuilder {
	t.Helper()
	return &UserBuilder{
		t:        t,
		tx:       tx,
		email:    fmt.Sprintf("test-%s@example.com", uuid.NewString()),
		locale:   "ja",
		timeZone: "Asia/Tokyo",
	}
}

// WithEmail sets the email.
// [Ja] WithEmail は email を設定します。
func (b *UserBuilder) WithEmail(email string) *UserBuilder {
	b.email = email
	return b
}

// WithLocale sets the locale.
// [Ja] WithLocale は locale を設定します。
func (b *UserBuilder) WithLocale(locale string) *UserBuilder {
	b.locale = locale
	return b
}

// WithTimeZone sets the time zone.
// [Ja] WithTimeZone は time zone を設定します。
func (b *UserBuilder) WithTimeZone(timeZone string) *UserBuilder {
	b.timeZone = timeZone
	return b
}

// Build inserts the user and returns its database-assigned ID, failing the test
// on error. id and timestamps are left to the database defaults.
//
// [Ja] Build はユーザーを挿入し、DB が採番した ID を返します。エラー時はテストを
// 失敗させます。id とタイムスタンプは DB の既定値に任せます。
func (b *UserBuilder) Build() model.UserID {
	b.t.Helper()

	var id uuid.UUID
	err := b.tx.QueryRow(context.Background(),
		`INSERT INTO users (email, locale, time_zone) VALUES ($1, $2, $3) RETURNING id`,
		b.email, b.locale, b.timeZone,
	).Scan(&id)
	if err != nil {
		b.t.Fatalf("テスト用ユーザーの作成に失敗: %v", err)
	}

	return model.UserID(id)
}
