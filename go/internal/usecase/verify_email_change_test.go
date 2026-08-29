package usecase_test

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/dispatcher"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/validator"
)

// newVerifyEmailChangeUsecase wires a VerifyEmailChangeUsecase over the test's
// own database. The UseCase compares the code, stamps the confirmation, and
// updates the email in a transaction of its own, so those writes land or roll back
// together. It returns the repositories and the fake job inserter so a test can
// seed confirmations, assert the post-verification state, and inspect the enqueued
// notification.
//
// [Ja] newVerifyEmailChangeUsecase はテスト専用のデータベース上に
// VerifyEmailChangeUsecase を組み立てる。UseCase はコードの照合・確認の打刻・email の更新を
// 自前のトランザクションで行うため、これらの書き込みはまとめて確定するかロールバックする。
// テストが確認を仕込み、検証後の状態を確認し、投入された通知を検査できるようリポジトリと
// フェイクのジョブ inserter も返す。
func newVerifyEmailChangeUsecase(t *testing.T, db *database.DB) (*usecase.VerifyEmailChangeUsecase, *repository.EmailConfirmationRepository, *repository.UserRepository, *testutil.FakeJobInserter) {
	t.Helper()

	emailConfirmationRepo := repository.NewEmailConfirmationRepository(db)
	userRepo := repository.NewUserRepository(db)
	inserter := &testutil.FakeJobInserter{}
	uc := usecase.NewVerifyEmailChangeUsecase(
		db.Writer,
		validator.NewSettingsEmailConfirmationCreateValidator(emailConfirmationRepo),
		emailConfirmationRepo,
		userRepo,
		dispatcher.NewDispatcher(inserter),
	)
	return uc, emailConfirmationRepo, userRepo, inserter
}

// TestVerifyEmailChangeUsecase_Execute_Success verifies that a correct code stamps
// the confirmation succeeded (so it is no longer active) and applies the new
// address to the user's email.
//
// [Ja] TestVerifyEmailChangeUsecase_Execute_Success は、正しいコードが確認を成功済みとして
// 打刻し (これ以降 active でなくなる)、新しいアドレスをユーザーの email に適用することを
// 検証する。
func TestVerifyEmailChangeUsecase_Execute_Success(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	uc, repo, userRepo, inserter := newVerifyEmailChangeUsecase(t, db)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

	currentEmail := "ec-vc-cur@example.com"
	userID := seedEmailChangeUser(t, db, currentEmail)
	newEmail := "ec-vc-new@example.com"

	if _, err := repo.CreateEmailChange(ctx, repository.CreateEmailChangeInput{UserID: userID, Email: newEmail, Code: "123456"}); err != nil {
		t.Fatalf("メール変更確認の作成に失敗: %v", err)
	}

	out, err := uc.Execute(ctx, usecase.VerifyEmailChangeInput{UserID: userID, Code: "123456"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if out == nil || out.EmailConfirmation == nil {
		t.Fatal("Execute() output / EmailConfirmation = nil")
	}
	if out.EmailConfirmation.Email != newEmail {
		t.Errorf("out.EmailConfirmation.Email = %q, want %q", out.EmailConfirmation.Email, newEmail)
	}

	// The user's email is switched to the new address.
	//
	// [Ja] ユーザーの email が新しいアドレスに切り替わる。
	user, err := userRepo.FindByID(ctx, userID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}
	if user == nil || user.Email != newEmail {
		t.Errorf("user.Email = %v, want %q", user, newEmail)
	}

	// The confirmation is stamped succeeded, so it no longer qualifies as active.
	//
	// [Ja] 確認は成功済みとして打刻されたため、もはや active として該当しない。
	active, err := repo.FindActiveEmailChangeByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("FindActiveEmailChangeByUserID() error = %v", err)
	}
	if active != nil {
		t.Error("検証成功後は succeeded_at が打刻され active でなくなるはず")
	}

	// A change notification is enqueued to the old (current) address, carrying the
	// new address and the account's stored locale.
	//
	// [Ja] 変更通知が旧 (現在の) アドレス宛に、新しいアドレスとアカウントに保存された
	// ロケールを載せて投入される。
	if !inserter.Called {
		t.Fatal("メールアドレス変更通知が投入されていない")
	}
	args, ok := inserter.Args.(dispatcher.SendEmailChangeNotificationArgs)
	if !ok {
		t.Fatalf("投入された Args の型 = %T, want dispatcher.SendEmailChangeNotificationArgs", inserter.Args)
	}
	if args.Email != currentEmail {
		t.Errorf("通知の宛先 = %q, want %q (旧アドレス)", args.Email, currentEmail)
	}
	if args.NewEmail != newEmail {
		t.Errorf("通知の NewEmail = %q, want %q", args.NewEmail, newEmail)
	}
	if args.Locale != "ja" {
		t.Errorf("通知の Locale = %q, want %q (アカウントのロケール)", args.Locale, "ja")
	}
}

// TestVerifyEmailChangeUsecase_Execute_WrongCode verifies that a wrong code fails
// with a form-wide ValidationError, increments the failed-attempt count, leaves the
// confirmation active, and does not change the user's email.
//
// [Ja] TestVerifyEmailChangeUsecase_Execute_WrongCode は、誤ったコードがフォーム全体の
// ValidationError で失敗し、失敗試行回数をインクリメントし、確認を active のまま残し、
// ユーザーの email を変更しないことを検証する。
func TestVerifyEmailChangeUsecase_Execute_WrongCode(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	uc, repo, userRepo, inserter := newVerifyEmailChangeUsecase(t, db)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

	currentEmail := "ec-vc-wc-cur@example.com"
	userID := seedEmailChangeUser(t, db, currentEmail)
	newEmail := "ec-vc-wc-new@example.com"

	if _, err := repo.CreateEmailChange(ctx, repository.CreateEmailChangeInput{UserID: userID, Email: newEmail, Code: "123456"}); err != nil {
		t.Fatalf("メール変更確認の作成に失敗: %v", err)
	}

	out, err := uc.Execute(ctx, usecase.VerifyEmailChangeInput{UserID: userID, Code: "000000"})
	if out != nil {
		t.Errorf("Execute() output = %v, want nil", out)
	}
	ve := model.AsValidationError(err)
	if ve == nil {
		t.Fatalf("Execute() error = %v, want *model.ValidationError", err)
	}
	if !ve.HasGlobalError() {
		t.Errorf("フォーム全体のエラーが無い: %+v", ve.Global)
	}

	// The email is untouched on a wrong code.
	//
	// [Ja] 誤ったコードでは email は変更されない。
	user, err := userRepo.FindByID(ctx, userID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}
	if user == nil || user.Email != currentEmail {
		t.Errorf("user.Email = %v, want %q (誤ったコードで変更されてはならない)", user, currentEmail)
	}

	// The confirmation stays active with its failed-attempt count incremented.
	//
	// [Ja] 確認は失敗試行回数をインクリメントされ active のまま残る。
	active, err := repo.FindActiveEmailChangeByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("FindActiveEmailChangeByUserID() error = %v", err)
	}
	if active == nil {
		t.Fatal("検証失敗時は succeeded_at を打刻せず active のままのはず")
	}
	if active.FailedAttemptsCount != 1 {
		t.Errorf("active.FailedAttemptsCount = %d, want 1", active.FailedAttemptsCount)
	}

	// No change happened, so no notification is enqueued.
	//
	// [Ja] 変更が起きていないため、通知は投入されない。
	if inserter.Called {
		t.Error("変更が成立していないのに通知が投入された")
	}
}

