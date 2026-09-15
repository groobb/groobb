package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/groobb/groobb/go/internal/auth"
	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/dispatcher"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/validator"
)

// newCreateEmailChangeUsecaseはテスト専用のデータベース上にCreateEmailChangeUsecaseを
// フェイクのジョブインサーターで組み立て、投入内容を検証したりenqueue失敗を強制したり
// できるようインサーターも返します。
func newCreateEmailChangeUsecase(t *testing.T, db *database.DB) (*usecase.CreateEmailChangeUsecase, *testutil.FakeJobInserter) {
	t.Helper()

	userRepo := repository.NewUserRepository(db)
	userPasswordRepo := repository.NewUserPasswordRepository(db)
	emailConfirmationRepo := repository.NewEmailConfirmationRepository(db)

	inserter := &testutil.FakeJobInserter{}
	uc := usecase.NewCreateEmailChangeUsecase(
		db.Writer,
		validator.NewSettingsEmailUpdateValidator(userRepo, userPasswordRepo),
		emailConfirmationRepo,
		dispatcher.NewDispatcher(inserter),
	)
	return uc, inserter
}

// seedEmailChangeUserは指定emailとパスワード "password123" を持つコミット済み
// ユーザーを作成し、そのidを返す。UseCaseテストが実在の認証可能なアカウントから
// メール変更申請を駆動できるようにする。
func seedEmailChangeUser(t *testing.T, db *database.DB, email string) model.UserID {
	t.Helper()

	ctx := context.Background()
	userRepo := repository.NewUserRepository(db)
	userPasswordRepo := repository.NewUserPasswordRepository(db)

	user, err := userRepo.Create(ctx, repository.CreateUserInput{
		Email:    email,
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
	return user.ID
}

// TestCreateEmailChangeUsecase_Execute_Successは、有効な申請がユーザーに紐付いた
// 新しいアドレスのメール変更確認を発行し、同じコードを運ぶ確認メールを投入することを
// 検証する。
func TestCreateEmailChangeUsecase_Execute_Success(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	uc, inserter := newCreateEmailChangeUsecase(t, db)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)
	userID := seedEmailChangeUser(t, db, "ec-uc-cur@example.com")
	newEmail := "ec-uc-new@example.com"

	output, err := uc.Execute(ctx, usecase.CreateEmailChangeInput{
		UserID:          userID,
		NewEmail:        newEmail,
		CurrentPassword: "password123",
		Locale:          model.LocaleJa,
	})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v、期待値 = nil", err)
	}
	if output == nil || output.EmailConfirmation == nil {
		t.Fatal("Execute() は作成した確認を返すべき")
	}
	if output.EmailConfirmation.Email != newEmail {
		t.Errorf("confirmation.Email = %q、期待値 = %q", output.EmailConfirmation.Email, newEmail)
	}
	if output.EmailConfirmation.Event != model.EmailConfirmationEventEmailChange {
		t.Errorf("confirmation.Event = %q、期待値 = %q", output.EmailConfirmation.Event, model.EmailConfirmationEventEmailChange)
	}
	if output.EmailConfirmation.UserID == nil || *output.EmailConfirmation.UserID != userID {
		t.Errorf("confirmation.UserID = %v、期待値 = %v", output.EmailConfirmation.UserID, userID)
	}

	if !inserter.Called {
		t.Fatal("確認メールが投入されていない")
	}
	args, ok := inserter.Args.(dispatcher.SendEmailConfirmationArgs)
	if !ok {
		t.Fatalf("投入されたArgsの型 = %T、期待値 = dispatcher.SendEmailConfirmationArgs", inserter.Args)
	}
	if args.Email != newEmail {
		t.Errorf("投入メールの宛先 = %q、期待値 = %q", args.Email, newEmail)
	}
	if args.Code != output.EmailConfirmation.Code {
		t.Errorf("投入メールのcode = %q、期待値 = %q (確認と同じコード)", args.Code, output.EmailConfirmation.Code)
	}
}

