package usecase_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"

	"github.com/groobb/groobb/go/internal/auth"
	"github.com/groobb/groobb/go/internal/dispatcher"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/query"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/validator"
)

// newCreateEmailChangeUsecase wires a CreateEmailChangeUsecase over the shared
// pool with a fake job inserter, returning the inserter too so a test can assert
// what was enqueued or force an enqueue failure. The UseCase opens its own
// transaction, so its tests commit rows and use unique emails (the test database
// is reset by make test) rather than the rolled-back transaction pattern.
//
// [Ja] newCreateEmailChangeUsecase は共有プール上に CreateEmailChangeUsecase を
// フェイクのジョブインサーターで組み立て、投入内容を検証したり enqueue 失敗を強制したり
// できるようインサーターも返します。UseCase は自前のトランザクションを開くため、そのテストは
// ロールバックされるトランザクションパターンではなく、行をコミットしユニークな email を
// 使います (テスト DB は make test がリセットする)。
func newCreateEmailChangeUsecase(t *testing.T) (*usecase.CreateEmailChangeUsecase, *testutil.FakeJobInserter) {
	t.Helper()

	db := testutil.GetTestDB()
	queries := query.New(db)
	userRepo := repository.NewUserRepository(queries)
	userPasswordRepo := repository.NewUserPasswordRepository(queries)
	emailConfirmationRepo := repository.NewEmailConfirmationRepository(queries)

	inserter := &testutil.FakeJobInserter{}
	uc := usecase.NewCreateEmailChangeUsecase(
		db,
		validator.NewSettingsEmailUpdateValidator(userRepo, userPasswordRepo),
		emailConfirmationRepo,
		dispatcher.NewDispatcher(inserter),
	)
	return uc, inserter
}

