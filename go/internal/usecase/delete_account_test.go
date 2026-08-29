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

// newDeleteAccountUsecase wires a DeleteAccountUsecase over the test's own
// database. The UseCase opens its own transaction, so a test asserts against the
// rows it commits.
//
// [Ja] newDeleteAccountUsecase はテスト専用のデータベース上に DeleteAccountUsecase を
// 組み立てます。UseCase は自前のトランザクションを開くため、テストはそれがコミットした行を
// 検証します。
func newDeleteAccountUsecase(t *testing.T, db *database.DB) *usecase.DeleteAccountUsecase {
	t.Helper()

	userRepo := repository.NewUserRepository(db)
	userPasswordRepo := repository.NewUserPasswordRepository(db)
	userSessionRepo := repository.NewUserSessionRepository(db)

	return usecase.NewDeleteAccountUsecase(
		db.Writer,
		validator.NewSettingsWithdrawalDeleteValidator(userPasswordRepo),
		userRepo,
		userSessionRepo,
	)
}

// seedWithdrawalUser creates a committed user with the password "password123" and
// two live sessions, returning its id so a UseCase test can drive a withdrawal
// from a real, authenticatable account and assert its sessions are cleared.
//
// [Ja] seedWithdrawalUser はパスワード "password123" と 2 つの有効なセッションを持つ
// コミット済みユーザーを作成し、その id を返す。UseCase テストが実在の認証可能な
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

// countUserSessions returns how many sessions the given user still owns, for
// asserting that withdrawal cleared them (or that a rejected withdrawal left them).
//
// [Ja] countUserSessions は指定ユーザーがまだ所有するセッション数を返す。退会が
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

// TestDeleteAccountUsecase_Execute_Success verifies that a valid withdrawal
// soft-deletes the user (stamping deleted_at), anonymizes the freed email/atname
// with the id-derived placeholders, and deletes all of the user's sessions.
//
// [Ja] TestDeleteAccountUsecase_Execute_Success は、有効な退会がユーザーを論理削除し
// (deleted_at を打つ)、解放された email / atname を id 由来の代替値で匿名化し、そのユーザーの
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
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	// The row is queried directly (not via a lookup that filters deleted_at) so the
	// soft-deleted, anonymized user is still observable.
	//
	// [Ja] 行は (deleted_at で絞るルックアップではなく) 直接クエリするため、論理削除・
	// 匿名化されたユーザーも観測できる。
	var deletedAt *time.Time
	var email, atname string
	if err := db.Reader.QueryRowContext(ctx,
		`SELECT deleted_at, email, atname FROM users WHERE id = ?`, int64(userID),
	).Scan(&deletedAt, &email, &atname); err != nil {
		t.Fatalf("退会後のユーザー行の取得に失敗: %v", err)
	}
	if deletedAt == nil {
		t.Error("deleted_at がセットされていない (論理削除されていない)")
	}

	wantEmail := fmt.Sprintf("deleted-%s@deleted.invalid", userID.String())
	if email != wantEmail {
		t.Errorf("email = %q, want %q (匿名化されるべき)", email, wantEmail)
	}
	wantAtname := "deleted-" + userID.String()
	if atname != wantAtname {
		t.Errorf("atname = %q, want %q (匿名化されるべき)", atname, wantAtname)
	}

	if got := countUserSessions(t, db, userID); got != 0 {
		t.Errorf("退会後のセッション数 = %d, want 0 (全端末サインアウト)", got)
	}
}

// TestDeleteAccountUsecase_Execute_ValidationError verifies that a wrong current
// password fails with a *model.ValidationError and leaves the account fully intact:
// not soft-deleted and with its sessions still present.
//
// [Ja] TestDeleteAccountUsecase_Execute_ValidationError は、誤った現在のパスワードが
// *model.ValidationError で失敗し、アカウントを完全に無傷のまま (論理削除されず、
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
		t.Fatalf("Execute() error = %v, want *model.ValidationError", err)
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
		t.Errorf("バリデーション失敗時のセッション数 = %d, want 2 (削除されるべきでない)", got)
	}
}

// TestDeleteAccountUsecase_Execute_SucceedsWhenALookAlikeAtnameIsTaken verifies
// that an account registered through the normal form cannot hold the atname a
// withdrawal overwrites its own atname with, and so cannot block that withdrawal
// on the users.atname UNIQUE constraint.
//
// The squatter here takes the closest atname the form does accept: the tombstone
// with its hyphen (which the atname format rejects) swapped for an underscore.
// That value used to be the tombstone itself, which made a squatted withdrawal
// fail with a constraint error the user could never resolve.
//
// [Ja] TestDeleteAccountUsecase_Execute_SucceedsWhenALookAlikeAtnameIsTaken は、
// 通常のフォームから登録したアカウントが、退会が自身の atname を上書きするのに使う値を
// 保持できず、したがって users.atname の UNIQUE 制約でその退会を止められないことを検証する。
//
// ここで先取りするのは、フォームが実際に受け付ける中で最も墓標に近い atname、すなわち
// 墓標のハイフン (atname の形式が拒否する文字) をアンダースコアに替えたものである。
// この値はかつて墓標そのものであり、先取りされた退会がユーザーには解消できない制約エラーで
// 失敗する原因になっていた。
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
		t.Fatalf("先取りに使う atname %q はフォームから登録できる想定: %v", lookAlike, err)
	}
	testutil.NewUserBuilder(t, db).WithAtname(lookAlike).Build()

	if err := uc.Execute(ctx, usecase.DeleteAccountInput{
		UserID:          userID,
		CurrentPassword: "password123",
	}); err != nil {
		t.Fatalf("Execute() error = %v, want nil (先取りされた atname が退会を止めてはならない)", err)
	}

	var atname string
	if err := db.Reader.QueryRowContext(ctx,
		`SELECT atname FROM users WHERE id = ?`, int64(userID),
	).Scan(&atname); err != nil {
		t.Fatalf("退会後のユーザー行の取得に失敗: %v", err)
	}
	if atname == lookAlike {
		t.Errorf("墓標 atname = %q で、フォームから登録できる値と同じになっている", atname)
	}
}
