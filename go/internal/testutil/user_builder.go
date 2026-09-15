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

// UserBuilderはテスト用のusers行をfluent APIで組み立てます。妥当な既定値を
// 適用するため、テストは関心のあるフィールドだけを設定すれば済みます。
type UserBuilder struct {
	t           *testing.T
	db          *database.DB
	email       string
	atname      string
	locale      model.Locale
	timeZone    string
	deletedAt   *time.Time
	suspendedAt *time.Time
}

// NewUserBuilderはUserBuilderを生成します。既定のemailとatnameはそのデータ
// ベースの次の連番を持つため、複数のユーザーを作るテストがusers.email / users.atnameの
// UNIQUE制約で互いを区別するために一つずつ名前を決める必要はありません。
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

// UniqueAtnameは形式に適合するatname (先頭の英字 + そのデータベースの次の連番) を
// 返します。UserBuilderを介さず直接ユーザーを作成するテストが、1つのデータベース内の
// 複数ユーザーでusers.atnameのUNIQUE制約に衝突しないようにするためのものです。値を
// atnameの形式 (ASCII英数字とアンダースコアを許す) の内側に保つのが先頭の英字です。
func UniqueAtname(db *database.DB) string {
	return fmt.Sprintf("u%d", nextSequence(db))
}

// UniqueEmailはそのデータベースの次の連番を持つメールアドレスを返します。
// UserBuilderを介さず直接ユーザーを作成し、1つのデータベース内で複数のアドレスを
// users.emailのUNIQUE制約に衝突させずに保つ必要があるテストのためのものです。prefixは
// そのアドレスが何のためのものかを表すため、失敗した検証はどのフィクスチャ由来かを示せます。
func UniqueEmail(db *database.DB, prefix string) string {
	return fmt.Sprintf("%s-%d@example.com", prefix, nextSequence(db))
}

// WithEmailはemailを設定します。
func (b *UserBuilder) WithEmail(email string) *UserBuilder {
	b.email = email
	return b
}

// WithAtnameはatnameを設定します。
func (b *UserBuilder) WithAtname(atname string) *UserBuilder {
	b.atname = atname
	return b
}

// WithLocaleはlocaleを設定します。
func (b *UserBuilder) WithLocale(locale model.Locale) *UserBuilder {
	b.locale = locale
	return b
}

// WithTimeZoneはtime zoneを設定します。
func (b *UserBuilder) WithTimeZone(timeZone string) *UserBuilder {
	b.timeZone = timeZone
	return b
}

// WithDeletedAtは指定時刻でユーザーを論理削除し、退会済みユーザーの扱い
// (例: 認証ルックアップが除外すること) をテストで再現できるようにします。未設定なら
// Buildはアクティブなユーザー (deleted_atがNULL) を作ります。
func (b *UserBuilder) WithDeletedAt(deletedAt time.Time) *UserBuilder {
	b.deletedAt = &deletedAt
	return b
}

// WithSuspendedAtは指定時刻で利用者を停止し、停止されたアカウントの扱い (セッション
// から解決されないこと、ロールの保持者として数えられないこと) をテストで再現できるように
// します。未設定ならBuildは行動できるアカウント (suspended_atがNULL) を作ります。
//
// 時刻をデータベースの時計からではなく与えるのは、ここでテストが用意するのがアカウントの
// 状態であって、そこへ至った瞬間ではないためです。
func (b *UserBuilder) WithSuspendedAt(suspendedAt time.Time) *UserBuilder {
	b.suspendedAt = &suspendedAt
	return b
}

// Buildはユーザーを挿入し、DBが採番したIDを返します。エラー時はテストを
// 失敗させます。idとタイムスタンプはDBの既定値に任せます。deleted_atとsuspended_atは
// WithDeletedAt・WithSuspendedAtで設定しない限りNULLです (nilの時刻はNULLとして
// バインドされます)。
func (b *UserBuilder) Build() model.UserID {
	b.t.Helper()

	var id int64
	err := b.db.Writer.QueryRowContext(context.Background(),
		`INSERT INTO users (email, atname, locale, time_zone, deleted_at, suspended_at) VALUES (?, ?, ?, ?, ?, ?) RETURNING id`,
		b.email, b.atname, string(b.locale), b.timeZone, sqlitetime.Ptr(b.deletedAt), sqlitetime.Ptr(b.suspendedAt),
	).Scan(&id)
	if err != nil {
		b.t.Fatalf("テスト用ユーザーの作成に失敗: %v", err)
	}

	return model.UserID(id)
}
