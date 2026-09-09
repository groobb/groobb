package testutil

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/model"
)

// UserRoleBuilder builds a user_roles row for tests via a fluent API. The user
// receiving the role is required and has no default, since an assignment always
// belongs to an existing user.
//
// [Ja] UserRoleBuilder はテスト用の user_roles 行を fluent API で組み立てます。ロールを
// 受け取るユーザーは必須で既定値はありません。割当は常に既存のユーザーに属するためです。
type UserRoleBuilder struct {
	t        *testing.T
	db       *database.DB
	userID   model.UserID
	roleName model.RoleName
}

// NewUserRoleBuilder creates a UserRoleBuilder. The role defaults to the
// built-in admin role, which every migrated database holds, so a test that only
// needs someone to be an administrator names nothing but the user.
//
// The role is addressed by name rather than by id because the id is assigned by
// whichever database the migration ran against, and each test owns its own.
//
// [Ja] NewUserRoleBuilder は UserRoleBuilder を生成します。ロールの既定値は、
// マイグレーション済みのどのデータベースも持つ組み込みの admin ロールです。誰かが管理者で
// あることだけを必要とするテストは、ユーザー以外に何も指定せずに済みます。
//
// ロールを id ではなく名前で指すのは、id がマイグレーションを適用したデータベースごとに
// 採番されるものであり、テストはそれぞれ自分のデータベースを所有するためです。
func NewUserRoleBuilder(t *testing.T, db *database.DB) *UserRoleBuilder {
	t.Helper()
	return &UserRoleBuilder{
		t:        t,
		db:       db,
		roleName: model.RoleNameAdmin,
	}
}

// WithUserID sets the user receiving the role.
//
// [Ja] WithUserID はロールを受け取るユーザーを設定します。
func (b *UserRoleBuilder) WithUserID(userID model.UserID) *UserRoleBuilder {
	b.userID = userID
	return b
}

// WithRoleName sets the role to assign, for a test whose subject is a role other
// than the built-in admin one.
//
// [Ja] WithRoleName は割り当てるロールを設定します。組み込みの admin 以外のロールを
// 扱うテストのためのものです。
func (b *UserRoleBuilder) WithRoleName(roleName model.RoleName) *UserRoleBuilder {
	b.roleName = roleName
	return b
}

// Build assigns the role and returns the assignment's database-assigned ID,
// failing the test on error. id and timestamps are left to the database
// defaults. It fails the test when no user has been set, since user_id is NOT
// NULL, and when no role carries the name, since a name nothing carries points
// at a fixture the test never created.
//
// [Ja] Build はロールを割り当て、DB が採番した割当の ID を返します。エラー時はテストを
// 失敗させます。id とタイムスタンプは DB の既定値に任せます。user_id は NOT NULL のため
// ユーザーが未設定の場合はテストを失敗させ、どのロールも持たない名前はテストが作っていない
// フィクスチャを指しているため、その場合もテストを失敗させます。
func (b *UserRoleBuilder) Build() model.UserRoleID {
	b.t.Helper()

	if b.userID == 0 {
		b.t.Fatal("UserRoleBuilder にはユーザー ID が必要です (WithUserID で設定してください)")
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
