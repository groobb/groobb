package usecase_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
)

// newRevokeUserRoleUsecaseは、渡したデータベース上にRevokeUserRoleUsecaseを
// 組み立てます。UseCaseは自前のトランザクションを開くため、テストはそれがコミットした行を
// 検証します。
func newRevokeUserRoleUsecase(db *database.DB) *usecase.RevokeUserRoleUsecase {
	return usecase.NewRevokeUserRoleUsecase(
		db.Writer,
		repository.NewRoleRepository(db),
		repository.NewUserRepository(db),
		repository.NewUserRoleRepository(db),
	)
}

// countAdminHoldersは、退会も停止もしていない組み込みのadminロールの保持者数を
// 返します。最後の管理者の保護が残すのは有効な管理者であり、user_rolesの行だけでは
// ないため、テストはこの件数を検証します。
func countAdminHolders(t *testing.T, db *database.DB) int {
	t.Helper()

	var count int
	if err := db.Reader.QueryRowContext(context.Background(), `
		SELECT count(*)
		FROM user_roles
		JOIN roles ON roles.id = user_roles.role_id
		JOIN users ON users.id = user_roles.user_id
		WHERE roles.name = ? AND users.deleted_at IS NULL AND users.suspended_at IS NULL
	`, string(model.RoleNameAdmin)).Scan(&count); err != nil {
		t.Fatalf("管理者の人数の取得に失敗: %v", err)
	}
	return count
}

// TestRevokeUserRoleUsecase_Execute_Successは、管理者がもう1人の管理者からadmin
// ロールを取り上げられること (操作者自身が管理者として残る状態で) と、その後に割当が
// 消えていることを検証します。
func TestRevokeUserRoleUsecase_Execute_Success(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	uc := newRevokeUserRoleUsecase(db)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

	actorID := seedAdmin(t, db)
	targetID := testutil.NewUserBuilder(t, db).WithAtname("revoked").Build()
	testutil.NewUserRoleBuilder(t, db).WithUserID(targetID).Build()

	output, err := uc.Execute(ctx, usecase.RevokeUserRoleInput{
		Actor:        usecase.UserActor(actorID),
		TargetUserID: targetID,
		RoleName:     model.RoleNameAdmin,
	})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v、期待値 = nil", err)
	}

	if got := countUserRoles(t, db, targetID); got != 0 {
		t.Errorf("剥奪後の対象のロール割当数 = %d、期待値 = 0", got)
	}
	if output.TargetAtname != "revoked" {
		t.Errorf("TargetAtname = %q、期待値 = %q", output.TargetAtname, "revoked")
	}
	if got := countAdminHolders(t, db); got != 1 {
		t.Errorf("剥奪後の管理者の人数 = %d、期待値 = 1", got)
	}
}

// TestRevokeUserRoleUsecase_Execute_WithSuspendedAdminは、最後の管理者の保護が
// 有効な対象にだけ必要なことを検証します。停止中の保持者を外しても有効な人数は減りませんが、
// 唯一の有効な保持者を外すと減ります。
func TestRevokeUserRoleUsecase_Execute_WithSuspendedAdmin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		revokeSelf bool
	}{
		{name: "停止中の管理者からは剥奪できる"},
		{name: "ほかの管理者が停止中なら自己剥奪を拒否する", revokeSelf: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			db := testutil.SetupDB(t)
			ctx := i18n.SetLocale(context.Background(), model.LocaleJa)
			activeID := seedAdmin(t, db)
			suspendedID := testutil.NewUserBuilder(t, db).WithSuspendedAt(time.Now()).Build()
			testutil.NewUserRoleBuilder(t, db).WithUserID(suspendedID).Build()
			targetID := suspendedID
			if tt.revokeSelf {
				targetID = activeID
			}

			_, err := newRevokeUserRoleUsecase(db).Execute(ctx, usecase.RevokeUserRoleInput{
				Actor:        usecase.UserActor(activeID),
				TargetUserID: targetID,
				RoleName:     model.RoleNameAdmin,
			})
			wantRoles := 0
			if tt.revokeSelf {
				assertAppErrCode(t, err, model.AppErrCodeConflict)
				wantRoles = 1
			} else if err != nil {
				t.Fatalf("Execute()のエラー = %v、期待値 = nil", err)
			}
			if got := countUserRoles(t, db, targetID); got != wantRoles {
				t.Errorf("対象のロール割当数 = %d、期待値 = %d", got, wantRoles)
			}
			if got := countAdminHolders(t, db); got != 1 {
				t.Errorf("有効な管理者の人数 = %d、期待値 = 1", got)
			}
		})
	}
}

