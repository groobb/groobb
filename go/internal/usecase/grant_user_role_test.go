package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
)

// newGrantUserRoleUsecase wires a GrantUserRoleUsecase over the test's own
// database. The UseCase opens its own transaction, so a test asserts against the
// rows it commits.
//
// [Ja] newGrantUserRoleUsecase はテスト専用のデータベース上に GrantUserRoleUsecase を
// 組み立てます。UseCase は自前のトランザクションを開くため、テストはそれがコミットした行を
// 検証します。
func newGrantUserRoleUsecase(t *testing.T, db *database.DB) *usecase.GrantUserRoleUsecase {
	t.Helper()

	return usecase.NewGrantUserRoleUsecase(
		db.Writer,
		repository.NewRoleRepository(db),
		repository.NewUserRepository(db),
		repository.NewUserRoleRepository(db),
	)
}

// seedAdmin creates a user holding the built-in admin role and returns its id,
// for a test that needs either an actor admitted to everything or an existing
// administrator to protect. The revoke tests use it for the same purposes.
//
// [Ja] seedAdmin は組み込みの admin ロールを持つユーザーを作り、その id を返します。
// すべてを許された操作者か、守るべき既存の管理者を必要とするテストのためのものです。
// 剥奪のテストも同じ目的でこれを使います。
func seedAdmin(t *testing.T, db *database.DB) model.UserID {
	t.Helper()

	userID := testutil.NewUserBuilder(t, db).Build()
	testutil.NewUserRoleBuilder(t, db).WithUserID(userID).Build()
	return userID
}

// assertAppErrCode fails the test unless err is an *model.AppError carrying the
// given code. The revoke tests use it as well.
//
// [Ja] assertAppErrCode は、err が指定したコードを持つ *model.AppError でなければテストを
// 失敗させます。剥奪のテストもこれを使います。
func assertAppErrCode(t *testing.T, err error, want model.AppErrorCode) {
	t.Helper()

	ae := model.AsAppError(err)
	if ae == nil {
		t.Fatalf("error = %v, want *model.AppError (%d)", err, want)
	}
	if ae.Code != want {
		t.Errorf("error code = %d, want %d", ae.Code, want)
	}
}

