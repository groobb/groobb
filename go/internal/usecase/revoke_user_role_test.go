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

// newRevokeUserRoleUsecase wires a RevokeUserRoleUsecase over the given database.
// The UseCase opens its own transaction, so a test asserts against the rows it
// commits.
//
// [Ja] newRevokeUserRoleUsecase は、渡したデータベース上に RevokeUserRoleUsecase を
// 組み立てます。UseCase は自前のトランザクションを開くため、テストはそれがコミットした行を
// 検証します。
func newRevokeUserRoleUsecase(db *database.DB) *usecase.RevokeUserRoleUsecase {
	return usecase.NewRevokeUserRoleUsecase(
		db.Writer,
		repository.NewRoleRepository(db),
		repository.NewUserRepository(db),
		repository.NewUserRoleRepository(db),
	)
}

// countAdminHolders returns how many people still hold the built-in admin role,
// counting only those who have not withdrawn. It is what the protection of the
// last administrator is about, so a test states its outcome in these terms
// rather than in rows of user_roles.
//
// [Ja] countAdminHolders は、組み込みの admin ロールを何人が持っているかを、退会していない
// 人だけ数えて返します。最後の管理者の保護が対象とするのがこれであるため、テストは結果を
// user_roles の行ではなくこの数で述べます。
func countAdminHolders(t *testing.T, db *database.DB) int {
	t.Helper()

	var count int
	if err := db.Reader.QueryRowContext(context.Background(), `
		SELECT count(*)
		FROM user_roles
		JOIN roles ON roles.id = user_roles.role_id
		JOIN users ON users.id = user_roles.user_id
		WHERE roles.name = ? AND users.deleted_at IS NULL
	`, string(model.RoleNameAdmin)).Scan(&count); err != nil {
		t.Fatalf("管理者の人数の取得に失敗: %v", err)
	}
	return count
}

// TestRevokeUserRoleUsecase_Execute_Success verifies that an administrator takes
// the admin role from another one while the acting administrator remains, and
// that the assignment is gone afterwards.
//
// [Ja] TestRevokeUserRoleUsecase_Execute_Success は、管理者がもう 1 人の管理者から admin
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
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	if got := countUserRoles(t, db, targetID); got != 0 {
		t.Errorf("剥奪後の対象のロール割当数 = %d, want 0", got)
	}
	if output.TargetAtname != "revoked" {
		t.Errorf("TargetAtname = %q, want %q", output.TargetAtname, "revoked")
	}
	if got := countAdminHolders(t, db); got != 1 {
		t.Errorf("剥奪後の管理者の人数 = %d, want 1", got)
	}
}

// TestRevokeUserRoleUsecase_Execute_SelfRevoke verifies that an administrator
// steps down on their own while another one remains. Stepping down does not
// require a second administrator to do it for them.
//
// [Ja] TestRevokeUserRoleUsecase_Execute_SelfRevoke は、もう 1 人の管理者が残っている
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
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	if got := countUserRoles(t, db, actorID); got != 0 {
		t.Errorf("自身からの剥奪後のロール割当数 = %d, want 0", got)
	}
}

// TestRevokeUserRoleUsecase_Execute_SucceedsWhenNotHeld verifies that revoking a
// role the person does not hold succeeds. What the request asked for is that they
// not hold it, and they do not.
//
// [Ja] TestRevokeUserRoleUsecase_Execute_SucceedsWhenNotHeld は、持っていないロールの
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
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	if got := countAdminHolders(t, db); got != 1 {
		t.Errorf("剥奪後の管理者の人数 = %d, want 1 (操作者は管理者のまま)", got)
	}
}

// TestRevokeUserRoleUsecase_Execute_RefusesTheLastAdmin verifies that the only
// administrator left cannot take the role away from themselves, and that the
// refusal is a conflict rather than a permission problem.
//
// [Ja] TestRevokeUserRoleUsecase_Execute_RefusesTheLastAdmin は、残る唯一の管理者が、
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
		t.Errorf("拒否後の管理者の人数 = %d, want 1", got)
	}
}

// TestRevokeUserRoleUsecase_Execute_Forbidden verifies that someone holding no
// role cannot take one away, and that the assignment is left alone.
//
// [Ja] TestRevokeUserRoleUsecase_Execute_Forbidden は、ロールを 1 つも持たない人が
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
		t.Errorf("拒否後の対象のロール割当数 = %d, want 1", got)
	}
}

// TestRevokeUserRoleUsecase_Execute_NotFound verifies that a role name nothing
// carries, an id nobody has, and an account that has withdrawn are all answered
// the same way, as they are when a role is granted.
//
// [Ja] TestRevokeUserRoleUsecase_Execute_NotFound は、どのロールも持たない名前、誰も持たない
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

// revokeAdminAtOnce sends every revocation at the same time and returns what each
// one answered, in the order they were given.
//
// The goroutines wait on a barrier before sending, so that the revocations are in
// flight together rather than one of them possibly finishing before the next is
// even started. What this test asks of the outcome is decided by the write lock:
// one revocation opens its transaction while the others wait to open theirs.
//
// [Ja] revokeAdminAtOnce はすべての剥奪を同時に行い、それぞれが返したものを、渡された順で
// 返します。
//
// goroutine は送信の前に待ち合わせます。そうしなければ、1 つが次の起動を待たずに完走して
// しまいうるためです。このテストが結果に問うことを決めるのは書き込みロックです。1 つの
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

// TestRevokeUserRoleUsecase_Execute_ConcurrentAcrossConnectionPools verifies that
// two revocations arriving at once for the last two administrators cannot both
// go through, even when they use separate connection pools opened on the same
// database file.
//
// A single pool caps the writer at one connection, so revocations through it are
// serialized before SQLite is asked anything. Opening a second pool takes that
// away and leaves the outcome to the write lock the database file itself holds:
// _txlock=immediate means the loser counts the administrators only after the
// winner has committed, and so sees the one it must not take away.
//
// [Ja] TestRevokeUserRoleUsecase_Execute_ConcurrentAcrossConnectionPools は、最後の
// 2 人の管理者に対して同時に届いた剥奪が、同じデータベースファイルに対して開かれた別々の
// 接続プールを使っていても、両方は通らないことを検証します。
//
// 1 つのプールは書き込み用コネクションを 1 本に制限するため、そこを通る剥奪は SQLite に
// 何かを尋ねる前に直列化されます。2 つ目のプールを開くとそれが無くなり、結果はデータベース
// ファイル自身が持つ書き込みロックに委ねられます。_txlock=immediate により、負けた側が
// 管理者を数えるのは勝った側がコミットした後になり、取り上げてはならない 1 人を見ます。
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
			t.Fatalf("revocations[%d] の error = %v", i, err)
		}
	}
	if succeeded != 1 {
		t.Errorf("成功した剥奪 = %d 件, want %d 件", succeeded, 1)
	}

	if got := countAdminHolders(t, db); got != 1 {
		t.Errorf("同時剥奪後の管理者の人数 = %d, want 1", got)
	}
}
