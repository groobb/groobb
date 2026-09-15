package usecase_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
)

// newGetAdminUsersUsecaseは、テストが所有するデータベース上にUseCaseを作り、
// テストが読み戻すつもりのアカウントを置けるよう両方を返す。
func newGetAdminUsersUsecase(t *testing.T) (*usecase.GetAdminUsersUsecase, *database.DB, context.Context) {
	t.Helper()

	db := testutil.SetupDB(t)
	uc := usecase.NewGetAdminUsersUsecase(
		repository.NewRoleRepository(db),
		repository.NewUserRepository(db),
	)
	return uc, db, i18n.SetLocale(context.Background(), model.LocaleJa)
}

// newAdministratorは組み込みのadminロールを持つアカウントを作り、その操作者を
// 返す。以下のテストの多くが一覧を読むのはこの人としてである。
func newAdministrator(t *testing.T, db *database.DB) usecase.Actor {
	t.Helper()

	userID := testutil.NewUserBuilder(t, db).WithAtname("administrator").Build()
	testutil.NewUserRoleBuilder(t, db).WithUserID(userID).Build()
	return usecase.UserActor(userID)
}

// listedAtnamesは、一覧の各行のatnameを順序のまま返す。
func listedAtnames(users []usecase.AdminUser) []string {
	atnames := make([]string, len(users))
	for i, row := range users {
		atnames[i] = row.User.Atname
	}
	return atnames
}

func TestGetAdminUsersUsecase_Execute_Permission(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		// seedは操作者のロールをデータベースへ置き、その操作者を返す。
		seed          func(t *testing.T, db *database.DB) usecase.Actor
		wantForbidden bool
	}{
		{
			name: "adminロールを持つ利用者は一覧を読める",
			seed: func(t *testing.T, db *database.DB) usecase.Actor {
				t.Helper()
				return newAdministrator(t, db)
			},
		},
		{
			name: "利用者を読むスコープだけを持つ利用者も一覧を読める",
			seed: func(t *testing.T, db *database.DB) usecase.Actor {
				t.Helper()
				userID := testutil.NewUserBuilder(t, db).Build()
				testutil.NewRoleBuilder(t, db).
					WithName(model.RoleName("user_reader")).
					WithScopes([]model.Scope{model.ScopeUserRead}).
					Build()
				testutil.NewUserRoleBuilder(t, db).
					WithUserID(userID).
					WithRoleName(model.RoleName("user_reader")).
					Build()
				return usecase.UserActor(userID)
			},
		},
		{
			name: "運用者は一覧を読める",
			seed: func(t *testing.T, db *database.DB) usecase.Actor {
				t.Helper()
				return usecase.OperatorActor()
			},
		},
		{
			// このロールは管理画面を許すスコープを持つが、この画面が求めるスコープは
			// 持たない。ハブを許されることは一覧を許されることではない。
			name: "ロールを書き換えるスコープだけでは一覧を読めない",
			seed: func(t *testing.T, db *database.DB) usecase.Actor {
				t.Helper()
				userID := testutil.NewUserBuilder(t, db).Build()
				testutil.NewRoleBuilder(t, db).
					WithName(model.RoleName("role_writer")).
					WithScopes([]model.Scope{model.ScopeUserRoleWrite}).
					Build()
				testutil.NewUserRoleBuilder(t, db).
					WithUserID(userID).
					WithRoleName(model.RoleName("role_writer")).
					Build()
				return usecase.UserActor(userID)
			},
			wantForbidden: true,
		},
		{
			name: "ロールを持たない利用者は拒まれる",
			seed: func(t *testing.T, db *database.DB) usecase.Actor {
				t.Helper()
				return usecase.UserActor(testutil.NewUserBuilder(t, db).Build())
			},
			wantForbidden: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			uc, db, ctx := newGetAdminUsersUsecase(t)

			output, err := uc.Execute(ctx, usecase.GetAdminUsersInput{
				Actor: tt.seed(t, db),
				Page:  1,
			})

			if !tt.wantForbidden {
				if err != nil {
					t.Fatalf("Execute()のエラー = %v、期待値 = nil", err)
				}
				if output == nil {
					t.Fatal("Execute()のoutput = nil、期待値 = 一覧")
				}
				return
			}
			if output != nil {
				t.Errorf("Execute()のoutput = %v、期待値 = nil", output)
			}
			assertAppErrCode(t, err, model.AppErrCodeForbidden)
		})
	}
}