// TestRevokeUserRoleUsecase_Execute_SelfRevokeは、もう1人の管理者が残っている
// 状態で、管理者が自分で降りられることを検証します。降りるために別の管理者にやってもらう
// 必要はありません。
func TestRevokeUserRoleUsecase_Execute_SelfRevoke(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	uc := newRevokeUserRoleUsecase(db)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

	actorID := seedAdmin(t, db)
	seedAdmin(t, db)

	if _, err := uc.Execute(ctx, usecase.RevokeUserRoleInput{
		Actor:        usecase.UserActor(actorID),
		TargetUserID: actorID,
		RoleName:     model.RoleNameAdmin,
	}); err != nil {
		t.Fatalf("Execute()のエラー = %v、期待値 = nil", err)
	}

	if got := countUserRoles(t, db, actorID); got != 0 {
		t.Errorf("自身からの剥奪後のロール割当数 = %d、期待値 = 0", got)
	}
}

// TestRevokeUserRoleUsecase_Execute_SucceedsWhenNotHeldは、持っていないロールの
// 剥奪が成功することを検証します。要求が求めたのはその人がそのロールを持っていないこと
// であり、実際に持っていないためです。
func TestRevokeUserRoleUsecase_Execute_SucceedsWhenNotHeld(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	uc := newRevokeUserRoleUsecase(db)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

	actorID := seedAdmin(t, db)
	targetID := testutil.NewUserBuilder(t, db).Build()

	if _, err := uc.Execute(ctx, usecase.RevokeUserRoleInput{
		Actor:        usecase.UserActor(actorID),
		TargetUserID: targetID,
		RoleName:     model.RoleNameAdmin,
	}); err != nil {
		t.Fatalf("Execute()のエラー = %v、期待値 = nil", err)
	}

	if got := countAdminHolders(t, db); got != 1 {
		t.Errorf("剥奪後の管理者の人数 = %d、期待値 = 1 (操作者は管理者のまま)", got)
	}
}

// TestRevokeUserRoleUsecase_Execute_RefusesTheLastAdminは、残る唯一の管理者が、
// 自分自身からロールを外せないこと、そしてその拒否が権限の問題ではなく状態の競合である
// ことを検証します。
func TestRevokeUserRoleUsecase_Execute_RefusesTheLastAdmin(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	uc := newRevokeUserRoleUsecase(db)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

	adminID := seedAdmin(t, db)

	_, err := uc.Execute(ctx, usecase.RevokeUserRoleInput{
		Actor:        usecase.UserActor(adminID),
		TargetUserID: adminID,
		RoleName:     model.RoleNameAdmin,
	})
	assertAppErrCode(t, err, model.AppErrCodeConflict)

	if got := countAdminHolders(t, db); got != 1 {
		t.Errorf("拒否後の管理者の人数 = %d、期待値 = 1", got)
	}
}

// TestRevokeUserRoleUsecase_Execute_Forbiddenは、ロールを1つも持たない人が
// ロールを取り上げられないこと、そして割当がそのまま残ることを検証します。
func TestRevokeUserRoleUsecase_Execute_Forbidden(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	uc := newRevokeUserRoleUsecase(db)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

	actorID := testutil.NewUserBuilder(t, db).Build()
	targetID := seedAdmin(t, db)

	_, err := uc.Execute(ctx, usecase.RevokeUserRoleInput{
		Actor:        usecase.UserActor(actorID),
		TargetUserID: targetID,
		RoleName:     model.RoleNameAdmin,
	})
	assertAppErrCode(t, err, model.AppErrCodeForbidden)

	if got := countUserRoles(t, db, targetID); got != 1 {
		t.Errorf("拒否後の対象のロール割当数 = %d、期待値 = 1", got)
	}
}

