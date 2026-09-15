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

// newGrantUserRoleUsecaseはテスト専用のデータベース上にGrantUserRoleUsecaseを
// 組み立てます。UseCaseは自前のトランザクションを開くため、テストはそれがコミットした行を
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

// seedAdminは組み込みのadminロールを持つユーザーを作り、そのidを返します。
// すべてを許された操作者か、守るべき既存の管理者を必要とするテストのためのものです。
// 剥奪のテストも同じ目的でこれを使います。
func seedAdmin(t *testing.T, db *database.DB) model.UserID {
	t.Helper()

	userID := testutil.NewUserBuilder(t, db).Build()
	testutil.NewUserRoleBuilder(t, db).WithUserID(userID).Build()
	return userID
}

// assertAppErrCodeは、errが指定したコードを持つ *model.AppErrorでなければテストを
// 失敗させます。剥奪のテストもこれを使います。
func assertAppErrCode(t *testing.T, err error, want model.AppErrorCode) {
	t.Helper()

	ae := model.AsAppError(err)
	if ae == nil {
		t.Fatalf("エラー = %v、期待値 = *model.AppError (%d)", err, want)
	}
	if ae.Code != want {
		t.Errorf("エラーコード = %d、期待値 = %d", ae.Code, want)
	}
}

// TestGrantUserRoleUsecase_Execute_Successは、管理者がadminロールをそれを持たない
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
		t.Fatalf("Execute()のエラー = %v、期待値 = nil", err)
	}

	if got := countUserRoles(t, db, targetID); got != 1 {
		t.Errorf("付与後のロール割当数 = %d、期待値 = 1", got)
	}
	if output.TargetAtname != "granted" {
		t.Errorf("TargetAtname = %q、期待値 = %q", output.TargetAtname, "granted")
	}
}

// TestGrantUserRoleUsecase_Execute_SucceedsWhenAlreadyHeldは、既に持っている
// ロールの付与が、2つ目の割当を増やさずに成功することを検証します。要求が求めたのは
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
		t.Fatalf("Execute()のエラー = %v、期待値 = nil", err)
	}

	if got := countUserRoles(t, db, actorID); got != 1 {
		t.Errorf("再付与後のロール割当数 = %d、期待値 = 1 (行が増えてはならない)", got)
	}
}

// TestGrantUserRoleUsecase_Execute_Operatorは、groobbのサブコマンドを実行する
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
		t.Fatalf("Execute()のエラー = %v、期待値 = nil", err)
	}

	if got := countUserRoles(t, db, targetID); got != 1 {
		t.Errorf("付与後のロール割当数 = %d、期待値 = 1", got)
	}
}

// TestGrantUserRoleUsecase_Execute_Forbiddenは、ロールを1つも持たない人が
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
		t.Errorf("拒否後のロール割当数 = %d、期待値 = 0", got)
	}
}

// TestGrantUserRoleUsecase_Execute_NotFoundは、どのロールも持たない名前、誰も持たない
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
				t.Errorf("拒否後のロール割当数 = %d、期待値 = 0", got)
			}
		})
	}
}
