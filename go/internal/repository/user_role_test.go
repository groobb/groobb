package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
)

// assignRoleはリポジトリ経由でロールを割り当て、エラー時はテストを失敗させる。
// 割当そのものを主題としないテストのためのものである。
func (r *roleRepos) assignRole(t *testing.T, ctx context.Context, userID model.UserID, roleID model.RoleID) *model.UserRole {
	t.Helper()

	userRole, err := r.userRole.Create(ctx, repository.CreateUserRoleInput{UserID: userID, RoleID: roleID})
	if err != nil {
		t.Fatalf("テスト用ロール割当の作成に失敗: %v", err)
	}

	return userRole
}

// roleNamesOfはユーザーが持つロールの名前を返す。書き込みが何を残したかの検証を、
// 呼び出し側が目にする一覧として読めるようにするためである。
func (r *roleRepos) roleNamesOf(t *testing.T, ctx context.Context, userID model.UserID) []model.RoleName {
	t.Helper()

	roles, err := r.role.ListByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("ListByUserID()のエラー = %v", err)
	}

	names := make([]model.RoleName, len(roles))
	for i, role := range roles {
		names[i] = role.Name
	}
	return names
}

func TestUserRoleRepository_Create(t *testing.T) {
	t.Parallel()

	repos, ctx := newRoleRepos(t)
	admin := repos.findRole(t, ctx, model.RoleNameAdmin)
	userID := testutil.NewUserBuilder(t, repos.db).Build()

	userRole := repos.assignRole(t, ctx, userID, admin.ID)

	if userRole.ID == 0 {
		t.Error("Create() userRole.IDはDB採番で空でないはず")
	}
	if userRole.UserID != userID {
		t.Errorf("userRole.UserID = %v、期待値 = %v", userRole.UserID, userID)
	}
	if userRole.RoleID != admin.ID {
		t.Errorf("userRole.RoleID = %v、期待値 = %v", userRole.RoleID, admin.ID)
	}
	if userRole.CreatedAt.IsZero() {
		t.Error("userRole.CreatedAtはDB既定値で設定されるはず")
	}
	if userRole.UpdatedAt.IsZero() {
		t.Error("userRole.UpdatedAtはDB既定値で設定されるはず")
	}
}

func TestUserRoleRepository_Create_RejectsTheSameAssignmentTwice(t *testing.T) {
	t.Parallel()

	repos, ctx := newRoleRepos(t)
	admin := repos.findRole(t, ctx, model.RoleNameAdmin)
	userID := testutil.NewUserBuilder(t, repos.db).Build()

	repos.assignRole(t, ctx, userID, admin.ID)

	_, err := repos.userRole.Create(ctx, repository.CreateUserRoleInput{UserID: userID, RoleID: admin.ID})
	if err == nil {
		t.Fatal("Create()のエラー = nil、期待値は一意制約違反")
	}
	if !repository.IsUniqueViolation(err) {
		t.Errorf("Create()のエラー = %v、期待値は一意制約違反", err)
	}
}

