package testutil

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/model"
)

// RoleBuilder builds a roles row for tests via a fluent API. The name is
// required and has no default, since roles.name is unique and an assignment
// reaches a role by that name. The scopes default to none, matching the column's
// own default.
//
// The application has no code that writes a role: the roles an instance has are
// created by migrations, so RoleRepository reads only. Tests need one anyway,
// because the single role a migrated database holds is the built-in admin,
// while what an authorization check admits is decided by roles narrower than it.
//
// [Ja] RoleBuilder はテスト用の roles 行を fluent API で組み立てます。roles.name は一意で
// あり、割当はその名前でロールに辿り着くため、名前は必須で既定値はありません。スコープの
// 既定は空で、列自身の既定値に一致します。
//
// アプリケーションにロールを書くコードはありません。インスタンスが持つロールは
// マイグレーションが作るものであり、RoleRepository は読むだけです。それでもテストには
// ロールを作る手立てが要ります。マイグレーション済みのデータベースが持つロールは組み込みの
// admin だけである一方、認可の判定が何を許すかを決めるのは、それより狭いロールであるため
// です。
type RoleBuilder struct {
	t      *testing.T
	db     *database.DB
	name   model.RoleName
	scopes string
}

// NewRoleBuilder creates a RoleBuilder carrying no scope.
//
// [Ja] NewRoleBuilder は、スコープを 1 つも持たない RoleBuilder を生成します。
func NewRoleBuilder(t *testing.T, db *database.DB) *RoleBuilder {
	t.Helper()
	return &RoleBuilder{
		t:      t,
		db:     db,
		scopes: "[]",
	}
}

// WithName sets the role's name, which is how an assignment and a lookup reach
// it.
//
// [Ja] WithName はロールの名前を設定します。割当とルックアップがロールに辿り着くのは
// この名前によってです。
func (b *RoleBuilder) WithName(name model.RoleName) *RoleBuilder {
	b.name = name
	return b
}

// WithScopes sets the scopes the role grants. A name outside model.Scopes may be
// among them, since that is what a database migrated ahead of the binary hands
// back.
//
// It writes the same field WithRawScopes does, so whichever is called last
// decides the column.
//
// [Ja] WithScopes は、ロールが与えるスコープを設定します。model.Scopes に無い名前が
// 混じっていてもかまいません。バイナリより先にマイグレートされたデータベースが返して
// くるのがそれであるためです。
//
// WithRawScopes と同じフィールドを書くため、後に呼んだほうが列を決めます。
func (b *RoleBuilder) WithScopes(scopes []model.Scope) *RoleBuilder {
	b.t.Helper()

	// A nil slice encodes as null, which the column's check constraint refuses.
	// The empty set is an empty array.
	//
	// [Ja] nil のスライスは null として符号化され、列のチェック制約がそれを拒む。空の集合は
	// 空の配列である。
	encoded := make([]model.Scope, 0, len(scopes))
	encoded = append(encoded, scopes...)

	// SQLite has no array type, so the column holds a JSON array; the builder
	// writes the same text the repository reads back.
	//
	// [Ja] SQLite に配列型は無く列は JSON 配列を保持するため、ビルダーはリポジトリが
	// 読み戻すのと同じテキストを書く。
	text, err := json.Marshal(encoded)
	if err != nil {
		b.t.Fatalf("テスト用スコープのエンコードに失敗: %v", err)
	}

	b.scopes = string(text)
	return b
}

// WithRawScopes sets the scopes column verbatim, for a test whose subject is
// what the column may hold rather than what a role grants. The column's check
// constraint admits any JSON array, so a row whose elements are not strings is
// one the database accepts and the repository cannot turn into a model; writing
// the column directly is how a test reaches that path.
//
// It writes the same field WithScopes does, so whichever is called last decides
// the column.
//
// [Ja] WithRawScopes は scopes 列をそのまま設定します。ロールが何を与えるかではなく、列が
// 何を持ちうるかを主題とするテストのためのものです。列のチェック制約は JSON 配列なら何でも
// 許すため、要素が文字列でない行は、データベースが受け入れる一方でリポジトリがモデルに
// 変換できない行になります。テストがその経路に辿り着く手立てが、列を直接書くことです。
//
// WithScopes と同じフィールドを書くため、後に呼んだほうが列を決めます。
func (b *RoleBuilder) WithRawScopes(scopes string) *RoleBuilder {
	b.scopes = scopes
	return b
}

// Build inserts the role and returns its database-assigned ID, failing the test
// on error. id and the timestamps are left to the database defaults. It fails
// the test when no name has been set, since roles.name is NOT NULL and an
// assignment addresses the role by it.
//
// [Ja] Build はロールを挿入し、DB が採番した ID を返します。エラー時はテストを失敗させ
// ます。id とタイムスタンプは DB の既定値に任せます。roles.name は NOT NULL であり、割当が
// ロールを指すのもこの名前であるため、名前が未設定の場合はテストを失敗させます。
func (b *RoleBuilder) Build() model.RoleID {
	b.t.Helper()

	if b.name == "" {
		b.t.Fatal("RoleBuilder にはロール名が必要です (WithName で設定してください)")
	}

	var id int64
	err := b.db.Writer.QueryRowContext(context.Background(),
		`INSERT INTO roles (name, scopes) VALUES (?, ?) RETURNING id`,
		string(b.name), b.scopes,
	).Scan(&id)
	if err != nil {
		b.t.Fatalf("テスト用ロールの作成に失敗: %v", err)
	}

	return model.RoleID(id)
}