// TestCreateEmailChangeUsecase_Execute_ReplacesPendingは、2回目の申請が1回目を
// 置き換えることを検証する。ユーザーには保留中のメール変更確認がちょうど1件残り、それが
// 最新の新しいアドレスを持つ。
func TestCreateEmailChangeUsecase_Execute_ReplacesPending(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	uc, _ := newCreateEmailChangeUsecase(t, db)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)
	userID := seedEmailChangeUser(t, db, "ec-uc-rep@example.com")
	firstEmail := "ec-uc-rep1@example.com"
	secondEmail := "ec-uc-rep2@example.com"

	if _, err := uc.Execute(ctx, usecase.CreateEmailChangeInput{
		UserID: userID, NewEmail: firstEmail, CurrentPassword: "password123", Locale: model.LocaleJa,
	}); err != nil {
		t.Fatalf("1回目のExecute()のエラー = %v", err)
	}
	if _, err := uc.Execute(ctx, usecase.CreateEmailChangeInput{
		UserID: userID, NewEmail: secondEmail, CurrentPassword: "password123", Locale: model.LocaleJa,
	}); err != nil {
		t.Fatalf("2回目のExecute()のエラー = %v", err)
	}

	var pending int
	err := db.Reader.QueryRowContext(ctx,
		`SELECT count(*) FROM email_confirmations WHERE user_id = ? AND event = 'email_change' AND succeeded_at IS NULL`,
		int64(userID),
	).Scan(&pending)
	if err != nil {
		t.Fatalf("保留件数の取得に失敗: %v", err)
	}
	if pending != 1 {
		t.Errorf("保留中のメール変更確認 = %d 件、期待値 = 1件", pending)
	}

	active, err := repository.NewEmailConfirmationRepository(db).FindActiveEmailChangeByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("FindActiveEmailChangeByUserID()のエラー = %v", err)
	}
	if active == nil || active.Email != secondEmail {
		t.Errorf("保留中の確認のアドレス = %v、期待値 = %q", active, secondEmail)
	}
}

// TestCreateEmailChangeUsecase_Execute_ValidationErrorは、誤った現在のパスワードが
// 確認の作成やメール投入の前に *model.ValidationErrorで失敗することを検証する。
func TestCreateEmailChangeUsecase_Execute_ValidationError(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	uc, inserter := newCreateEmailChangeUsecase(t, db)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)
	userID := seedEmailChangeUser(t, db, "ec-uc-ve@example.com")

	_, err := uc.Execute(ctx, usecase.CreateEmailChangeInput{
		UserID:          userID,
		NewEmail:        "ec-uc-ve-new@example.com",
		CurrentPassword: "wrongpassword",
		Locale:          model.LocaleJa,
	})
	if ve := model.AsValidationError(err); ve == nil {
		t.Fatalf("Execute()のエラー = %v、期待値 = *model.ValidationError", err)
	}
	if inserter.Called {
		t.Error("バリデーション失敗時に確認メールが投入された")
	}

	active, err := repository.NewEmailConfirmationRepository(db).FindActiveEmailChangeByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("FindActiveEmailChangeByUserID()のエラー = %v", err)
	}
	if active != nil {
		t.Error("バリデーション失敗時にメール変更確認が作成された")
	}
}

// TestCreateEmailChangeUsecase_Execute_EnqueueFailureは、確認メールを投入できない
// ときExecuteが *model.AppError (ハンドラーの再申請導線) を返すことを検証する。
func TestCreateEmailChangeUsecase_Execute_EnqueueFailure(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	uc, inserter := newCreateEmailChangeUsecase(t, db)
	inserter.Err = errors.New("queue unavailable")
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)
	userID := seedEmailChangeUser(t, db, "ec-uc-enq@example.com")

	_, err := uc.Execute(ctx, usecase.CreateEmailChangeInput{
		UserID:          userID,
		NewEmail:        "ec-uc-enq-new@example.com",
		CurrentPassword: "password123",
		Locale:          model.LocaleJa,
	})
	if ae := model.AsAppError(err); ae == nil {
		t.Fatalf("Execute()のエラー = %v、期待値 = *model.AppError", err)
	}
}