func TestUserRoleRepository_ListByUserIDs(t *testing.T) {
	t.Parallel()

	t.Run("渡した利用者の割当だけを利用者ごと・ロールid順に返す", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newRoleRepos(t)
		admin := repos.findRole(t, ctx, model.RoleNameAdmin)
		moderatorID := testutil.NewRoleBuilder(t, repos.db).
			WithName("moderator").
			WithScopes([]model.Scope{model.ScopeUserRead}).
			Build()

		first := testutil.NewUserBuilder(t, repos.db).Build()
		second := testutil.NewUserBuilder(t, repos.db).Build()
		listed := testutil.NewUserBuilder(t, repos.db).Build()

		// 行は期待する並びともその逆とも異なる順で書いてあり、挿入順をそのまま返す
		// 結果では通らないようにしている。
		repos.assignRole(t, ctx, second, admin.ID)
		repos.assignRole(t, ctx, first, moderatorID)
		repos.assignRole(t, ctx, listed, admin.ID)
		repos.assignRole(t, ctx, first, admin.ID)

		userRoles, err := repos.userRole.ListByUserIDs(ctx, []model.UserID{first, second})
		if err != nil {
			t.Fatalf("ListByUserIDs()のエラー = %v", err)
		}

		want := []model.UserRole{
			{UserID: first, RoleID: admin.ID},
			{UserID: first, RoleID: moderatorID},
			{UserID: second, RoleID: admin.ID},
		}
		if len(userRoles) != len(want) {
			t.Fatalf("len(ListByUserIDs()) = %d、期待値 = %d", len(userRoles), len(want))
		}
		for i, w := range want {
			if userRoles[i].UserID != w.UserID || userRoles[i].RoleID != w.RoleID {
				t.Errorf("ListByUserIDs()[%d] = (ユーザー %v、ロール %v)、期待値 = (ユーザー %v、ロール %v)",
					i, userRoles[i].UserID, userRoles[i].RoleID, w.UserID, w.RoleID)
			}
		}
	})

	t.Run("idを1つも渡さなければクエリせず空を返す", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newRoleRepos(t)
		admin := repos.findRole(t, ctx, model.RoleNameAdmin)
		userID := testutil.NewUserBuilder(t, repos.db).Build()
		repos.assignRole(t, ctx, userID, admin.ID)

		userRoles, err := repos.userRole.ListByUserIDs(ctx, nil)
		if err != nil {
			t.Fatalf("ListByUserIDs()のエラー = %v", err)
		}
		if len(userRoles) != 0 {
			t.Errorf("len(ListByUserIDs()) = %d、期待値 = 0", len(userRoles))
		}
	})

	t.Run("ロールを持たない利用者だけなら空を返す", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newRoleRepos(t)
		userID := testutil.NewUserBuilder(t, repos.db).Build()

		userRoles, err := repos.userRole.ListByUserIDs(ctx, []model.UserID{userID})
		if err != nil {
			t.Fatalf("ListByUserIDs()のエラー = %v", err)
		}
		if len(userRoles) != 0 {
			t.Errorf("len(ListByUserIDs()) = %d、期待値 = 0", len(userRoles))
		}
	})
}

func TestUserRoleRepository_CountHoldersByRoleID(t *testing.T) {
	t.Parallel()

	t.Run("そのロールの保持者を数える", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newRoleRepos(t)
		admin := repos.findRole(t, ctx, model.RoleNameAdmin)
		moderatorID := testutil.NewRoleBuilder(t, repos.db).
			WithName("moderator").
			WithScopes([]model.Scope{model.ScopeUserRead}).
			Build()

		first := testutil.NewUserBuilder(t, repos.db).Build()
		second := testutil.NewUserBuilder(t, repos.db).Build()
		moderator := testutil.NewUserBuilder(t, repos.db).Build()
		repos.assignRole(t, ctx, first, admin.ID)
		repos.assignRole(t, ctx, second, admin.ID)
		repos.assignRole(t, ctx, moderator, moderatorID)

		count, err := repos.userRole.CountHoldersByRoleID(ctx, admin.ID)
		if err != nil {
			t.Fatalf("CountHoldersByRoleID()のエラー = %v", err)
		}
		if count != 2 {
			t.Errorf("CountHoldersByRoleID() = %d、期待値 = 2 (別のロールの保持者は数えない)", count)
		}
	})

	t.Run("退会した保持者は数えない", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newRoleRepos(t)
		admin := repos.findRole(t, ctx, model.RoleNameAdmin)

		active := testutil.NewUserBuilder(t, repos.db).Build()
		withdrawn := testutil.NewUserBuilder(t, repos.db).WithDeletedAt(time.Now()).Build()
		repos.assignRole(t, ctx, active, admin.ID)
		repos.assignRole(t, ctx, withdrawn, admin.ID)

		count, err := repos.userRole.CountHoldersByRoleID(ctx, admin.ID)
		if err != nil {
			t.Fatalf("CountHoldersByRoleID()のエラー = %v", err)
		}
		if count != 1 {
			t.Errorf("CountHoldersByRoleID() = %d、期待値 = 1 (退会済みの保持者は数えない)", count)
		}
	})

	// 停止は、それが続く間その管理者を数から外す。管理者を1人以上保つ保護が、
	// 管理者として行動するためにサインインできない人を数えないようにするためである。
	t.Run("停止中の保持者は数えない", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newRoleRepos(t)
		admin := repos.findRole(t, ctx, model.RoleNameAdmin)

		active := testutil.NewUserBuilder(t, repos.db).Build()
		suspended := testutil.NewUserBuilder(t, repos.db).WithSuspendedAt(time.Now()).Build()
		repos.assignRole(t, ctx, active, admin.ID)
		repos.assignRole(t, ctx, suspended, admin.ID)

		count, err := repos.userRole.CountHoldersByRoleID(ctx, admin.ID)
		if err != nil {
			t.Fatalf("CountHoldersByRoleID()のエラー = %v", err)
		}
		if count != 1 {
			t.Errorf("CountHoldersByRoleID() = %d、期待値 = 1 (停止中の保持者は数えない)", count)
		}
	})

	t.Run("誰も持たないロールは0を返す", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newRoleRepos(t)
		admin := repos.findRole(t, ctx, model.RoleNameAdmin)

		count, err := repos.userRole.CountHoldersByRoleID(ctx, admin.ID)
		if err != nil {
			t.Fatalf("CountHoldersByRoleID()のエラー = %v", err)
		}
		if count != 0 {
			t.Errorf("CountHoldersByRoleID() = %d、期待値 = 0", count)
		}
	})
}

