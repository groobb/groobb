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

// newVerifyEmailChangeUsecaseはテスト専用のデータベース上に
// VerifyEmailChangeUsecaseを組み立てる。UseCaseはコードの照合・確認の打刻・emailの更新を
// 自前のトランザクションで行うため、これらの書き込みはまとめて確定するかロールバックする。
// テストが確認を仕込み、検証後の状態を確認し、投入された通知を検査できるようリポジトリと
// フェイクのジョブinserterも返す。
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

// TestVerifyEmailChangeUsecase_Execute_Successは、正しいコードが確認を成功済みとして
// 打刻し (これ以降activeでなくなる)、新しいアドレスをユーザーのemailに適用することを
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
		t.Fatalf("Execute()のエラー = %v", err)
	}
	if out == nil || out.EmailConfirmation == nil {
		t.Fatal("Execute()のoutput / EmailConfirmation = nil")
	}
	if out.EmailConfirmation.Email != newEmail {
		t.Errorf("out.EmailConfirmation.Email = %q、期待値 = %q", out.EmailConfirmation.Email, newEmail)
	}

	// ユーザーのemailが新しいアドレスに切り替わる。
	user, err := userRepo.FindByID(ctx, userID)
	if err != nil {
		t.Fatalf("FindByID()のエラー = %v", err)
	}
	if user == nil || user.Email != newEmail {
		t.Errorf("user.Email = %v、期待値 = %q", user, newEmail)
	}

	// 確認は成功済みとして打刻されたため、もはやactiveとして該当しない。
	active, err := repo.FindActiveEmailChangeByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("FindActiveEmailChangeByUserID()のエラー = %v", err)
	}
	if active != nil {
		t.Error("検証成功後はsucceeded_atが打刻されactiveでなくなるはず")
	}

	// 変更通知が旧 (現在の) アドレス宛に、新しいアドレスとアカウントに保存された
	// ロケールを載せて投入される。
	if !inserter.Called {
		t.Fatal("メールアドレス変更通知が投入されていない")
	}
	args, ok := inserter.Args.(dispatcher.SendEmailChangeNotificationArgs)
	if !ok {
		t.Fatalf("投入されたArgsの型 = %T、期待値 = dispatcher.SendEmailChangeNotificationArgs", inserter.Args)
	}
	if args.Email != currentEmail {
		t.Errorf("通知の宛先 = %q、期待値 = %q (旧アドレス)", args.Email, currentEmail)
	}
	if args.NewEmail != newEmail {
		t.Errorf("通知のNewEmail = %q、期待値 = %q", args.NewEmail, newEmail)
	}
	if args.Locale != "ja" {
		t.Errorf("通知のLocale = %q、期待値 = %q (アカウントのロケール)", args.Locale, "ja")
	}
}

// TestVerifyEmailChangeUsecase_Execute_WrongCodeは、誤ったコードがフォーム全体の
// ValidationErrorで失敗し、失敗試行回数をインクリメントし、確認をactiveのまま残し、
// ユーザーのemailを変更しないことを検証する。
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
		t.Errorf("Execute()のoutput = %v、期待値 = nil", out)
	}
	ve := model.AsValidationError(err)
	if ve == nil {
		t.Fatalf("Execute()のエラー = %v、期待値 = *model.ValidationError", err)
	}
	if !ve.HasGlobalError() {
		t.Errorf("フォーム全体のエラーが無い: %+v", ve.Global)
	}

	// 誤ったコードではemailは変更されない。
	user, err := userRepo.FindByID(ctx, userID)
	if err != nil {
		t.Fatalf("FindByID()のエラー = %v", err)
	}
	if user == nil || user.Email != currentEmail {
		t.Errorf("user.Email = %v、期待値 = %q (誤ったコードで変更されてはならない)", user, currentEmail)
	}

	// 確認は失敗試行回数をインクリメントされactiveのまま残る。
	active, err := repo.FindActiveEmailChangeByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("FindActiveEmailChangeByUserID()のエラー = %v", err)
	}
	if active == nil {
		t.Fatal("検証失敗時はsucceeded_atを打刻せずactiveのままのはず")
	}
	if active.FailedAttemptsCount != 1 {
		t.Errorf("active.FailedAttemptsCount = %d、期待値 = 1", active.FailedAttemptsCount)
	}

	// 変更が起きていないため、通知は投入されない。
	if inserter.Called {
		t.Error("変更が成立していないのに通知が投入された")
	}
}

// TestVerifyEmailChangeUsecase_Execute_UniqueConflictは、コード確認の時点で新しい
// アドレスが別アカウントに取得されていた場合、正しいコードがフォーム全体の
// ValidationErrorで失敗し、ユーザーのemailが変更されず、確認が成功済みに打刻されない
// (トランザクションがロールバックする) ことを検証する。
func TestVerifyEmailChangeUsecase_Execute_UniqueConflict(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	uc, repo, userRepo, inserter := newVerifyEmailChangeUsecase(t, db)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

	currentEmail := "ec-vc-cf-cur@example.com"
	takenEmail := "ec-vc-cf-taken@example.com"

	userID := seedEmailChangeUser(t, db, currentEmail)
	// 確認が対象とするアドレスを別アカウントが既に保持しているため、適用は
	// users.emailのUNIQUE制約に当たる。
	seedEmailChangeUser(t, db, takenEmail)

	if _, err := repo.CreateEmailChange(ctx, repository.CreateEmailChangeInput{UserID: userID, Email: takenEmail, Code: "123456"}); err != nil {
		t.Fatalf("メール変更確認の作成に失敗: %v", err)
	}

	out, err := uc.Execute(ctx, usecase.VerifyEmailChangeInput{UserID: userID, Code: "123456"})
	if out != nil {
		t.Errorf("Execute()のoutput = %v、期待値 = nil", out)
	}
	ve := model.AsValidationError(err)
	if ve == nil {
		t.Fatalf("Execute()のエラー = %v、期待値 = *model.ValidationError (一意制約の競合)", err)
	}
	if !ve.HasGlobalError() {
		t.Errorf("フォーム全体のエラーが無い: %+v", ve.Global)
	}

	// emailは変更されていてはならない。
	user, err := userRepo.FindByID(ctx, userID)
	if err != nil {
		t.Fatalf("FindByID()のエラー = %v", err)
	}
	if user == nil || user.Email != currentEmail {
		t.Errorf("user.Email = %v、期待値 = %q (競合時に変更されてはならない)", user, currentEmail)
	}

	// 打刻は失敗した更新とともにロールバックされるため、確認はactiveのまま残る。
	active, err := repo.FindActiveEmailChangeByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("FindActiveEmailChangeByUserID()のエラー = %v", err)
	}
	if active == nil {
		t.Error("競合時は打刻をロールバックしactiveのまま残るはず")
	}

	// 変更はコミットされていないため、通知は投入されない。
	if inserter.Called {
		t.Error("変更が成立していないのに通知が投入された")
	}
}
