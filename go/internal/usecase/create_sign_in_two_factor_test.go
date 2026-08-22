package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/validator"
)

// TestCreateSignInTwoFactorUsecase_Execute_Success verifies that Execute returns
// nil when a correct TOTP code is submitted for a user with enabled 2FA, so the
// handler proceeds to issue the session.
//
// [Ja] TestCreateSignInTwoFactorUsecase_Execute_Success は、2FA が有効なユーザーに正しい
// TOTP コードが送られたとき Execute が nil を返し、ハンドラーがセッション発行へ進めることを
// 検証する。
func TestCreateSignInTwoFactorUsecase_Execute_Success(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)

	userID := testutil.NewUserBuilder(t, db).WithEmail("2fa-uc@example.com").Build()
	testutil.NewUserTwoFactorAuthBuilder(t, db).WithUserID(userID).WithEnabled(true).Build()

	code, err := totp.GenerateCode(testutil.DefaultBuilderTOTPSecret, time.Now())
	if err != nil {
		t.Fatalf("テスト用 TOTP コードの生成に失敗: %v", err)
	}

	repo := repository.NewUserTwoFactorAuthRepository(db)
	uc := usecase.NewCreateSignInTwoFactorUsecase(validator.NewSignInTwoFactorCreateValidator(repo))
	if err := uc.Execute(ctx, usecase.CreateSignInTwoFactorInput{UserID: userID, Code: code}); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
}

// TestCreateSignInTwoFactorUsecase_Execute_WrongCode verifies that a non-matching
// code surfaces the validator's *model.ValidationError unchanged, so the handler
// re-renders the challenge form.
//
// [Ja] TestCreateSignInTwoFactorUsecase_Execute_WrongCode は、一致しないコードが
// バリデーターの *model.ValidationError をそのまま表面化し、ハンドラーがチャレンジフォームを
// 再描画できることを検証する。
func TestCreateSignInTwoFactorUsecase_Execute_WrongCode(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)

	userID := testutil.NewUserBuilder(t, db).WithEmail("2fa-uc-bad@example.com").Build()
	testutil.NewUserTwoFactorAuthBuilder(t, db).WithUserID(userID).WithEnabled(true).Build()

	validCode, err := totp.GenerateCode(testutil.DefaultBuilderTOTPSecret, time.Now())
	if err != nil {
		t.Fatalf("テスト用 TOTP コードの生成に失敗: %v", err)
	}
	wrongCode := "000000"
	if wrongCode == validCode {
		wrongCode = "111111"
	}

	repo := repository.NewUserTwoFactorAuthRepository(db)
	uc := usecase.NewCreateSignInTwoFactorUsecase(validator.NewSignInTwoFactorCreateValidator(repo))
	err = uc.Execute(ctx, usecase.CreateSignInTwoFactorInput{UserID: userID, Code: wrongCode})
	if ve := model.AsValidationError(err); ve == nil {
		t.Fatalf("Execute() error = %v, want *model.ValidationError", err)
	}
}
