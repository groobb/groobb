package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
)

// assignRole assigns a role through the repository, failing the test on error,
// for the tests whose subject is something other than the assignment itself.
//
// [Ja] assignRole はリポジトリ経由でロールを割り当て、エラー時はテストを失敗させる。
// 割当そのものを主題としないテストのためのものである。
func (r *roleRepos) assignRole(t *testing.T, ctx context.Context, userID model.UserID, roleID model.RoleID) *model.UserRole {
	t.Helper()

	userRole, err := r.userRole.Create(ctx, repository.CreateUserRoleInput{UserID: userID, RoleID: roleID})
	if err != nil {
		t.Fatalf("テスト用ロール割当の作成に失敗: %v", err)
	}

	return userRole
}

// roleNamesOf returns the names of the roles the user holds, so an assertion
// about what a write left behind reads as the list a caller would see.
//
// [Ja] roleNamesOf はユーザーが持つロールの名前を返す。書き込みが何を残したかの検証を、
// 呼び出し側が目にする一覧として読めるようにするためである。
func (r *roleRepos) roleNamesOf(t *testing.T, ctx context.Context, userID model.UserID) []model.RoleName {
	t.Helper()

	roles, err := r.role.ListByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("ListByUserID() error = %v", err)
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
		t.Error("Create() userRole.ID は DB 採番で空でないはず")
	}
	if userRole.UserID != userID {
		t.Errorf("userRole.UserID = %v, want %v", userRole.UserID, userID)
	}
	if userRole.RoleID != admin.ID {
		t.Errorf("userRole.RoleID = %v, want %v", userRole.RoleID, admin.ID)
	}
	if userRole.CreatedAt.IsZero() {
		t.Error("userRole.CreatedAt は DB 既定値で設定されるはず")
	}
	if userRole.UpdatedAt.IsZero() {
		t.Error("userRole.UpdatedAt は DB 既定値で設定されるはず")
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
		t.Fatal("Create() error = nil, want a unique violation")
	}
	if !repository.IsUniqueViolation(err) {
		t.Errorf("Create() error = %v, want a unique violation", err)
	}
}

func TestUserRoleRepository_ListByUserIDs(t *testing.T) {
	t.Parallel()

	t.Run("渡した利用者の割当だけを利用者ごと・ロール id 順に返す", func(t *testing.T) {
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

		// The rows are written in neither the expected order nor its reverse, so
		// a result that merely echoes the insertion order cannot pass.
		//
		// [Ja] 行は期待する並びともその逆とも異なる順で書いてあり、挿入順をそのまま返す
		// 結果では通らないようにしている。
		repos.assignRole(t, ctx, second, admin.ID)
		repos.assignRole(t, ctx, first, moderatorID)
		repos.assignRole(t, ctx, listed, admin.ID)
		repos.assignRole(t, ctx, first, admin.ID)

		userRoles, err := repos.userRole.ListByUserIDs(ctx, []model.UserID{first, second})
		if err != nil {
			t.Fatalf("ListByUserIDs() error = %v", err)
		}

		want := []model.UserRole{
			{UserID: first, RoleID: admin.ID},
			{UserID: first, RoleID: moderatorID},
			{UserID: second, RoleID: admin.ID},
		}
		if len(userRoles) != len(want) {
			t.Fatalf("len(ListByUserIDs()) = %d, want %d", len(userRoles), len(want))
		}
		for i, w := range want {
			if userRoles[i].UserID != w.UserID || userRoles[i].RoleID != w.RoleID {
				t.Errorf("ListByUserIDs()[%d] = (user %v, role %v), want (user %v, role %v)",
					i, userRoles[i].UserID, userRoles[i].RoleID, w.UserID, w.RoleID)
			}
		}
	})

	t.Run("id を 1 つも渡さなければクエリせず空を返す", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newRoleRepos(t)
		admin := repos.findRole(t, ctx, model.RoleNameAdmin)
		userID := testutil.NewUserBuilder(t, repos.db).Build()
		repos.assignRole(t, ctx, userID, admin.ID)

		userRoles, err := repos.userRole.ListByUserIDs(ctx, nil)
		if err != nil {
			t.Fatalf("ListByUserIDs() error = %v", err)
		}
		if len(userRoles) != 0 {
			t.Errorf("len(ListByUserIDs()) = %d, want 0", len(userRoles))
		}
	})

	t.Run("ロールを持たない利用者だけなら空を返す", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newRoleRepos(t)
		userID := testutil.NewUserBuilder(t, repos.db).Build()

		userRoles, err := repos.userRole.ListByUserIDs(ctx, []model.UserID{userID})
		if err != nil {
			t.Fatalf("ListByUserIDs() error = %v", err)
		}
		if len(userRoles) != 0 {
			t.Errorf("len(ListByUserIDs()) = %d, want 0", len(userRoles))
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
			t.Fatalf("CountHoldersByRoleID() error = %v", err)
		}
		if count != 2 {
			t.Errorf("CountHoldersByRoleID() = %d, want 2 (別のロールの保持者は数えない)", count)
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
			t.Fatalf("CountHoldersByRoleID() error = %v", err)
		}
		if count != 1 {
			t.Errorf("CountHoldersByRoleID() = %d, want 1 (退会済みの保持者は数えない)", count)
		}
	})

	t.Run("誰も持たないロールは 0 を返す", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newRoleRepos(t)
		admin := repos.findRole(t, ctx, model.RoleNameAdmin)

		count, err := repos.userRole.CountHoldersByRoleID(ctx, admin.ID)
		if err != nil {
			t.Fatalf("CountHoldersByRoleID() error = %v", err)
		}
		if count != 0 {
			t.Errorf("CountHoldersByRoleID() = %d, want 0", count)
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
			t.Fatalf("DeleteByUserIDAndRoleID() error = %v", err)
		}

		names := repos.roleNamesOf(t, ctx, userID)
		if len(names) != 1 || names[0] != model.RoleName("moderator") {
			t.Errorf("剥奪後に持つロール = %v, want [moderator]", names)
		}
		if got := repos.roleNamesOf(t, ctx, other); len(got) != 1 {
			t.Errorf("他の利用者が持つロール = %v, want [admin]", got)
		}
	})

	t.Run("持たないロールの剥奪はエラーにならない", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newRoleRepos(t)
		admin := repos.findRole(t, ctx, model.RoleNameAdmin)
		userID := testutil.NewUserBuilder(t, repos.db).Build()

		if err := repos.userRole.DeleteByUserIDAndRoleID(ctx, userID, admin.ID); err != nil {
			t.Fatalf("DeleteByUserIDAndRoleID() error = %v", err)
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
			t.Fatalf("DeleteByUserID() error = %v", err)
		}

		if names := repos.roleNamesOf(t, ctx, userID); len(names) != 0 {
			t.Errorf("退会後に持つロール = %v, want 空", names)
		}
		if names := repos.roleNamesOf(t, ctx, other); len(names) != 1 {
			t.Errorf("他の利用者が持つロール = %v, want [admin]", names)
		}
	})

	t.Run("ロールを持たない利用者の削除はエラーにならない", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newRoleRepos(t)
		userID := testutil.NewUserBuilder(t, repos.db).Build()

		if err := repos.userRole.DeleteByUserID(ctx, userID); err != nil {
			t.Fatalf("DeleteByUserID() error = %v", err)
		}
	})
}
