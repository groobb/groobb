package testutil

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

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
	t         *testing.T
	tx        pgx.Tx
	email     string
	atname    string
	locale    string
	timeZone  string
	deletedAt *time.Time
}

// NewUserBuilder creates a UserBuilder. The default email and atname each embed a
// random UUID so concurrent tests (t.Parallel) do not collide on the users.email
// or users.atname UNIQUE constraint without each test having to pick a distinct
// value. The default atname strips the UUID's hyphens and truncates it to stay
// within the atname format (ASCII letters/digits/underscore, 20 chars max).
//
// [Ja] NewUserBuilder は UserBuilder を生成します。既定の email と atname はそれぞれ
// ランダムな UUID を埋め込み、並行テスト (t.Parallel) が各自で別値を選ばずとも
// users.email / users.atname の UNIQUE 制約で衝突しないようにします。既定の atname は
// UUID のハイフンを除き切り詰めて、atname の形式 (ASCII 英数字 / アンダースコア・最大
// 20 文字) に収めます。
func NewUserBuilder(t *testing.T, tx pgx.Tx) *UserBuilder {
	t.Helper()
	return &UserBuilder{
		t:        t,
		tx:       tx,
		email:    fmt.Sprintf("test-%s@example.com", uuid.NewString()),
		atname:   UniqueAtname(),
		locale:   "ja",
		timeZone: "Asia/Tokyo",
	}
}

// UniqueAtname returns a random, format-compliant atname (a leading letter plus
// 15 hex chars, 16 total) for tests that create users directly (not via
// UserBuilder) and commit the rows, so they do not collide on the users.atname
// UNIQUE constraint. It strips the UUID's hyphens and truncates it to stay within
// the atname format (ASCII letters/digits/underscore, 20 chars max).
//
// [Ja] UniqueAtname は形式適合のランダムな atname (先頭の英字 + 16 進 15 文字の計 16
// 文字) を返す。UserBuilder を介さず直接ユーザーを作成し行をコミットするテストが
// users.atname の UNIQUE 制約で衝突しないようにするためのもの。UUID のハイフンを除き
// 切り詰めて atname の形式 (ASCII 英数字 / アンダースコア・最大 20 文字) に収める。
func UniqueAtname() string {
	return "u" + strings.ReplaceAll(uuid.NewString(), "-", "")[:15]
}

// WithEmail sets the email.
//
// [Ja] WithEmail は email を設定します。
func (b *UserBuilder) WithEmail(email string) *UserBuilder {
	b.email = email
	return b
}

// WithAtname sets the atname.
//
// [Ja] WithAtname は atname を設定します。
func (b *UserBuilder) WithAtname(atname string) *UserBuilder {
	b.atname = atname
	return b
}

// WithLocale sets the locale.
//
// [Ja] WithLocale は locale を設定します。
func (b *UserBuilder) WithLocale(locale string) *UserBuilder {
	b.locale = locale
	return b
}

// WithTimeZone sets the time zone.
//
// [Ja] WithTimeZone は time zone を設定します。
func (b *UserBuilder) WithTimeZone(timeZone string) *UserBuilder {
	b.timeZone = timeZone
	return b
}

// WithDeletedAt soft-deletes the user at the given time, so tests can exercise
// how a withdrawn user is treated (e.g. that authentication lookups exclude it).
// Left unset, Build creates an active user (deleted_at NULL).
//
// [Ja] WithDeletedAt は指定時刻でユーザーを論理削除し、退会済みユーザーの扱い
// (例: 認証ルックアップが除外すること) をテストで再現できるようにします。未設定なら
// Build はアクティブなユーザー (deleted_at が NULL) を作ります。
func (b *UserBuilder) WithDeletedAt(deletedAt time.Time) *UserBuilder {
	b.deletedAt = &deletedAt
	return b
}

// Build inserts the user and returns its database-assigned ID, failing the test
// on error. id and timestamps are left to the database defaults. deleted_at is
// NULL unless WithDeletedAt set it (a nil *time.Time binds as NULL).
//
// [Ja] Build はユーザーを挿入し、DB が採番した ID を返します。エラー時はテストを
// 失敗させます。id とタイムスタンプは DB の既定値に任せます。deleted_at は WithDeletedAt で
// 設定しない限り NULL です (nil の *time.Time は NULL としてバインドされます)。
func (b *UserBuilder) Build() model.UserID {
	b.t.Helper()

	var id uuid.UUID
	err := b.tx.QueryRow(context.Background(),
		`INSERT INTO users (email, atname, locale, time_zone, deleted_at) VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		b.email, b.atname, b.locale, b.timeZone, b.deletedAt,
	).Scan(&id)
	if err != nil {
		b.t.Fatalf("テスト用ユーザーの作成に失敗: %v", err)
	}

	return model.UserID(id)
}
