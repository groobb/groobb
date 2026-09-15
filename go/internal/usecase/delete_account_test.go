package usecase_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/groobb/groobb/go/internal/auth"
	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/validator"
)

// newDeleteAccountUsecaseはテスト専用のデータベース上にDeleteAccountUsecaseを
// 組み立てます。UseCaseは自前のトランザクションを開くため、テストはそれがコミットした行を
// 検証します。
func newDeleteAccountUsecase(t *testing.T, db *database.DB) *usecase.DeleteAccountUsecase {
	t.Helper()

	userRepo := repository.NewUserRepository(db)
	userPasswordRepo := repository.NewUserPasswordRepository(db)
	userSessionRepo := repository.NewUserSessionRepository(db)
	roleRepo := repository.NewRoleRepository(db)
	userRoleRepo := repository.NewUserRoleRepository(db)

	return usecase.NewDeleteAccountUsecase(
		db.Writer,
		validator.NewSettingsWithdrawalDeleteValidator(userPasswordRepo),
		userRepo,
		userSessionRepo,
		roleRepo,
		userRoleRepo,
	)
}

// seedWithdrawalUserはパスワード "password123" と2つの有効なセッションを持つ
// コミット済みユーザーを作成し、そのidを返す。UseCaseテストが実在の認証可能な
// アカウントから退会を駆動し、セッションが消えることを検証できるようにする。
func seedWithdrawalUser(t *testing.T, db *database.DB) model.UserID {
	t.Helper()

	ctx := context.Background()
	userRepo := repository.NewUserRepository(db)
	userPasswordRepo := repository.NewUserPasswordRepository(db)
	userSessionRepo := repository.NewUserSessionRepository(db)

	user, err := userRepo.Create(ctx, repository.CreateUserInput{
		Email:    "wd-uc@example.com",
		Atname:   testutil.UniqueAtname(db),
		Locale:   "ja",
		TimeZone: "Asia/Tokyo",
	})
	if err != nil {
		t.Fatalf("テスト用ユーザーの作成に失敗: %v", err)
	}
	digest, err := auth.HashPassword("password123")
	if err != nil {
		t.Fatalf("パスワードのハッシュ化に失敗: %v", err)
	}
	if _, err := userPasswordRepo.Create(ctx, repository.CreateUserPasswordInput{
		UserID:         user.ID,
		PasswordDigest: digest,
	}); err != nil {
		t.Fatalf("テスト用パスワードの作成に失敗: %v", err)
	}
	for i := 0; i < 2; i++ {
		if _, err := userSessionRepo.Create(ctx, repository.CreateUserSessionInput{
			UserID:    user.ID,
			Token:     fmt.Sprintf("wd-token-%d", i),
			IPAddress: "127.0.0.1",
			UserAgent: "test-user-agent",
		}); err != nil {
			t.Fatalf("テスト用セッションの作成に失敗: %v", err)
		}
	}
	return user.ID
}

// countUserSessionsは指定ユーザーがまだ所有するセッション数を返す。退会が
// それらを消したこと (または拒否された退会がそれらを残したこと) を検証するために使う。
func countUserSessions(t *testing.T, db *database.DB, userID model.UserID) int {
	t.Helper()

	var count int
	if err := db.Reader.QueryRowContext(context.Background(),
		`SELECT count(*) FROM user_sessions WHERE user_id = ?`, int64(userID),
	).Scan(&count); err != nil {
		t.Fatalf("セッション件数の取得に失敗: %v", err)
	}
	return count
}

// TestDeleteAccountUsecase_Execute_Successは、有効な退会がユーザーを論理削除し
// (deleted_atを打つ)、解放されたemail / atnameをid由来の代替値で匿名化し、そのユーザーの
// 全セッションを削除することを検証する。
func TestDeleteAccountUsecase_Execute_Success(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	uc := newDeleteAccountUsecase(t, db)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)
	userID := seedWithdrawalUser(t, db)

	if err := uc.Execute(ctx, usecase.DeleteAccountInput{
		UserID:          userID,
		CurrentPassword: "password123",
	}); err != nil {
		t.Fatalf("Execute()のエラー = %v、期待値 = nil", err)
	}

	// 行は (deleted_atで絞るルックアップではなく) 直接クエリするため、論理削除・
	// 匿名化されたユーザーも観測できる。
	var deletedAt *time.Time
	var email, atname string
	if err := db.Reader.QueryRowContext(ctx,
		`SELECT deleted_at, email, atname FROM users WHERE id = ?`, int64(userID),
	).Scan(&deletedAt, &email, &atname); err != nil {
		t.Fatalf("退会後のユーザー行の取得に失敗: %v", err)
	}
	if deletedAt == nil {
		t.Error("deleted_atがセットされていない (論理削除されていない)")
	}

	wantEmail := fmt.Sprintf("deleted-%s@deleted.invalid", userID.String())
	if email != wantEmail {
		t.Errorf("email = %q、期待値 = %q (匿名化されるべき)", email, wantEmail)
	}
	wantAtname := "deleted-" + userID.String()
	if atname != wantAtname {
		t.Errorf("atname = %q、期待値 = %q (匿名化されるべき)", atname, wantAtname)
	}

	if got := countUserSessions(t, db, userID); got != 0 {
		t.Errorf("退会後のセッション数 = %d、期待値 = 0 (全端末サインアウト)", got)
	}
}