// TestRevokeUserRoleUsecase_Execute_NotFoundは、どのロールも持たない名前、誰も持たない
// id、そして退会したアカウントのいずれもが、付与のときと同じ形で答えられることを検証します。
func TestRevokeUserRoleUsecase_Execute_NotFound(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	uc := newRevokeUserRoleUsecase(db)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

	actorID := seedAdmin(t, db)
	withdrawnID := testutil.NewUserBuilder(t, db).WithDeletedAt(time.Now()).Build()

	tests := []struct {
		name     string
		targetID model.UserID
		roleName model.RoleName
	}{
		{name: "存在しないロール名", targetID: actorID, roleName: "moderator"},
		{name: "存在しない利用者", targetID: model.UserID(999999), roleName: model.RoleNameAdmin},
		{name: "退会した利用者", targetID: withdrawnID, roleName: model.RoleNameAdmin},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := uc.Execute(ctx, usecase.RevokeUserRoleInput{
				Actor:        usecase.UserActor(actorID),
				TargetUserID: tt.targetID,
				RoleName:     tt.roleName,
			})
			assertAppErrCode(t, err, model.AppErrCodeResourceNotFound)
		})
	}
}

// revokeAdminAtOnceはすべての剥奪を同時に行い、それぞれが返したものを、渡された順で
// 返します。
//
// goroutineは送信の前に待ち合わせます。そうしなければ、1つが次の起動を待たずに完走して
// しまいうるためです。このテストが結果に問うことを決めるのは書き込みロックです。1つの
// 剥奪がトランザクションを開く間、他の剥奪は自身のトランザクションを開くのを待ちます。
func revokeAdminAtOnce(ctx context.Context, ucs []*usecase.RevokeUserRoleUsecase, targets []model.UserID) []error {
	errs := make([]error, len(ucs))
	start := make(chan struct{})

	var wg sync.WaitGroup
	for i := range ucs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, errs[i] = ucs[i].Execute(ctx, usecase.RevokeUserRoleInput{
				Actor:        usecase.OperatorActor(),
				TargetUserID: targets[i],
				RoleName:     model.RoleNameAdmin,
			})
		}()
	}
	close(start)
	wg.Wait()

	return errs
}

// TestRevokeUserRoleUsecase_Execute_ConcurrentAcrossConnectionPoolsは、最後の
// 2人の管理者に対して同時に届いた剥奪が、同じデータベースファイルに対して開かれた別々の
// 接続プールを使っていても、両方は通らないことを検証します。
//
// 1つのプールは書き込み用コネクションを1本に制限するため、そこを通る剥奪はSQLiteに
// 何かを尋ねる前に直列化されます。2つ目のプールを開くとそれが無くなり、結果はデータベース
// ファイル自身が持つ書き込みロックに委ねられます。_txlock=immediateにより、負けた側が
// 管理者を数えるのは勝った側がコミットした後になり、取り上げてはならない1人を見ます。
func TestRevokeUserRoleUsecase_Execute_ConcurrentAcrossConnectionPools(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)
	path := testutil.SetupDBPath(t)
	db := openDB(t, path)

	first := seedAdmin(t, db)
	second := seedAdmin(t, db)

	errs := revokeAdminAtOnce(ctx,
		[]*usecase.RevokeUserRoleUsecase{newRevokeUserRoleUsecase(db), newRevokeUserRoleUsecase(openDB(t, path))},
		[]model.UserID{first, second},
	)

	succeeded := 0
	for i, err := range errs {
		if err == nil {
			succeeded++
			continue
		}
		assertAppErrCode(t, err, model.AppErrCodeConflict)
		if t.Failed() {
			t.Fatalf("revocations[%d] のエラー = %v", i, err)
		}
	}
	if succeeded != 1 {
		t.Errorf("成功した剥奪 = %d 件、期待値 = %d 件", succeeded, 1)
	}

	if got := countAdminHolders(t, db); got != 1 {
		t.Errorf("同時剥奪後の管理者の人数 = %d、期待値 = 1", got)
	}
}
