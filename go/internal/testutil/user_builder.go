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

// UserBuilder builds a users row for tests via a fluent API, applying sensible
// defaults so a test only sets the fields it cares about.
//
// [Ja] UserBuilder はテスト用の users 行を fluent API で組み立てます。妥当な既定値を
// 適用するため、テストは関心のあるフィールドだけを設定すれば済みます。
type UserBuilder struct {
	t         *testing.T
	db        *database.DB
	email     string
	atname    string
	locale    model.Locale
	timeZone  string
	deletedAt *time.Time
}

// NewUserBuilder creates a UserBuilder. The default email and atname each carry
// the database's next sequence number, so a test that builds several users does
// not have to name each one to keep them apart on the users.email or users.atname
// UNIQUE constraint.
//
// [Ja] NewUserBuilder は UserBuilder を生成します。既定の email と atname はそのデータ
// ベースの次の連番を持つため、複数のユーザーを作るテストが users.email / users.atname の
// UNIQUE 制約で互いを区別するために一つずつ名前を決める必要はありません。
func NewUserBuilder(t *testing.T, db *database.DB) *UserBuilder {
	t.Helper()

	sequence := nextSequence(db)
	return &UserBuilder{
		t:        t,
		db:       db,
		email:    fmt.Sprintf("test-%d@example.com", sequence),
		atname:   fmt.Sprintf("u%d", sequence),
		locale:   model.DefaultLocale,
		timeZone: "Asia/Tokyo",
	}
}

// UniqueAtname returns a format-compliant atname (a leading letter plus the
// database's next sequence number) for tests that create users directly rather
// than through UserBuilder, so several users in one database do not collide on
// the users.atname UNIQUE constraint. The leading letter is what keeps the value
// inside the atname format, which allows ASCII letters, digits, and underscore.
//
// [Ja] UniqueAtname は形式に適合する atname (先頭の英字 + そのデータベースの次の連番) を
// 返します。UserBuilder を介さず直接ユーザーを作成するテストが、1 つのデータベース内の
// 複数ユーザーで users.atname の UNIQUE 制約に衝突しないようにするためのものです。値を
// atname の形式 (ASCII 英数字とアンダースコアを許す) の内側に保つのが先頭の英字です。
func UniqueAtname(db *database.DB) string {
	return fmt.Sprintf("u%d", nextSequence(db))
}

// UniqueEmail returns an email address carrying the database's next sequence
// number, for tests that create users directly rather than through UserBuilder
// and need several addresses in one database to stay apart on the users.email
// UNIQUE constraint. The prefix names what the address is for, so a failing
// assertion still says which fixture it came from.
//
// [Ja] UniqueEmail はそのデータベースの次の連番を持つメールアドレスを返します。
// UserBuilder を介さず直接ユーザーを作成し、1 つのデータベース内で複数のアドレスを
// users.email の UNIQUE 制約に衝突させずに保つ必要があるテストのためのものです。prefix は
// そのアドレスが何のためのものかを表すため、失敗した検証はどのフィクスチャ由来かを示せます。
func UniqueEmail(db *database.DB, prefix string) string {
	return fmt.Sprintf("%s-%d@example.com", prefix, nextSequence(db))
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
func (b *UserBuilder) WithLocale(locale model.Locale) *UserBuilder {
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
// NULL unless WithDeletedAt set it (a nil timestamp binds as NULL).
//
// [Ja] Build はユーザーを挿入し、DB が採番した ID を返します。エラー時はテストを
// 失敗させます。id とタイムスタンプは DB の既定値に任せます。deleted_at は WithDeletedAt で
// 設定しない限り NULL です (nil の時刻は NULL としてバインドされます)。
func (b *UserBuilder) Build() model.UserID {
	b.t.Helper()

	var id int64
	err := b.db.Writer.QueryRowContext(context.Background(),
		`INSERT INTO users (email, atname, locale, time_zone, deleted_at) VALUES (?, ?, ?, ?, ?) RETURNING id`,
		b.email, b.atname, string(b.locale), b.timeZone, sqlitetime.Ptr(b.deletedAt),
	).Scan(&id)
	if err != nil {
		b.t.Fatalf("テスト用ユーザーの作成に失敗: %v", err)
	}

	return model.UserID(id)
}