// TestDeleteAccountUsecase_Execute_ValidationErrorは、誤った現在のパスワードが
// *model.ValidationErrorで失敗し、アカウントを完全に無傷のまま (論理削除されず、
// セッションも残ったまま) にすることを検証する。
func TestDeleteAccountUsecase_Execute_ValidationError(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	uc := newDeleteAccountUsecase(t, db)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)
	userID := seedWithdrawalUser(t, db)

	err := uc.Execute(ctx, usecase.DeleteAccountInput{
		UserID:          userID,
		CurrentPassword: "wrongpassword",
	})
	if ve := model.AsValidationError(err); ve == nil {
		t.Fatalf("Execute()のエラー = %v、期待値 = *model.ValidationError", err)
	}

	var deletedAt *time.Time
	if err := db.Reader.QueryRowContext(ctx,
		`SELECT deleted_at FROM users WHERE id = ?`, int64(userID),
	).Scan(&deletedAt); err != nil {
		t.Fatalf("ユーザー行の取得に失敗: %v", err)
	}
	if deletedAt != nil {
		t.Error("バリデーション失敗時にユーザーが論理削除された")
	}

	if got := countUserSessions(t, db, userID); got != 2 {
		t.Errorf("バリデーション失敗時のセッション数 = %d、期待値 = 2 (削除されるべきでない)", got)
	}
}

// TestDeleteAccountUsecase_Execute_SucceedsWhenALookAlikeAtnameIsTakenは、
// 通常のフォームから登録したアカウントが、退会が自身のatnameを上書きするのに使う値を
// 保持できず、したがってusers.atnameのUNIQUE制約でその退会を止められないことを検証する。
//
// ここで先取りするのは、フォームが実際に受け付ける中で最も墓標に近いatname、すなわち
// 墓標のハイフン (atnameの形式が拒否する文字) をアンダースコアに替えたものである。
func TestDeleteAccountUsecase_Execute_SucceedsWhenALookAlikeAtnameIsTaken(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	uc := newDeleteAccountUsecase(t, db)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)
	userID := seedWithdrawalUser(t, db)

	lookAlike := fmt.Sprintf("deleted_%d", int64(userID))
	if err := validator.NewAccountCreateValidator(repository.NewUserRepository(db)).Validate(ctx,
		validator.AccountCreateValidatorInput{
			Atname:               lookAlike,
			Password:             "password123",
			PasswordConfirmation: "password123",
		},
	); err != nil {
		t.Fatalf("先取りに使うatname %q はフォームから登録できる想定: %v", lookAlike, err)
	}
	testutil.NewUserBuilder(t, db).WithAtname(lookAlike).Build()

	if err := uc.Execute(ctx, usecase.DeleteAccountInput{
		UserID:          userID,
		CurrentPassword: "password123",
	}); err != nil {
		t.Fatalf("Execute()のエラー = %v、期待値 = nil (先取りされたatnameが退会を止めてはならない)", err)
	}

	var atname string
	if err := db.Reader.QueryRowContext(ctx,
		`SELECT atname FROM users WHERE id = ?`, int64(userID),
	).Scan(&atname); err != nil {
		t.Fatalf("退会後のユーザー行の取得に失敗: %v", err)
	}
	if atname == lookAlike {
		t.Errorf("墓標atname = %q で、フォームから登録できる値と同じになっている", atname)
	}
}

// countUserRolesは指定ユーザーがまだ持つロールの数を返す。退会がそれらを消したこと
// (または拒否された退会がそれらを残したこと) を検証するために使う。
func countUserRoles(t *testing.T, db *database.DB, userID model.UserID) int {
	t.Helper()

	var count int
	if err := db.Reader.QueryRowContext(context.Background(),
		`SELECT count(*) FROM user_roles WHERE user_id = ?`, int64(userID),
	).Scan(&count); err != nil {
		t.Fatalf("ロール割当の件数の取得に失敗: %v", err)
	}
	return count
}

