package usecase_test

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/auth"
	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/validator"
)

// newCreateAccountUsecase wires the usecase over the test's own database. It
// returns the repositories so a test can seed a confirmation and assert the
// created user and password.
//
// [Ja] newCreateAccountUsecase はテスト専用のデータベース上に UseCase を組み立てます。
// テストが確認を仕込み、作成されたユーザーとパスワードを検証できるようリポジトリも返し
// ます。
func newCreateAccountUsecase(t *testing.T, db *database.DB) (*usecase.CreateAccountUsecase, *repository.EmailConfirmationRepository, *repository.UserRepository, *repository.UserPasswordRepository) {
	t.Helper()

	emailConfirmationRepo := repository.NewEmailConfirmationRepository(db)
	userRepo := repository.NewUserRepository(db)
	userPasswordRepo := repository.NewUserPasswordRepository(db)

	uc := usecase.NewCreateAccountUsecase(
		db.Writer,
		validator.NewAccountCreateValidator(userRepo),
		emailConfirmationRepo,
		userRepo,
		userPasswordRepo,
	)
	return uc, emailConfirmationRepo, userRepo, userPasswordRepo
}

// seedSucceededConfirmation creates and stamps a sign-up confirmation as
// succeeded (committed), returning it so a test can drive account creation from
// a verified confirmation.
//
// [Ja] seedSucceededConfirmation はサインアップ確認を作成し成功済みとして打刻
// (コミット) し、テストが検証済みの確認からアカウント作成を駆動できるよう返します。
func seedSucceededConfirmation(t *testing.T, ctx context.Context, repo *repository.EmailConfirmationRepository, email string) *model.EmailConfirmation {
	t.Helper()

	confirmation, err := repo.Create(ctx, repository.CreateEmailConfirmationInput{
		Email: email,
		Event: model.EmailConfirmationEventSignUp,
		Code:  "123456",
	})
	if err != nil {
		t.Fatalf("確認の作成に失敗: %v", err)
	}
	if err := repo.Succeed(ctx, confirmation.ID); err != nil {
		t.Fatalf("確認の成功打刻に失敗: %v", err)
	}
	return confirmation
}

// TestCreateAccountUsecase_Execute_Success verifies that a verified confirmation
// plus a valid password creates the user (with the confirmation's email and the
// request locale) and a matching password credential.
//
// [Ja] TestCreateAccountUsecase_Execute_Success は、検証済みの確認と有効なパスワードが、
// ユーザー (確認の email とリクエストのロケールを持つ) と対応するパスワード資格情報を
// 作成することを検証します。
func TestCreateAccountUsecase_Execute_Success(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	uc, ecRepo, userRepo, userPasswordRepo := newCreateAccountUsecase(t, db)
	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)

	email := "acct-success@example.com"
	atname := testutil.UniqueAtname(db)
	confirmation := seedSucceededConfirmation(t, ctx, ecRepo, email)

	out, err := uc.Execute(ctx, usecase.CreateAccountInput{
		EmailConfirmationID:  confirmation.ID,
		Atname:               atname,
		Password:             "password123",
		PasswordConfirmation: "password123",
		Locale:               "ja",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if out == nil || out.User == nil {
		t.Fatal("Execute() output / User = nil")
	}
	if out.User.Email != email {
		t.Errorf("out.User.Email = %q, want %q", out.User.Email, email)
	}
	if out.User.Atname != atname {
		t.Errorf("out.User.Atname = %q, want %q", out.User.Atname, atname)
	}
	if out.User.Locale != "ja" {
		t.Errorf("out.User.Locale = %q, want %q", out.User.Locale, "ja")
	}

	// The user (with the submitted atname) and a matching password credential are
	// persisted.
	//
	// [Ja] ユーザー (送信された atname を持つ) と対応するパスワード資格情報が永続化
	// されている。
	user, err := userRepo.FindByEmail(ctx, email)
	if err != nil {
		t.Fatalf("FindByEmail() error = %v", err)
	}
	if user == nil {
		t.Fatal("作成したユーザーを email で引けない")
	}
	if user.Atname != atname {
		t.Errorf("永続化された user.Atname = %q, want %q", user.Atname, atname)
	}
	password, err := userPasswordRepo.FindByUserID(ctx, user.ID)
	if err != nil {
		t.Fatalf("FindByUserID() error = %v", err)
	}
	if password == nil {
		t.Fatal("作成したユーザーのパスワード資格情報を引けない")
	}
	if err := auth.CheckPassword(password.PasswordDigest, "password123"); err != nil {
		t.Errorf("保存されたダイジェストが元のパスワードで検証できない: %v", err)
	}
}

// TestCreateAccountUsecase_Execute_NoSucceededConfirmation verifies that without
// a verified confirmation (the confirmation exists but was never succeeded),
// Execute returns an AppError and creates no user.
//
// [Ja] TestCreateAccountUsecase_Execute_NoSucceededConfirmation は、検証済みの確認が
// 無い (確認は存在するが未成功) とき、Execute が AppError を返しユーザーを作成しない
// ことを検証します。
func TestCreateAccountUsecase_Execute_NoSucceededConfirmation(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	uc, ecRepo, userRepo, _ := newCreateAccountUsecase(t, db)
	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)

	email := "acct-unverified@example.com"
	// Create the confirmation but do not stamp it succeeded.
	//
	// [Ja] 確認を作成するが成功済みとして打刻しない。
	confirmation, err := ecRepo.Create(ctx, repository.CreateEmailConfirmationInput{
		Email: email,
		Event: model.EmailConfirmationEventSignUp,
		Code:  "123456",
	})
	if err != nil {
		t.Fatalf("確認の作成に失敗: %v", err)
	}

	out, err := uc.Execute(ctx, usecase.CreateAccountInput{
		EmailConfirmationID:  confirmation.ID,
		Password:             "password123",
		PasswordConfirmation: "password123",
		Locale:               "ja",
	})
	if out != nil {
		t.Errorf("Execute() output = %v, want nil", out)
	}
	ae := model.AsAppError(err)
	if ae == nil {
		t.Fatalf("Execute() error = %v, want *model.AppError", err)
	}
	if ae.Code != model.AppErrCodeResourceNotFound {
		t.Errorf("ae.Code = %d, want %d (AppErrCodeResourceNotFound)", ae.Code, model.AppErrCodeResourceNotFound)
	}

	user, err := userRepo.FindByEmail(ctx, email)
	if err != nil {
		t.Fatalf("FindByEmail() error = %v", err)
	}
	if user != nil {
		t.Error("検証済みの確認が無い場合はユーザーを作成すべきでない")
	}
}

