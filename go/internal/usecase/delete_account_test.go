package usecase_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/groobb/groobb/go/internal/auth"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/query"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/validator"
)

// newDeleteAccountUsecase wires a DeleteAccountUsecase over the shared pool. The
// UseCase opens its own transaction, so its tests commit rows and use unique
// identifiers (the test database is reset by make test) rather than the
// rolled-back transaction pattern.
//
// [Ja] newDeleteAccountUsecase は共有プール上に DeleteAccountUsecase を組み立てます。
// UseCase は自前のトランザクションを開くため、そのテストはロールバックされるトランザクション
// パターンではなく、行をコミットしユニークな識別子を使います (テスト DB は make test が
// リセットする)。
func newDeleteAccountUsecase(t *testing.T) *usecase.DeleteAccountUsecase {
	t.Helper()

	db := testutil.GetTestDB()
	queries := query.New(db)
	userRepo := repository.NewUserRepository(queries)
	userPasswordRepo := repository.NewUserPasswordRepository(queries)
	userSessionRepo := repository.NewUserSessionRepository(queries)

	return usecase.NewDeleteAccountUsecase(
		db,
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
func seedWithdrawalUser(t *testing.T) model.UserID {
	t.Helper()

	ctx := context.Background()
	queries := query.New(testutil.GetTestDB())
	userRepo := repository.NewUserRepository(queries)
	userPasswordRepo := repository.NewUserPasswordRepository(queries)
	userSessionRepo := repository.NewUserSessionRepository(queries)

	user, err := userRepo.Create(ctx, repository.CreateUserInput{
		Email:    fmt.Sprintf("wd-uc-%s@example.com", uuid.NewString()),
		Atname:   testutil.UniqueAtname(),
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
			Token:     fmt.Sprintf("wd-token-%d-%s", i, uuid.NewString()),
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
func countUserSessions(t *testing.T, userID model.UserID) int {
	t.Helper()

	var count int
	if err := testutil.GetTestDB().QueryRow(context.Background(),
		`SELECT count(*) FROM user_sessions WHERE user_id = $1`, uuid.UUID(userID),
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

	uc := newDeleteAccountUsecase(t)
	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	userID := seedWithdrawalUser(t)

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
	if err := testutil.GetTestDB().QueryRow(ctx,
		`SELECT deleted_at, email, atname FROM users WHERE id = $1`, uuid.UUID(userID),
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
	wantAtname := "deleted_" + strings.ReplaceAll(userID.String(), "-", "")
	if atname != wantAtname {
		t.Errorf("atname = %q, want %q (匿名化されるべき)", atname, wantAtname)
	}

	if got := countUserSessions(t, userID); got != 0 {
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

	uc := newDeleteAccountUsecase(t)
	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	userID := seedWithdrawalUser(t)

	err := uc.Execute(ctx, usecase.DeleteAccountInput{
		UserID:          userID,
		CurrentPassword: "wrongpassword",
	})
	if ve := model.AsValidationError(err); ve == nil {
		t.Fatalf("Execute() error = %v, want *model.ValidationError", err)
	}

	var deletedAt *time.Time
	if err := testutil.GetTestDB().QueryRow(ctx,
		`SELECT deleted_at FROM users WHERE id = $1`, uuid.UUID(userID),
	).Scan(&deletedAt); err != nil {
		t.Fatalf("ユーザー行の取得に失敗: %v", err)
	}
	if deletedAt != nil {
		t.Error("バリデーション失敗時にユーザーが論理削除された")
	}

	if got := countUserSessions(t, userID); got != 2 {
		t.Errorf("バリデーション失敗時のセッション数 = %d, want 2 (削除されるべきでない)", got)
	}
}