func TestGetAdminUsersUsecase_Execute(t *testing.T) {
	t.Parallel()

	t.Run("退会していない利用者を登録の新しい順に返す", func(t *testing.T) {
		t.Parallel()

		uc, db, ctx := newGetAdminUsersUsecase(t)
		actor := newAdministrator(t, db)
		testutil.NewUserBuilder(t, db).WithAtname("earlier").Build()
		testutil.NewUserBuilder(t, db).WithAtname("later").Build()
		testutil.NewUserBuilder(t, db).WithAtname("left").WithDeletedAt(time.Now()).Build()

		output, err := uc.Execute(ctx, usecase.GetAdminUsersInput{Actor: actor, Page: 1})
		if err != nil {
			t.Fatalf("Execute()のエラー = %v", err)
		}

		want := []string{"later", "earlier", "administrator"}
		if got := listedAtnames(output.Users); !equalStrings(got, want) {
			t.Errorf("一覧のatname = %v、期待値 = %v", got, want)
		}
		if output.TotalCount != 3 {
			t.Errorf("TotalCount = %d、期待値 = 3", output.TotalCount)
		}
	})

	t.Run("行ごとにその利用者のロールを添える", func(t *testing.T) {
		t.Parallel()

		uc, db, ctx := newGetAdminUsersUsecase(t)
		actor := newAdministrator(t, db)
		testutil.NewUserBuilder(t, db).WithAtname("plain").Build()

		output, err := uc.Execute(ctx, usecase.GetAdminUsersInput{Actor: actor, Page: 1})
		if err != nil {
			t.Fatalf("Execute()のエラー = %v", err)
		}
		if len(output.Users) != 2 {
			t.Fatalf("len(Users) = %d、期待値 = 2", len(output.Users))
		}

		for _, row := range output.Users {
			switch row.User.Atname {
			case "administrator":
				if len(row.Roles) != 1 || row.Roles[0].Name != model.RoleNameAdmin {
					t.Errorf("管理者のRoles = %v、期待値 = [%q]", row.Roles, model.RoleNameAdmin)
				}
			case "plain":
				if len(row.Roles) != 0 {
					t.Errorf("ロールを持たない利用者のRoles = %v、期待値 = 空", row.Roles)
				}
			}
		}
	})

	t.Run("atnameの前方一致で絞り込み、大文字小文字を区別しない", func(t *testing.T) {
		t.Parallel()

		uc, db, ctx := newGetAdminUsersUsecase(t)
		actor := newAdministrator(t, db)
		testutil.NewUserBuilder(t, db).WithAtname("Alice").Build()
		testutil.NewUserBuilder(t, db).WithAtname("alberta").Build()
		testutil.NewUserBuilder(t, db).WithAtname("bob").Build()

		output, err := uc.Execute(ctx, usecase.GetAdminUsersInput{
			Actor:        actor,
			AtnamePrefix: "AL",
			Page:         1,
		})
		if err != nil {
			t.Fatalf("Execute()のエラー = %v", err)
		}

		want := []string{"alberta", "Alice"}
		if got := listedAtnames(output.Users); !equalStrings(got, want) {
			t.Errorf("一覧のatname = %v、期待値 = %v", got, want)
		}
		if output.TotalCount != 2 {
			t.Errorf("TotalCount = %d、期待値 = 2", output.TotalCount)
		}
	})

	// どのアカウントも持てない値は何にも一致しないため、一覧はエラーではなく空になる。
	// 届いたのは誰も見つからなかった検索であって、画面が答えられない要求ではない。
	t.Run("atnameの文字集合の外にある絞り込みは空の一覧になる", func(t *testing.T) {
		t.Parallel()

		uc, db, ctx := newGetAdminUsersUsecase(t)
		actor := newAdministrator(t, db)

		for _, prefix := range []string{"a-b", "アリス", strings.Repeat("a", 100)} {
			output, err := uc.Execute(ctx, usecase.GetAdminUsersInput{
				Actor:        actor,
				AtnamePrefix: prefix,
				Page:         1,
			})
			if err != nil {
				t.Fatalf("Execute(prefix=%q)のエラー = %v", prefix, err)
			}
			if len(output.Users) != 0 {
				t.Errorf("Execute(prefix=%q) の一覧 = %v、期待値 = 空", prefix, listedAtnames(output.Users))
			}
			if output.TotalCount != 0 {
				t.Errorf("Execute(prefix=%q) のTotalCount = %d、期待値 = 0", prefix, output.TotalCount)
			}
		}
	})

	t.Run("1ページ分ずつ返し、最後のページまで送れる", func(t *testing.T) {
		t.Parallel()

		uc, db, ctx := newGetAdminUsersUsecase(t)
		actor := newAdministrator(t, db)
		for i := range model.AdminUsersPerPage {
			testutil.NewUserBuilder(t, db).WithAtname(fmt.Sprintf("member%02d", i)).Build()
		}

		first, err := uc.Execute(ctx, usecase.GetAdminUsersInput{Actor: actor, Page: 1})
		if err != nil {
			t.Fatalf("Execute(page=1)のエラー = %v", err)
		}
		if len(first.Users) != model.AdminUsersPerPage {
			t.Errorf("1ページ目の件数 = %d、期待値 = %d", len(first.Users), model.AdminUsersPerPage)
		}
		if first.TotalCount != model.AdminUsersPerPage+1 {
			t.Errorf("TotalCount = %d、期待値 = %d", first.TotalCount, model.AdminUsersPerPage+1)
		}

		second, err := uc.Execute(ctx, usecase.GetAdminUsersInput{Actor: actor, Page: 2})
		if err != nil {
			t.Fatalf("Execute(page=2)のエラー = %v", err)
		}
		want := []string{"administrator"}
		if got := listedAtnames(second.Users); !equalStrings(got, want) {
			t.Errorf("2ページ目のatname = %v、期待値 = %v", got, want)
		}
	})

	t.Run("最後のページを越えた番号は件数を保ったまま空の一覧になる", func(t *testing.T) {
		t.Parallel()

		uc, db, ctx := newGetAdminUsersUsecase(t)
		actor := newAdministrator(t, db)

		output, err := uc.Execute(ctx, usecase.GetAdminUsersInput{Actor: actor, Page: 2})
		if err != nil {
			t.Fatalf("Execute()のエラー = %v", err)
		}
		if len(output.Users) != 0 {
			t.Errorf("一覧 = %v、期待値 = 空", listedAtnames(output.Users))
		}
		if output.TotalCount != 1 {
			t.Errorf("TotalCount = %d、期待値 = 1", output.TotalCount)
		}
	})

	// ページ番号はアドレスの一部であり、一覧が決して振らない番号はどのページも
	// 名指していない。
	t.Run("最初のページより前の番号は不存在として拒む", func(t *testing.T) {
		t.Parallel()

		uc, db, ctx := newGetAdminUsersUsecase(t)
		actor := newAdministrator(t, db)

		for _, page := range []int{0, -1} {
			output, err := uc.Execute(ctx, usecase.GetAdminUsersInput{Actor: actor, Page: page})
			if output != nil {
				t.Errorf("Execute(page=%d)のoutput = %v、期待値 = nil", page, output)
			}
			assertAppErrCode(t, err, model.AppErrCodeResourceNotFound)
		}
	})

	// 権限はページ番号より先に答えるため、一覧を読めない操作者は、そのどのページが
	// 存在するかについて何も知らない。
	t.Run("権限が無ければページ番号より先に拒む", func(t *testing.T) {
		t.Parallel()

		uc, db, ctx := newGetAdminUsersUsecase(t)
		actor := usecase.UserActor(testutil.NewUserBuilder(t, db).Build())

		_, err := uc.Execute(ctx, usecase.GetAdminUsersInput{Actor: actor, Page: 0})

		assertAppErrCode(t, err, model.AppErrCodeForbidden)
	})
}

// equalStringsは、2つのスライスが同じ文字列を同じ順序で持つかどうかを返す。
func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
