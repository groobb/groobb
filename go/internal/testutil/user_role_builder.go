package testutil

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/model"
)

// UserRoleBuilderはテスト用のuser_roles行をfluent APIで組み立てます。ロールを
// 受け取るユーザーは必須で既定値はありません。割当は常に既存のユーザーに属するためです。
type UserRoleBuilder struct {
	t        *testing.T
	db       *database.DB
	userID   model.UserID
	roleName model.RoleName
}

// NewUserRoleBuilderはUserRoleBuilderを生成します。ロールの既定値は、
// マイグレーション済みのどのデータベースも持つ組み込みのadminロールです。誰かが管理者で
// あることだけを必要とするテストは、ユーザー以外に何も指定せずに済みます。
//
// ロールをidではなく名前で指すのは、idがマイグレーションを適用したデータベースごとに
// 採番されるものであり、テストはそれぞれ自分のデータベースを所有するためです。
func NewUserRoleBuilder(t *testing.T, db *database.DB) *UserRoleBuilder {
	t.Helper()
	return &UserRoleBuilder{
		t:        t,
		db:       db,
		roleName: model.RoleNameAdmin,
	}
}

// WithUserIDはロールを受け取るユーザーを設定します。
func (b *UserRoleBuilder) WithUserID(userID model.UserID) *UserRoleBuilder {
	b.userID = userID
	return b
}

// WithRoleNameは割り当てるロールを設定します。組み込みのadmin以外のロールを
// 扱うテストのためのものです。
func (b *UserRoleBuilder) WithRoleName(roleName model.RoleName) *UserRoleBuilder {
	b.roleName = roleName
	return b
}

// Buildはロールを割り当て、DBが採番した割当のIDを返します。エラー時はテストを
// 失敗させます。idとタイムスタンプはDBの既定値に任せます。user_idはNOT NULLのため
// ユーザーが未設定の場合はテストを失敗させ、どのロールも持たない名前はテストが作っていない
// フィクスチャを指しているため、その場合もテストを失敗させます。
func (b *UserRoleBuilder) Build() model.UserRoleID {
	b.t.Helper()

	if b.userID == 0 {
		b.t.Fatal("UserRoleBuilderにはユーザーIDが必要です (WithUserIDで設定してください)")
	}

	ctx := context.Background()

	var roleID int64
	if err := b.db.Writer.QueryRowContext(ctx,
		`SELECT id FROM roles WHERE name = ?`, string(b.roleName),
	).Scan(&roleID); err != nil {
		b.t.Fatalf("テスト用ロール %q の取得に失敗: %v", b.roleName, err)
	}

	var id int64
	err := b.db.Writer.QueryRowContext(ctx,
		`INSERT INTO user_roles (user_id, role_id) VALUES (?, ?) RETURNING id`,
		int64(b.userID), roleID,
	).Scan(&id)
	if err != nil {
		b.t.Fatalf("テスト用ロール割当の作成に失敗: %v", err)
	}

	return model.UserRoleID(id)
}