// TestVerifyEmailChangeUsecase_Execute_UniqueConflict verifies that when the new
// address has been claimed by another account by the time the code is confirmed,
// the correct code fails with a form-wide ValidationError, the user's email is left
// unchanged, and the confirmation is not stamped succeeded (the transaction rolls
// back).
//
// [Ja] TestVerifyEmailChangeUsecase_Execute_UniqueConflict は、コード確認の時点で新しい
// アドレスが別アカウントに取得されていた場合、正しいコードがフォーム全体の
// ValidationError で失敗し、ユーザーの email が変更されず、確認が成功済みに打刻されない
// (トランザクションがロールバックする) ことを検証する。
func TestVerifyEmailChangeUsecase_Execute_UniqueConflict(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	uc, repo, userRepo, inserter := newVerifyEmailChangeUsecase(t, db)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

	currentEmail := "ec-vc-cf-cur@example.com"
	takenEmail := "ec-vc-cf-taken@example.com"

	userID := seedEmailChangeUser(t, db, currentEmail)
	// Another account already holds the address the confirmation targets, so the
	// apply hits the users.email UNIQUE constraint.
	//
	// [Ja] 確認が対象とするアドレスを別アカウントが既に保持しているため、適用は
	// users.email の UNIQUE 制約に当たる。
	seedEmailChangeUser(t, db, takenEmail)

	if _, err := repo.CreateEmailChange(ctx, repository.CreateEmailChangeInput{UserID: userID, Email: takenEmail, Code: "123456"}); err != nil {
		t.Fatalf("メール変更確認の作成に失敗: %v", err)
	}

	out, err := uc.Execute(ctx, usecase.VerifyEmailChangeInput{UserID: userID, Code: "123456"})
	if out != nil {
		t.Errorf("Execute() output = %v, want nil", out)
	}
	ve := model.AsValidationError(err)
	if ve == nil {
		t.Fatalf("Execute() error = %v, want *model.ValidationError (unique race)", err)
	}
	if !ve.HasGlobalError() {
		t.Errorf("フォーム全体のエラーが無い: %+v", ve.Global)
	}

	// The email must not have changed.
	//
	// [Ja] email は変更されていてはならない。
	user, err := userRepo.FindByID(ctx, userID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}
	if user == nil || user.Email != currentEmail {
		t.Errorf("user.Email = %v, want %q (競合時に変更されてはならない)", user, currentEmail)
	}

	// The stamp is rolled back with the failed update, so the confirmation stays
	// active.
	//
	// [Ja] 打刻は失敗した更新とともにロールバックされるため、確認は active のまま残る。
	active, err := repo.FindActiveEmailChangeByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("FindActiveEmailChangeByUserID() error = %v", err)
	}
	if active == nil {
		t.Error("競合時は打刻をロールバックし active のまま残るはず")
	}

	// The change did not commit, so no notification is enqueued.
	//
	// [Ja] 変更はコミットされていないため、通知は投入されない。
	if inserter.Called {
		t.Error("変更が成立していないのに通知が投入された")
	}
}