// TestCreateAccountUsecase_Execute_InvalidPassword verifies that a password that
// fails validation (here, a mismatched confirmation) returns a ValidationError
// and creates no user, even though the confirmation is verified.
//
// [Ja] TestCreateAccountUsecase_Execute_InvalidPassword は、確認が検証済みでも、
// バリデーションに失敗するパスワード (ここでは確認の不一致) が ValidationError を返し
// ユーザーを作成しないことを検証します。
func TestCreateAccountUsecase_Execute_InvalidPassword(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	uc, ecRepo, userRepo, _ := newCreateAccountUsecase(t, db)
	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)

	email := "acct-badpw@example.com"
	confirmation := seedSucceededConfirmation(t, ctx, ecRepo, email)

	out, err := uc.Execute(ctx, usecase.CreateAccountInput{
		EmailConfirmationID:  confirmation.ID,
		Atname:               testutil.UniqueAtname(db),
		Password:             "password123",
		PasswordConfirmation: "different456",
		Locale:               "ja",
	})
	if out != nil {
		t.Errorf("Execute() output = %v, want nil", out)
	}
	if ve := model.AsValidationError(err); ve == nil {
		t.Fatalf("Execute() error = %v, want *model.ValidationError", err)
	}

	user, err := userRepo.FindByEmail(ctx, email)
	if err != nil {
		t.Fatalf("FindByEmail() error = %v", err)
	}
	if user != nil {
		t.Error("パスワードが不正な場合はユーザーを作成すべきでない")
	}
}

// TestCreateAccountUsecase_Execute_EmptyAtname verifies that an empty atname (a
// validation failure) returns a ValidationError on the atname field and creates
// no user, even though the confirmation is verified and the password is valid.
//
// [Ja] TestCreateAccountUsecase_Execute_EmptyAtname は、確認が検証済みでパスワードが
// 有効でも、空の atname (バリデーション失敗) が atname フィールドの ValidationError を
// 返しユーザーを作成しないことを検証する。
func TestCreateAccountUsecase_Execute_EmptyAtname(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	uc, ecRepo, userRepo, _ := newCreateAccountUsecase(t, db)
	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)

	email := "acct-noatname@example.com"
	confirmation := seedSucceededConfirmation(t, ctx, ecRepo, email)

	out, err := uc.Execute(ctx, usecase.CreateAccountInput{
		EmailConfirmationID:  confirmation.ID,
		Atname:               "",
		Password:             "password123",
		PasswordConfirmation: "password123",
		Locale:               "ja",
	})
	if out != nil {
		t.Errorf("Execute() output = %v, want nil", out)
	}
	ve := model.AsValidationError(err)
	if ve == nil {
		t.Fatalf("Execute() error = %v, want *model.ValidationError", err)
	}
	if !ve.HasFieldError("atname") {
		t.Errorf("atname フィールドのエラーが無い: %+v", ve.Fields)
	}

	user, err := userRepo.FindByEmail(ctx, email)
	if err != nil {
		t.Fatalf("FindByEmail() error = %v", err)
	}
	if user != nil {
		t.Error("atname が空の場合はユーザーを作成すべきでない")
	}
}
