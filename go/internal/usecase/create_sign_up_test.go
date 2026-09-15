package usecase_test

import (
	"context"
	"errors"
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

// newCreateSignUpUsecaseはテストのデータベース上に、フェイクのジョブ
// インサーターを伴ってUseCaseを組み立て、何が投入されたかをテストが検証できるよう
// インサーターと一緒に返します。
func newCreateSignUpUsecase(t *testing.T, db *database.DB) (*usecase.CreateSignUpUsecase, *testutil.FakeJobInserter, *repository.EmailConfirmationRepository) {
	t.Helper()

	userRepo := repository.NewUserRepository(db)
	emailConfirmationRepo := repository.NewEmailConfirmationRepository(db)

	inserter := &testutil.FakeJobInserter{}
	uc := usecase.NewCreateSignUpUsecase(
		validator.NewSignUpCreateValidator(userRepo),
		emailConfirmationRepo,
		dispatcher.NewDispatcher(inserter),
	)
	return uc, inserter, emailConfirmationRepo
}

// TestCreateSignUpUsecase_Execute_Successは、有効なメールがサインアップ確認を
// 作成し (コード付きで永続化)、同じコードとロケールを載せた確認メールを投入することを
// 検証します。
func TestCreateSignUpUsecase_Execute_Success(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	uc, inserter, _ := newCreateSignUpUsecase(t, db)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

	output, err := uc.Execute(ctx, usecase.CreateSignUpInput{
		Email:  "new@example.com",
		Locale: "ja",
	})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}

	confirmation := output.EmailConfirmation
	if confirmation == nil {
		t.Fatal("Execute()のoutput.EmailConfirmation = nil")
	}
	if confirmation.ID == 0 {
		t.Error("作成された確認のIDが空 (永続化されていない可能性)")
	}
	if confirmation.Email != "new@example.com" {
		t.Errorf("confirmation.Email = %q、期待値 = %q", confirmation.Email, "new@example.com")
	}
	if confirmation.Event != model.EmailConfirmationEventSignUp {
		t.Errorf("confirmation.Event = %q、期待値 = %q", confirmation.Event, model.EmailConfirmationEventSignUp)
	}
	if confirmation.Code == "" {
		t.Error("confirmation.Codeが空")
	}

	if !inserter.Called {
		t.Fatal("確認メールのジョブが投入されていない")
	}
	args, ok := inserter.Args.(dispatcher.SendEmailConfirmationArgs)
	if !ok {
		t.Fatalf("投入ジョブの引数型 = %T、期待値 = dispatcher.SendEmailConfirmationArgs", inserter.Args)
	}
	if args.Email != "new@example.com" {
		t.Errorf("args.Email = %q、期待値 = %q", args.Email, "new@example.com")
	}
	if args.Code != confirmation.Code {
		t.Errorf("args.Code = %q、期待値 = %q (永続化したコードと一致すべき)", args.Code, confirmation.Code)
	}
	if args.Locale != "ja" {
		t.Errorf("args.Locale = %q、期待値 = %q", args.Locale, "ja")
	}
}

// TestCreateSignUpUsecase_Execute_DuplicateEmailは、既に登録済みのメールの申請が
// バリデーションで失敗し、メールを投入しないことを検証します。
func TestCreateSignUpUsecase_Execute_DuplicateEmail(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	userRepo := repository.NewUserRepository(db)
	emailConfirmationRepo := repository.NewEmailConfirmationRepository(db)
	inserter := &testutil.FakeJobInserter{}
	uc := usecase.NewCreateSignUpUsecase(
		validator.NewSignUpCreateValidator(userRepo),
		emailConfirmationRepo,
		dispatcher.NewDispatcher(inserter),
	)

	testutil.NewUserBuilder(t, db).WithEmail("taken@example.com").Build()

	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)
	output, err := uc.Execute(ctx, usecase.CreateSignUpInput{
		Email:  "taken@example.com",
		Locale: "ja",
	})

	if output != nil {
		t.Errorf("Execute()のoutput = %v、期待値 = nil", output)
	}
	if ve := model.AsValidationError(err); ve == nil {
		t.Fatalf("Execute()のエラー = %v、期待値 = *model.ValidationError", err)
	}
	if inserter.Called {
		t.Error("重複メールではジョブを投入すべきでない")
	}
}

// TestCreateSignUpUsecase_Execute_EnqueueFailureは、確認メールを投入できないとき
// にExecuteが *model.AppError (Internal) を出力なしで返すことを検証します。これにより
// ハンドラーは、送られなかったコードへ進ませる代わりにユーザーをフォームに留めて再申請
// させられます。
func TestCreateSignUpUsecase_Execute_EnqueueFailure(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	uc, inserter, _ := newCreateSignUpUsecase(t, db)
	inserter.Err = errors.New("queue unavailable")
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

	output, err := uc.Execute(ctx, usecase.CreateSignUpInput{
		Email:  "new@example.com",
		Locale: "ja",
	})

	if output != nil {
		t.Errorf("Execute()のoutput = %v、期待値 = nil", output)
	}
	ae := model.AsAppError(err)
	if ae == nil {
		t.Fatalf("Execute()のエラー = %v、期待値 = *model.AppError", err)
	}
	if ae.Code != model.AppErrCodeInternal {
		t.Errorf("ae.Code = %d、期待値 = %d (AppErrCodeInternal)", ae.Code, model.AppErrCodeInternal)
	}
}