// TestDeleteAccountUsecase_Execute_RefusesTheLastAdminは、残る唯一の管理者が退会
// できないこと、そしてその拒否がフォーム全体のバリデーションエラーであり、アカウントと
// そのロールを手つかずのまま残すことを検証する。
//
// これを通せば管理画面を開ける人が誰もいなくなり、管理者を立てるのはその画面である。
func TestDeleteAccountUsecase_Execute_RefusesTheLastAdmin(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	uc := newDeleteAccountUsecase(t, db)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)
	userID := seedWithdrawalUser(t, db)
	testutil.NewUserRoleBuilder(t, db).WithUserID(userID).Build()

	err := uc.Execute(ctx, usecase.DeleteAccountInput{
		UserID:          userID,
		CurrentPassword: "password123",
	})
	ve := model.AsValidationError(err)
	if ve == nil {
		t.Fatalf("Execute()のエラー = %v、期待値 = *model.ValidationError", err)
	}
	if !ve.HasGlobalError() {
		t.Errorf("拒否がフォーム全体のエラーを持っていない: %+v", ve)
	}

	var deletedAt *time.Time
	if err := db.Reader.QueryRowContext(ctx,
		`SELECT deleted_at FROM users WHERE id = ?`, int64(userID),
	).Scan(&deletedAt); err != nil {
		t.Fatalf("ユーザー行の取得に失敗: %v", err)
	}
	if deletedAt != nil {
		t.Error("最後の管理者の退会が拒否されたのにユーザーが論理削除された")
	}
	if got := countUserRoles(t, db, userID); got != 1 {
		t.Errorf("拒否後のロール割当数 = %d、期待値 = 1 (削除されるべきでない)", got)
	}
}

// TestDeleteAccountUsecase_Execute_RefusesWhenTheOtherAdminIsSuspendedは、停止中の
// 管理者が、まだサインインできる管理者のために退会の道を開けたままにしないことを検証する。
//
// 停止中の保持者を数えれば、行動できる最後の管理者が去れてしまい、その停止を解除するのは、
// そのとき誰も開けなくなる管理画面である。
func TestDeleteAccountUsecase_Execute_RefusesWhenTheOtherAdminIsSuspended(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	uc := newDeleteAccountUsecase(t, db)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)
	userID := seedWithdrawalUser(t, db)
	testutil.NewUserRoleBuilder(t, db).WithUserID(userID).Build()

	suspendedAdminID := testutil.NewUserBuilder(t, db).WithSuspendedAt(time.Now()).Build()
	testutil.NewUserRoleBuilder(t, db).WithUserID(suspendedAdminID).Build()

	err := uc.Execute(ctx, usecase.DeleteAccountInput{
		UserID:          userID,
		CurrentPassword: "password123",
	})
	ve := model.AsValidationError(err)
	if ve == nil {
		t.Fatalf("Execute()のエラー = %v、期待値 = *model.ValidationError (停止中の管理者は数に含まれない)", err)
	}
	if !ve.HasGlobalError() {
		t.Errorf("拒否がフォーム全体のエラーを持っていない: %+v", ve)
	}
	if got := countUserRoles(t, db, userID); got != 1 {
		t.Errorf("拒否後のロール割当数 = %d、期待値 = 1 (削除されるべきでない)", got)
	}
}

// TestDeleteAccountUsecase_Execute_DeletesTheRolesWhenAnotherAdminRemainsは、
// もう1人の管理者が残っている状態で管理者が退会できること、そしてそのアカウントが持って
// いたものの保持者として数えられなくなることを検証する。
func TestDeleteAccountUsecase_Execute_DeletesTheRolesWhenAnotherAdminRemains(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	uc := newDeleteAccountUsecase(t, db)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)
	userID := seedWithdrawalUser(t, db)
	testutil.NewUserRoleBuilder(t, db).WithUserID(userID).Build()

	otherAdminID := testutil.NewUserBuilder(t, db).Build()
	testutil.NewUserRoleBuilder(t, db).WithUserID(otherAdminID).Build()

	if err := uc.Execute(ctx, usecase.DeleteAccountInput{
		UserID:          userID,
		CurrentPassword: "password123",
	}); err != nil {
		t.Fatalf("Execute()のエラー = %v、期待値 = nil (もう1人の管理者が残っている)", err)
	}

	if got := countUserRoles(t, db, userID); got != 0 {
		t.Errorf("退会後のロール割当数 = %d、期待値 = 0", got)
	}
	if got := countUserRoles(t, db, otherAdminID); got != 1 {
		t.Errorf("残る管理者のロール割当数 = %d、期待値 = 1", got)
	}
}
