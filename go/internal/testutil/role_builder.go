package testutil

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/model"
)

// RoleBuilderはテスト用のroles行をfluent APIで組み立てます。roles.nameは一意で
// あり、割当はその名前でロールに辿り着くため、名前は必須で既定値はありません。スコープの
// 既定は空で、列自身の既定値に一致します。
//
// アプリケーションにロールを書くコードはありません。インスタンスが持つロールは
// マイグレーションが作るものであり、RoleRepositoryは読むだけです。それでもテストには
// ロールを作る手立てが要ります。マイグレーション済みのデータベースが持つロールは組み込みの
// adminだけである一方、認可の判定が何を許すかを決めるのは、それより狭いロールであるため
// です。
type RoleBuilder struct {
	t      *testing.T
	db     *database.DB
	name   model.RoleName
	scopes string
}

// NewRoleBuilderは、スコープを1つも持たないRoleBuilderを生成します。
func NewRoleBuilder(t *testing.T, db *database.DB) *RoleBuilder {
	t.Helper()
	return &RoleBuilder{
		t:      t,
		db:     db,
		scopes: "[]",
	}
}

// WithNameはロールの名前を設定します。割当とルックアップがロールに辿り着くのは
// この名前によってです。
func (b *RoleBuilder) WithName(name model.RoleName) *RoleBuilder {
	b.name = name
	return b
}

// WithScopesは、ロールが与えるスコープを設定します。model.Scopesに無い名前が
// 混じっていてもかまいません。バイナリより先にマイグレートされたデータベースが返して
// くるのがそれであるためです。
//
// WithRawScopesと同じフィールドを書くため、後に呼んだほうが列を決めます。
func (b *RoleBuilder) WithScopes(scopes []model.Scope) *RoleBuilder {
	b.t.Helper()

	// nilのスライスはnullとして符号化され、列のチェック制約がそれを拒む。空の集合は
	// 空の配列である。
	encoded := make([]model.Scope, 0, len(scopes))
	encoded = append(encoded, scopes...)

	// SQLiteに配列型は無く列はJSON配列を保持するため、ビルダーはリポジトリが
	// 読み戻すのと同じテキストを書く。
	text, err := json.Marshal(encoded)
	if err != nil {
		b.t.Fatalf("テスト用スコープのエンコードに失敗: %v", err)
	}

	b.scopes = string(text)
	return b
}

// WithRawScopesはscopes列をそのまま設定します。ロールが何を与えるかではなく、列が
// 何を持ちうるかを主題とするテストのためのものです。列のチェック制約はJSON配列なら何でも
// 許すため、要素が文字列でない行は、データベースが受け入れる一方でリポジトリがモデルに
// 変換できない行になります。テストがその経路に辿り着く手立てが、列を直接書くことです。
//
// WithScopesと同じフィールドを書くため、後に呼んだほうが列を決めます。
func (b *RoleBuilder) WithRawScopes(scopes string) *RoleBuilder {
	b.scopes = scopes
	return b
}

// Buildはロールを挿入し、DBが採番したIDを返します。エラー時はテストを失敗させ
// ます。idとタイムスタンプはDBの既定値に任せます。roles.nameはNOT NULLであり、割当が
// ロールを指すのもこの名前であるため、名前が未設定の場合はテストを失敗させます。
func (b *RoleBuilder) Build() model.RoleID {
	b.t.Helper()

	if b.name == "" {
		b.t.Fatal("RoleBuilderにはロール名が必要です (WithNameで設定してください)")
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