// seedEmailChangeUser creates a committed user with the given email and the
// password "password123", returning its id so a UseCase test can drive an
// email-change request from a real, authenticatable account.
//
// [Ja] seedEmailChangeUser は指定 email とパスワード "password123" を持つコミット済み
// ユーザーを作成し、その id を返す。UseCase テストが実在の認証可能なアカウントから
// メール変更申請を駆動できるようにする。
func seedEmailChangeUser(t *testing.T, email string) model.UserID {
	t.Helper()

	ctx := context.Background()
	queries := query.New(testutil.GetTestDB())
	userRepo := repository.NewUserRepository(queries)
	userPasswordRepo := repository.NewUserPasswordRepository(queries)

	user, err := userRepo.Create(ctx, repository.CreateUserInput{
		Email:    email,
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
	return user.ID
}

// TestCreateEmailChangeUsecase_Execute_Success verifies that a valid request
// issues an email-change confirmation for the new address tied to the user and
// enqueues the confirmation mail carrying the same code.
//
// [Ja] TestCreateEmailChangeUsecase_Execute_Success は、有効な申請がユーザーに紐付いた
// 新しいアドレスのメール変更確認を発行し、同じコードを運ぶ確認メールを投入することを
// 検証する。
func TestCreateEmailChangeUsecase_Execute_Success(t *testing.T) {
	t.Parallel()

	uc, inserter := newCreateEmailChangeUsecase(t)
	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	userID := seedEmailChangeUser(t, fmt.Sprintf("ec-uc-cur-%s@example.com", uuid.NewString()))
	newEmail := fmt.Sprintf("ec-uc-new-%s@example.com", uuid.NewString())

	output, err := uc.Execute(ctx, usecase.CreateEmailChangeInput{
		UserID:          userID,
		NewEmail:        newEmail,
		CurrentPassword: "password123",
		Locale:          i18n.LangJa,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if output == nil || output.EmailConfirmation == nil {
		t.Fatal("Execute() は作成した確認を返すべき")
	}
	if output.EmailConfirmation.Email != newEmail {
		t.Errorf("confirmation.Email = %q, want %q", output.EmailConfirmation.Email, newEmail)
	}
	if output.EmailConfirmation.Event != model.EmailConfirmationEventEmailChange {
		t.Errorf("confirmation.Event = %q, want %q", output.EmailConfirmation.Event, model.EmailConfirmationEventEmailChange)
	}
	if output.EmailConfirmation.UserID == nil || *output.EmailConfirmation.UserID != userID {
		t.Errorf("confirmation.UserID = %v, want %v", output.EmailConfirmation.UserID, userID)
	}

	if !inserter.Called {
		t.Fatal("確認メールが投入されていない")
	}
	args, ok := inserter.Args.(dispatcher.SendEmailConfirmationArgs)
	if !ok {
		t.Fatalf("投入された Args の型 = %T, want dispatcher.SendEmailConfirmationArgs", inserter.Args)
	}
	if args.Email != newEmail {
		t.Errorf("投入メールの宛先 = %q, want %q", args.Email, newEmail)
	}
	if args.Code != output.EmailConfirmation.Code {
		t.Errorf("投入メールの code = %q, want %q (確認と同じコード)", args.Code, output.EmailConfirmation.Code)
	}
}

// TestCreateEmailChangeUsecase_Execute_ReplacesPending verifies that issuing a
// second request replaces the first: the user is left with exactly one pending
// email-change confirmation, and it carries the latest new address.
//
// [Ja] TestCreateEmailChangeUsecase_Execute_ReplacesPending は、2 回目の申請が 1 回目を
// 置き換えることを検証する。ユーザーには保留中のメール変更確認がちょうど 1 件残り、それが
// 最新の新しいアドレスを持つ。
func TestCreateEmailChangeUsecase_Execute_ReplacesPending(t *testing.T) {
	t.Parallel()

	uc, _ := newCreateEmailChangeUsecase(t)
	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	userID := seedEmailChangeUser(t, fmt.Sprintf("ec-uc-rep-%s@example.com", uuid.NewString()))
	firstEmail := fmt.Sprintf("ec-uc-rep1-%s@example.com", uuid.NewString())
	secondEmail := fmt.Sprintf("ec-uc-rep2-%s@example.com", uuid.NewString())

	if _, err := uc.Execute(ctx, usecase.CreateEmailChangeInput{
		UserID: userID, NewEmail: firstEmail, CurrentPassword: "password123", Locale: i18n.LangJa,
	}); err != nil {
		t.Fatalf("1 回目の Execute() error = %v", err)
	}
	if _, err := uc.Execute(ctx, usecase.CreateEmailChangeInput{
		UserID: userID, NewEmail: secondEmail, CurrentPassword: "password123", Locale: i18n.LangJa,
	}); err != nil {
		t.Fatalf("2 回目の Execute() error = %v", err)
	}

	var pending int
	err := testutil.GetTestDB().QueryRow(ctx,
		`SELECT count(*) FROM email_confirmations WHERE user_id = $1 AND event = 'email_change' AND succeeded_at IS NULL`,
		uuid.UUID(userID),
	).Scan(&pending)
	if err != nil {
		t.Fatalf("保留件数の取得に失敗: %v", err)
	}
	if pending != 1 {
		t.Errorf("保留中のメール変更確認 = %d 件, want 1 件", pending)
	}

	active, err := repository.NewEmailConfirmationRepository(query.New(testutil.GetTestDB())).FindActiveEmailChangeByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("FindActiveEmailChangeByUserID() error = %v", err)
	}
	if active == nil || active.Email != secondEmail {
		t.Errorf("保留中の確認のアドレス = %v, want %q", active, secondEmail)
	}
}

// TestCreateEmailChangeUsecase_Execute_ValidationError verifies that a wrong
// current password fails with a *model.ValidationError before any confirmation is
// created or mail enqueued.
//
// [Ja] TestCreateEmailChangeUsecase_Execute_ValidationError は、誤った現在のパスワードが
// 確認の作成やメール投入の前に *model.ValidationError で失敗することを検証する。
func TestCreateEmailChangeUsecase_Execute_ValidationError(t *testing.T) {
	t.Parallel()

	uc, inserter := newCreateEmailChangeUsecase(t)
	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	userID := seedEmailChangeUser(t, fmt.Sprintf("ec-uc-ve-%s@example.com", uuid.NewString()))

	_, err := uc.Execute(ctx, usecase.CreateEmailChangeInput{
		UserID:          userID,
		NewEmail:        fmt.Sprintf("ec-uc-ve-new-%s@example.com", uuid.NewString()),
		CurrentPassword: "wrongpassword",
		Locale:          i18n.LangJa,
	})
	if ve := model.AsValidationError(err); ve == nil {
		t.Fatalf("Execute() error = %v, want *model.ValidationError", err)
	}
	if inserter.Called {
		t.Error("バリデーション失敗時に確認メールが投入された")
	}

	active, err := repository.NewEmailConfirmationRepository(query.New(testutil.GetTestDB())).FindActiveEmailChangeByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("FindActiveEmailChangeByUserID() error = %v", err)
	}
	if active != nil {
		t.Error("バリデーション失敗時にメール変更確認が作成された")
	}
}

// TestCreateEmailChangeUsecase_Execute_EnqueueFailure verifies that when the
// confirmation mail cannot be enqueued, Execute returns a *model.AppError (the
// handler's retry path).
//
// [Ja] TestCreateEmailChangeUsecase_Execute_EnqueueFailure は、確認メールを投入できない
// とき Execute が *model.AppError (ハンドラーの再申請導線) を返すことを検証する。
func TestCreateEmailChangeUsecase_Execute_EnqueueFailure(t *testing.T) {
	t.Parallel()

	uc, inserter := newCreateEmailChangeUsecase(t)
	inserter.Err = errors.New("queue unavailable")
	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	userID := seedEmailChangeUser(t, fmt.Sprintf("ec-uc-enq-%s@example.com", uuid.NewString()))

	_, err := uc.Execute(ctx, usecase.CreateEmailChangeInput{
		UserID:          userID,
		NewEmail:        fmt.Sprintf("ec-uc-enq-new-%s@example.com", uuid.NewString()),
		CurrentPassword: "password123",
		Locale:          i18n.LangJa,
	})
	if ae := model.AsAppError(err); ae == nil {
		t.Fatalf("Execute() error = %v, want *model.AppError", err)
	}
}
