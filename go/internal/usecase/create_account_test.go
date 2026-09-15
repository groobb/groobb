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

// newCreateAccountUsecaseはテスト専用のデータベース上にUseCaseを組み立てます。
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

// seedSucceededConfirmationはサインアップ確認を作成し成功済みとして打刻
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

// TestCreateAccountUsecase_Execute_Successは、検証済みの確認と有効なパスワードが、
// ユーザー (確認のemailとリクエストのロケールを持つ) と対応するパスワード資格情報を
// 作成することを検証します。
func TestCreateAccountUsecase_Execute_Success(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	uc, ecRepo, userRepo, userPasswordRepo := newCreateAccountUsecase(t, db)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

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
		t.Fatalf("Execute()のエラー = %v", err)
	}
	if out == nil || out.User == nil {
		t.Fatal("Execute()のoutput / User = nil")
	}
	if out.User.Email != email {
		t.Errorf("out.User.Email = %q、期待値 = %q", out.User.Email, email)
	}
	if out.User.Atname != atname {
		t.Errorf("out.User.Atname = %q、期待値 = %q", out.User.Atname, atname)
	}
	if out.User.Locale != "ja" {
		t.Errorf("out.User.Locale = %q、期待値 = %q", out.User.Locale, "ja")
	}

	// ユーザー (送信されたatnameを持つ) と対応するパスワード資格情報が永続化
	// されている。
	user, err := userRepo.FindByEmail(ctx, email)
	if err != nil {
		t.Fatalf("FindByEmail()のエラー = %v", err)
	}
	if user == nil {
		t.Fatal("作成したユーザーをemailで引けない")
	}
	if user.Atname != atname {
		t.Errorf("永続化されたuser.Atname = %q、期待値 = %q", user.Atname, atname)
	}
	password, err := userPasswordRepo.FindByUserID(ctx, user.ID)
	if err != nil {
		t.Fatalf("FindByUserID()のエラー = %v", err)
	}
	if password == nil {
		t.Fatal("作成したユーザーのパスワード資格情報を引けない")
	}
	if err := auth.CheckPassword(password.PasswordDigest, "password123"); err != nil {
		t.Errorf("保存されたダイジェストが元のパスワードで検証できない: %v", err)
	}
}

// TestCreateAccountUsecase_Execute_NoSucceededConfirmationは、検証済みの確認が
// 無い (確認は存在するが未成功) とき、ExecuteがAppErrorを返しユーザーを作成しない
// ことを検証します。
func TestCreateAccountUsecase_Execute_NoSucceededConfirmation(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	uc, ecRepo, userRepo, _ := newCreateAccountUsecase(t, db)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

	email := "acct-unverified@example.com"
	// 確認を作成するが成功済みとして打刻しない。
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
		t.Errorf("Execute()のoutput = %v、期待値 = nil", out)
	}
	ae := model.AsAppError(err)
	if ae == nil {
		t.Fatalf("Execute()のエラー = %v、期待値 = *model.AppError", err)
	}
	if ae.Code != model.AppErrCodeResourceNotFound {
		t.Errorf("ae.Code = %d、期待値 = %d (AppErrCodeResourceNotFound)", ae.Code, model.AppErrCodeResourceNotFound)
	}

	user, err := userRepo.FindByEmail(ctx, email)
	if err != nil {
		t.Fatalf("FindByEmail()のエラー = %v", err)
	}
	if user != nil {
		t.Error("検証済みの確認が無い場合はユーザーを作成すべきでない")
	}
}

// TestCreateAccountUsecase_Execute_InvalidPasswordは、確認が検証済みでも、
// バリデーションに失敗するパスワード (ここでは確認の不一致) がValidationErrorを返し
// ユーザーを作成しないことを検証します。
func TestCreateAccountUsecase_Execute_InvalidPassword(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	uc, ecRepo, userRepo, _ := newCreateAccountUsecase(t, db)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

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
		t.Errorf("Execute()のoutput = %v、期待値 = nil", out)
	}
	if ve := model.AsValidationError(err); ve == nil {
		t.Fatalf("Execute()のエラー = %v、期待値 = *model.ValidationError", err)
	}

	user, err := userRepo.FindByEmail(ctx, email)
	if err != nil {
		t.Fatalf("FindByEmail()のエラー = %v", err)
	}
	if user != nil {
		t.Error("パスワードが不正な場合はユーザーを作成すべきでない")
	}
}

// TestCreateAccountUsecase_Execute_EmptyAtnameは、確認が検証済みでパスワードが
// 有効でも、空のatname (バリデーション失敗) がatnameフィールドのValidationErrorを
// 返しユーザーを作成しないことを検証する。
func TestCreateAccountUsecase_Execute_EmptyAtname(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	uc, ecRepo, userRepo, _ := newCreateAccountUsecase(t, db)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

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
		t.Errorf("Execute()のoutput = %v、期待値 = nil", out)
	}
	ve := model.AsValidationError(err)
	if ve == nil {
		t.Fatalf("Execute()のエラー = %v、期待値 = *model.ValidationError", err)
	}
	if !ve.HasFieldError("atname") {
		t.Errorf("atnameフィールドのエラーが無い: %+v", ve.Fields)
	}

	user, err := userRepo.FindByEmail(ctx, email)
	if err != nil {
		t.Fatalf("FindByEmail()のエラー = %v", err)
	}
	if user != nil {
		t.Error("atnameが空の場合はユーザーを作成すべきでない")
	}
}