// TestGrantUserRoleUsecase_Execute_Success verifies that an administrator gives
// the admin role to someone who does not hold it, and that the assignment is
// committed.
//
// [Ja] TestGrantUserRoleUsecase_Execute_Success は、管理者が admin ロールをそれを持たない
// 人に与え、その割当がコミットされることを検証します。
func TestGrantUserRoleUsecase_Execute_Success(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	uc := newGrantUserRoleUsecase(t, db)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

	actorID := seedAdmin(t, db)
	targetID := testutil.NewUserBuilder(t, db).WithAtname("granted").Build()

	output, err := uc.Execute(ctx, usecase.GrantUserRoleInput{
		Actor:        usecase.UserActor(actorID),
		TargetUserID: targetID,
		RoleName:     model.RoleNameAdmin,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	if got := countUserRoles(t, db, targetID); got != 1 {
		t.Errorf("付与後のロール割当数 = %d, want 1", got)
	}
	if output.TargetAtname != "granted" {
		t.Errorf("TargetAtname = %q, want %q", output.TargetAtname, "granted")
	}
}

// TestGrantUserRoleUsecase_Execute_SucceedsWhenAlreadyHeld verifies that granting
// a role the person already holds succeeds without adding a second assignment.
// What the request asked for is that they hold the role, and they do.
//
// [Ja] TestGrantUserRoleUsecase_Execute_SucceedsWhenAlreadyHeld は、既に持っている
// ロールの付与が、2 つ目の割当を増やさずに成功することを検証します。要求が求めたのは
// その人がロールを持っていることであり、実際に持っているためです。
func TestGrantUserRoleUsecase_Execute_SucceedsWhenAlreadyHeld(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	uc := newGrantUserRoleUsecase(t, db)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

	actorID := seedAdmin(t, db)

	if _, err := uc.Execute(ctx, usecase.GrantUserRoleInput{
		Actor:        usecase.UserActor(actorID),
		TargetUserID: actorID,
		RoleName:     model.RoleNameAdmin,
	}); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	if got := countUserRoles(t, db, actorID); got != 1 {
		t.Errorf("再付与後のロール割当数 = %d, want 1 (行が増えてはならない)", got)
	}
}

// TestGrantUserRoleUsecase_Execute_Operator verifies that the operator running a
// groobb subcommand appoints an administrator on an instance where nobody is one
// yet. This is how the first administrator comes to be.
//
// [Ja] TestGrantUserRoleUsecase_Execute_Operator は、groobb のサブコマンドを実行する
// 運用者が、まだ誰も管理者でないインスタンスで管理者を立てられることを検証します。
// 最初の管理者はこうして生まれます。
func TestGrantUserRoleUsecase_Execute_Operator(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	uc := newGrantUserRoleUsecase(t, db)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

	targetID := testutil.NewUserBuilder(t, db).Build()

	if _, err := uc.Execute(ctx, usecase.GrantUserRoleInput{
		Actor:        usecase.OperatorActor(),
		TargetUserID: targetID,
		RoleName:     model.RoleNameAdmin,
	}); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	if got := countUserRoles(t, db, targetID); got != 1 {
		t.Errorf("付与後のロール割当数 = %d, want 1", got)
	}
}

// TestGrantUserRoleUsecase_Execute_Forbidden verifies that someone holding no
// role cannot hand one out, and that nothing is written.
//
// [Ja] TestGrantUserRoleUsecase_Execute_Forbidden は、ロールを 1 つも持たない人が
// ロールを配れないこと、そして何も書き込まれないことを検証します。
func TestGrantUserRoleUsecase_Execute_Forbidden(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	uc := newGrantUserRoleUsecase(t, db)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

	actorID := testutil.NewUserBuilder(t, db).Build()
	targetID := testutil.NewUserBuilder(t, db).Build()

	_, err := uc.Execute(ctx, usecase.GrantUserRoleInput{
		Actor:        usecase.UserActor(actorID),
		TargetUserID: targetID,
		RoleName:     model.RoleNameAdmin,
	})
	assertAppErrCode(t, err, model.AppErrCodeForbidden)

	if got := countUserRoles(t, db, targetID); got != 0 {
		t.Errorf("拒否後のロール割当数 = %d, want 0", got)
	}
}

// TestGrantUserRoleUsecase_Execute_NotFound verifies that a role name nothing
// carries, an id nobody has, and an account that has withdrawn are all answered
// the same way: there is nothing at the address the request named.
//
// [Ja] TestGrantUserRoleUsecase_Execute_NotFound は、どのロールも持たない名前、誰も持たない
// id、そして退会したアカウントのいずれもが同じ形で答えられることを検証します。要求が
// 名指したアドレスには何も無い、というものです。
func TestGrantUserRoleUsecase_Execute_NotFound(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	uc := newGrantUserRoleUsecase(t, db)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

	actorID := seedAdmin(t, db)
	presentID := testutil.NewUserBuilder(t, db).Build()
	withdrawnID := testutil.NewUserBuilder(t, db).WithDeletedAt(time.Now()).Build()

	tests := []struct {
		name     string
		targetID model.UserID
		roleName model.RoleName
	}{
		{name: "存在しないロール名", targetID: presentID, roleName: "moderator"},
		{name: "存在しない利用者", targetID: model.UserID(999999), roleName: model.RoleNameAdmin},
		{name: "退会した利用者", targetID: withdrawnID, roleName: model.RoleNameAdmin},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := uc.Execute(ctx, usecase.GrantUserRoleInput{
				Actor:        usecase.UserActor(actorID),
				TargetUserID: tt.targetID,
				RoleName:     tt.roleName,
			})
			assertAppErrCode(t, err, model.AppErrCodeResourceNotFound)

			if got := countUserRoles(t, db, tt.targetID); got != 0 {
				t.Errorf("拒否後のロール割当数 = %d, want 0", got)
			}
		})
	}
}
