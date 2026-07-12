package usecase_test

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/query"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/validator"
)

// TestCreateSignInUsecase_Execute_Success verifies that Execute returns the
// authenticated user when the email and password match.
//
// [Ja] TestCreateSignInUsecase_Execute_Success は、email とパスワードが一致するとき
// Execute が認証されたユーザーを返すことを検証します。
func TestCreateSignInUsecase_Execute_Success(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)

	userID := testutil.NewUserBuilder(t, tx).WithEmail("signin-uc@example.com").Build()
	testutil.NewUserPasswordBuilder(t, tx).WithUserID(userID).WithPassword("password123").Build()

	userRepo := repository.NewUserRepository(query.New(db)).WithTx(tx)
	userPasswordRepo := repository.NewUserPasswordRepository(query.New(db)).WithTx(tx)
	userTwoFactorAuthRepo := repository.NewUserTwoFactorAuthRepository(query.New(db)).WithTx(tx)
	uc := usecase.NewCreateSignInUsecase(validator.NewSignInCreateValidator(userRepo, userPasswordRepo, userTwoFactorAuthRepo))
	out, err := uc.Execute(ctx, usecase.CreateSignInInput{
		Email:    "signin-uc@example.com",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if out == nil || out.User == nil || out.User.ID != userID {
		t.Fatalf("Execute() user = %v, want id %v", out, userID)
	}
	// A user without 2FA carries no setting, so the handler signs them in directly.
	//
	// [Ja] 2FA 無しのユーザーは設定を持たないため、ハンドラーはそのままサインインさせる。
	if out.UserTwoFactorAuth != nil {
		t.Errorf("Execute() UserTwoFactorAuth = %v, want nil (2FA 未設定のため)", out.UserTwoFactorAuth)
	}
}

// TestCreateSignInUsecase_Execute_TwoFactorEnabled verifies that Execute carries
// the enabled two-factor setting alongside the user, so the handler diverts to
// the challenge instead of issuing a session.
//
// [Ja] TestCreateSignInUsecase_Execute_TwoFactorEnabled は、Execute が有効な 2 段階認証
// 設定をユーザーと併せて運び、ハンドラーがセッション発行の代わりにチャレンジへ迂回できる
// ことを検証します。
func TestCreateSignInUsecase_Execute_TwoFactorEnabled(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)

	userID := testutil.NewUserBuilder(t, tx).WithEmail("signin-uc-2fa@example.com").Build()
	testutil.NewUserPasswordBuilder(t, tx).WithUserID(userID).WithPassword("password123").Build()
	testutil.NewUserTwoFactorAuthBuilder(t, tx).WithUserID(userID).WithEnabled(true).Build()

	userRepo := repository.NewUserRepository(query.New(db)).WithTx(tx)
	userPasswordRepo := repository.NewUserPasswordRepository(query.New(db)).WithTx(tx)
	userTwoFactorAuthRepo := repository.NewUserTwoFactorAuthRepository(query.New(db)).WithTx(tx)
	uc := usecase.NewCreateSignInUsecase(validator.NewSignInCreateValidator(userRepo, userPasswordRepo, userTwoFactorAuthRepo))
	out, err := uc.Execute(ctx, usecase.CreateSignInInput{
		Email:    "signin-uc-2fa@example.com",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if out == nil || out.User == nil || out.User.ID != userID {
		t.Fatalf("Execute() user = %v, want id %v", out, userID)
	}
	if out.UserTwoFactorAuth == nil {
		t.Fatal("Execute() UserTwoFactorAuth = nil, want 有効な 2FA 設定")
	}
}

// TestCreateSignInUsecase_Execute_InvalidCredentials verifies that a wrong
// password surfaces the validator's *model.ValidationError unchanged, so the
// handler re-renders the form.
//
// [Ja] TestCreateSignInUsecase_Execute_InvalidCredentials は、誤ったパスワードが
// バリデーターの *model.ValidationError をそのまま表面化し、ハンドラーがフォームを
// 再描画できることを検証します。
func TestCreateSignInUsecase_Execute_InvalidCredentials(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)

	userID := testutil.NewUserBuilder(t, tx).WithEmail("signin-uc-bad@example.com").Build()
	testutil.NewUserPasswordBuilder(t, tx).WithUserID(userID).WithPassword("password123").Build()

	userRepo := repository.NewUserRepository(query.New(db)).WithTx(tx)
	userPasswordRepo := repository.NewUserPasswordRepository(query.New(db)).WithTx(tx)
	userTwoFactorAuthRepo := repository.NewUserTwoFactorAuthRepository(query.New(db)).WithTx(tx)
	uc := usecase.NewCreateSignInUsecase(validator.NewSignInCreateValidator(userRepo, userPasswordRepo, userTwoFactorAuthRepo))
	out, err := uc.Execute(ctx, usecase.CreateSignInInput{
		Email:    "signin-uc-bad@example.com",
		Password: "wrongpassword",
	})
	if out != nil {
		t.Errorf("Execute() output = %v, want nil", out)
	}
	if ve := model.AsValidationError(err); ve == nil || !ve.HasGlobalError() {
		t.Fatalf("Execute() error = %v, want *model.ValidationError with a global error", err)
	}
}
