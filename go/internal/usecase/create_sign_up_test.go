package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/groobb/groobb/go/internal/dispatcher"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/query"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/validator"
)

// newCreateSignUpUsecase wires the usecase with transaction-bound repositories
// and a fake job inserter, returning the usecase together with the inserter so a
// test can assert what was enqueued.
//
// [Ja] newCreateSignUpUsecase はトランザクション束縛のリポジトリとフェイクのジョブ
// インサーターで UseCase を組み立て、何が投入されたかをテストが検証できるよう
// インサーターと一緒に返します。
func newCreateSignUpUsecase(t *testing.T) (*usecase.CreateSignUpUsecase, *testutil.FakeJobInserter, *repository.EmailConfirmationRepository) {
	t.Helper()

	db, tx := testutil.SetupTx(t)
	queries := query.New(db)
	userRepo := repository.NewUserRepository(queries).WithTx(tx)
	emailConfirmationRepo := repository.NewEmailConfirmationRepository(queries).WithTx(tx)

	inserter := &testutil.FakeJobInserter{}
	uc := usecase.NewCreateSignUpUsecase(
		validator.NewSignUpCreateValidator(userRepo),
		emailConfirmationRepo,
		dispatcher.NewDispatcher(inserter),
	)
	return uc, inserter, emailConfirmationRepo
}

// TestCreateSignUpUsecase_Execute_Success verifies that a valid email creates a
// sign-up confirmation (persisted with a code) and enqueues the confirmation
// mail carrying the same code and locale.
//
// [Ja] TestCreateSignUpUsecase_Execute_Success は、有効なメールがサインアップ確認を
// 作成し (コード付きで永続化)、同じコードとロケールを載せた確認メールを投入することを
// 検証します。
func TestCreateSignUpUsecase_Execute_Success(t *testing.T) {
	t.Parallel()

	uc, inserter, _ := newCreateSignUpUsecase(t)
	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)

	output, err := uc.Execute(ctx, usecase.CreateSignUpInput{
		Email:  "new@example.com",
		Locale: "ja",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	confirmation := output.EmailConfirmation
	if confirmation == nil {
		t.Fatal("Execute() output.EmailConfirmation = nil")
	}
	if confirmation.ID == (model.EmailConfirmationID{}) {
		t.Error("作成された確認の ID が空 (永続化されていない可能性)")
	}
	if confirmation.Email != "new@example.com" {
		t.Errorf("confirmation.Email = %q, want %q", confirmation.Email, "new@example.com")
	}
	if confirmation.Event != model.EmailConfirmationEventSignUp {
		t.Errorf("confirmation.Event = %q, want %q", confirmation.Event, model.EmailConfirmationEventSignUp)
	}
	if confirmation.Code == "" {
		t.Error("confirmation.Code が空")
	}

	if !inserter.Called {
		t.Fatal("確認メールのジョブが投入されていない")
	}
	args, ok := inserter.Args.(dispatcher.SendEmailConfirmationArgs)
	if !ok {
		t.Fatalf("投入ジョブの引数型 = %T, want dispatcher.SendEmailConfirmationArgs", inserter.Args)
	}
	if args.Email != "new@example.com" {
		t.Errorf("args.Email = %q, want %q", args.Email, "new@example.com")
	}
	if args.Code != confirmation.Code {
		t.Errorf("args.Code = %q, want %q (永続化したコードと一致すべき)", args.Code, confirmation.Code)
	}
	if args.Locale != "ja" {
		t.Errorf("args.Locale = %q, want %q", args.Locale, "ja")
	}
}

// TestCreateSignUpUsecase_Execute_DuplicateEmail verifies that a request for an
// already-registered email fails validation and enqueues no mail.
//
// [Ja] TestCreateSignUpUsecase_Execute_DuplicateEmail は、既に登録済みのメールの申請が
// バリデーションで失敗し、メールを投入しないことを検証します。
func TestCreateSignUpUsecase_Execute_DuplicateEmail(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	queries := query.New(db)
	userRepo := repository.NewUserRepository(queries).WithTx(tx)
	emailConfirmationRepo := repository.NewEmailConfirmationRepository(queries).WithTx(tx)
	inserter := &testutil.FakeJobInserter{}
	uc := usecase.NewCreateSignUpUsecase(
		validator.NewSignUpCreateValidator(userRepo),
		emailConfirmationRepo,
		dispatcher.NewDispatcher(inserter),
	)

	testutil.NewUserBuilder(t, tx).WithEmail("taken@example.com").Build()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	output, err := uc.Execute(ctx, usecase.CreateSignUpInput{
		Email:  "taken@example.com",
		Locale: "ja",
	})

	if output != nil {
		t.Errorf("Execute() output = %v, want nil", output)
	}
	if ve := model.AsValidationError(err); ve == nil {
		t.Fatalf("Execute() error = %v, want *model.ValidationError", err)
	}
	if inserter.Called {
		t.Error("重複メールではジョブを投入すべきでない")
	}
}

// TestCreateSignUpUsecase_Execute_EnqueueFailure verifies that when the
// confirmation mail cannot be enqueued, Execute returns a *model.AppError
// (Internal) and no output, so the handler can keep the user on the form to
// retry instead of advancing to a code that was never sent.
//
// [Ja] TestCreateSignUpUsecase_Execute_EnqueueFailure は、確認メールを投入できないとき
// に Execute が *model.AppError (Internal) を出力なしで返すことを検証します。これにより
// ハンドラーは、送られなかったコードへ進ませる代わりにユーザーをフォームに留めて再申請
// させられます。
func TestCreateSignUpUsecase_Execute_EnqueueFailure(t *testing.T) {
	t.Parallel()

	uc, inserter, _ := newCreateSignUpUsecase(t)
	inserter.Err = errors.New("queue unavailable")
	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)

	output, err := uc.Execute(ctx, usecase.CreateSignUpInput{
		Email:  "new@example.com",
		Locale: "ja",
	})

	if output != nil {
		t.Errorf("Execute() output = %v, want nil", output)
	}
	ae := model.AsAppError(err)
	if ae == nil {
		t.Fatalf("Execute() error = %v, want *model.AppError", err)
	}
	if ae.Code != model.AppErrCodeInternal {
		t.Errorf("ae.Code = %d, want %d (AppErrCodeInternal)", ae.Code, model.AppErrCodeInternal)
	}
}