func TestUserRoleRepository_DeleteByUserIDAndRoleID(t *testing.T) {
	t.Parallel()

	t.Run("そのロールの割当だけを削除する", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newRoleRepos(t)
		admin := repos.findRole(t, ctx, model.RoleNameAdmin)
		moderatorID := testutil.NewRoleBuilder(t, repos.db).
			WithName("moderator").
			WithScopes([]model.Scope{model.ScopeUserRead}).
			Build()

		userID := testutil.NewUserBuilder(t, repos.db).Build()
		other := testutil.NewUserBuilder(t, repos.db).Build()
		repos.assignRole(t, ctx, userID, admin.ID)
		repos.assignRole(t, ctx, userID, moderatorID)
		repos.assignRole(t, ctx, other, admin.ID)

		if err := repos.userRole.DeleteByUserIDAndRoleID(ctx, userID, admin.ID); err != nil {
			t.Fatalf("DeleteByUserIDAndRoleID()のエラー = %v", err)
		}

		names := repos.roleNamesOf(t, ctx, userID)
		if len(names) != 1 || names[0] != model.RoleName("moderator") {
			t.Errorf("剥奪後に持つロール = %v、期待値 = [moderator]", names)
		}
		if got := repos.roleNamesOf(t, ctx, other); len(got) != 1 {
			t.Errorf("他の利用者が持つロール = %v、期待値 = [admin]", got)
		}
	})

	t.Run("持たないロールの剥奪はエラーにならない", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newRoleRepos(t)
		admin := repos.findRole(t, ctx, model.RoleNameAdmin)
		userID := testutil.NewUserBuilder(t, repos.db).Build()

		if err := repos.userRole.DeleteByUserIDAndRoleID(ctx, userID, admin.ID); err != nil {
			t.Fatalf("DeleteByUserIDAndRoleID()のエラー = %v", err)
		}
	})
}

func TestUserRoleRepository_DeleteByUserID(t *testing.T) {
	t.Parallel()

	t.Run("その利用者の割当をすべて削除する", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newRoleRepos(t)
		admin := repos.findRole(t, ctx, model.RoleNameAdmin)
		moderatorID := testutil.NewRoleBuilder(t, repos.db).
			WithName("moderator").
			WithScopes([]model.Scope{model.ScopeUserRead}).
			Build()

		userID := testutil.NewUserBuilder(t, repos.db).Build()
		other := testutil.NewUserBuilder(t, repos.db).Build()
		repos.assignRole(t, ctx, userID, admin.ID)
		repos.assignRole(t, ctx, userID, moderatorID)
		repos.assignRole(t, ctx, other, admin.ID)

		if err := repos.userRole.DeleteByUserID(ctx, userID); err != nil {
			t.Fatalf("DeleteByUserID()のエラー = %v", err)
		}

		if names := repos.roleNamesOf(t, ctx, userID); len(names) != 0 {
			t.Errorf("退会後に持つロール = %v、期待値は空", names)
		}
		if names := repos.roleNamesOf(t, ctx, other); len(names) != 1 {
			t.Errorf("他の利用者が持つロール = %v、期待値 = [admin]", names)
		}
	})

	t.Run("ロールを持たない利用者の削除はエラーにならない", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newRoleRepos(t)
		userID := testutil.NewUserBuilder(t, repos.db).Build()

		if err := repos.userRole.DeleteByUserID(ctx, userID); err != nil {
			t.Fatalf("DeleteByUserID()のエラー = %v", err)
		}
	})
}
