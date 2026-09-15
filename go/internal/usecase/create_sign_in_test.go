package usecase_test

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/validator"
)

// TestCreateSignInUsecase_Execute_Successは、emailとパスワードが一致するとき
// Executeが認証されたユーザーを返すことを検証します。
func TestCreateSignInUsecase_Execute_Success(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

	userID := testutil.NewUserBuilder(t, db).WithEmail("signin-uc@example.com").Build()
	testutil.NewUserPasswordBuilder(t, db).WithUserID(userID).WithPassword("password123").Build()

	userRepo := repository.NewUserRepository(db)
	userPasswordRepo := repository.NewUserPasswordRepository(db)
	userTwoFactorAuthRepo := repository.NewUserTwoFactorAuthRepository(db)
	uc := usecase.NewCreateSignInUsecase(validator.NewSignInCreateValidator(userRepo, userPasswordRepo, userTwoFactorAuthRepo))
	out, err := uc.Execute(ctx, usecase.CreateSignInInput{
		Email:    "signin-uc@example.com",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	if out == nil || out.User == nil || out.User.ID != userID {
		t.Fatalf("Execute()のoutput = %v、期待値 = IDが %v のUserを持つoutput", out, userID)
	}
	// 2FA無しのユーザーは設定を持たないため、ハンドラーはそのままサインインさせる。
	if out.UserTwoFactorAuth != nil {
		t.Errorf("Execute()のUserTwoFactorAuth = %v、期待値 = nil (2FA未設定のため)", out.UserTwoFactorAuth)
	}
}

// TestCreateSignInUsecase_Execute_TwoFactorEnabledは、Executeが有効な2段階認証
// 設定をユーザーと併せて運び、ハンドラーがセッション発行の代わりにチャレンジへ迂回できる
// ことを検証します。
func TestCreateSignInUsecase_Execute_TwoFactorEnabled(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

	userID := testutil.NewUserBuilder(t, db).WithEmail("signin-uc-2fa@example.com").Build()
	testutil.NewUserPasswordBuilder(t, db).WithUserID(userID).WithPassword("password123").Build()
	testutil.NewUserTwoFactorAuthBuilder(t, db).WithUserID(userID).WithEnabled(true).Build()

	userRepo := repository.NewUserRepository(db)
	userPasswordRepo := repository.NewUserPasswordRepository(db)
	userTwoFactorAuthRepo := repository.NewUserTwoFactorAuthRepository(db)
	uc := usecase.NewCreateSignInUsecase(validator.NewSignInCreateValidator(userRepo, userPasswordRepo, userTwoFactorAuthRepo))
	out, err := uc.Execute(ctx, usecase.CreateSignInInput{
		Email:    "signin-uc-2fa@example.com",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	if out == nil || out.User == nil || out.User.ID != userID {
		t.Fatalf("Execute()のoutput = %v、期待値 = IDが %v のUserを持つoutput", out, userID)
	}
	if out.UserTwoFactorAuth == nil {
		t.Fatal("Execute()のUserTwoFactorAuth = nil、期待値 = 有効な2FA設定")
	}
}

// TestCreateSignInUsecase_Execute_InvalidCredentialsは、誤ったパスワードが
// バリデーターの *model.ValidationErrorをそのまま表面化し、ハンドラーがフォームを
// 再描画できることを検証します。
func TestCreateSignInUsecase_Execute_InvalidCredentials(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

	userID := testutil.NewUserBuilder(t, db).WithEmail("signin-uc-bad@example.com").Build()
	testutil.NewUserPasswordBuilder(t, db).WithUserID(userID).WithPassword("password123").Build()

	userRepo := repository.NewUserRepository(db)
	userPasswordRepo := repository.NewUserPasswordRepository(db)
	userTwoFactorAuthRepo := repository.NewUserTwoFactorAuthRepository(db)
	uc := usecase.NewCreateSignInUsecase(validator.NewSignInCreateValidator(userRepo, userPasswordRepo, userTwoFactorAuthRepo))
	out, err := uc.Execute(ctx, usecase.CreateSignInInput{
		Email:    "signin-uc-bad@example.com",
		Password: "wrongpassword",
	})
	if out != nil {
		t.Errorf("Execute()のoutput = %v、期待値 = nil", out)
	}
	if ve := model.AsValidationError(err); ve == nil || !ve.HasGlobalError() {
		t.Fatalf("Execute()のエラー = %v、期待値 = フォーム全体のエラーを持つ*model.ValidationError", err)
	}
}
