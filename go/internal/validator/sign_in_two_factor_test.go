package validator_test

import (
	"context"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/validator"
)

// newSignInTwoFactorValidatorはテスト専用のデータベース上に
// SignInTwoFactorCreateValidatorを組み立て、その2FAルックアップがそこへ仕込んだ行を
// 読むようにする。
func newSignInTwoFactorValidator(t *testing.T, db *database.DB) *validator.SignInTwoFactorCreateValidator {
	t.Helper()
	repo := repository.NewUserTwoFactorAuthRepository(db)
	return validator.NewSignInTwoFactorCreateValidator(repo)
}

// TestSignInTwoFactorCreateValidator_Validate_Successは、2FAが有効なユーザーの
// 正しいTOTPコードがエラーなしで通ることを検証する。
func TestSignInTwoFactorCreateValidator_Validate_Success(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

	userID := testutil.NewUserBuilder(t, db).WithEmail("2fa-v@example.com").Build()
	testutil.NewUserTwoFactorAuthBuilder(t, db).WithUserID(userID).WithEnabled(true).Build()

	code, err := totp.GenerateCode(testutil.DefaultBuilderTOTPSecret, time.Now())
	if err != nil {
		t.Fatalf("テスト用TOTPコードの生成に失敗: %v", err)
	}

	v := newSignInTwoFactorValidator(t, db)
	if err := v.Validate(ctx, validator.SignInTwoFactorCreateValidatorInput{UserID: userID, Code: code}); err != nil {
		t.Fatalf("Validate()のエラー = %v、期待値 = nil", err)
	}
}

// TestSignInTwoFactorCreateValidator_Validate_FieldErrorsは、コードの未入力・
// 形式不正・不一致がそれぞれcodeフィールドのValidationErrorとして表れ、チャレンジが
// 通らないことを検証する。
func TestSignInTwoFactorCreateValidator_Validate_FieldErrors(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

	userID := testutil.NewUserBuilder(t, db).WithEmail("2fa-v-bad@example.com").Build()
	testutil.NewUserTwoFactorAuthBuilder(t, db).WithUserID(userID).WithEnabled(true).Build()

	validCode, err := totp.GenerateCode(testutil.DefaultBuilderTOTPSecret, time.Now())
	if err != nil {
		t.Fatalf("テスト用TOTPコードの生成に失敗: %v", err)
	}
	// 整った形式で、意図的に現在のコードと等しくない値。
	wrongCode := "000000"
	if wrongCode == validCode {
		wrongCode = "111111"
	}

	tests := []struct {
		name string
		code string
	}{
		{name: "未入力", code: ""},
		{name: "形式不正", code: "abc"},
		{name: "不一致", code: wrongCode},
	}

	v := newSignInTwoFactorValidator(t, db)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := v.Validate(ctx, validator.SignInTwoFactorCreateValidatorInput{UserID: userID, Code: tt.code})
			ve := model.AsValidationError(err)
			if ve == nil {
				t.Fatalf("Validate()のエラー = %v、期待値 = *model.ValidationError", err)
			}
			if !ve.HasFieldError("code") {
				t.Error("codeフィールドのエラーが無い")
			}
		})
	}
}

// TestSignInTwoFactorCreateValidator_Validate_NoEnabledTwoFactorは、有効な2FAを
// 持たない保留中ユーザー (登録中のみの行、または全く無い) がフォーム全体のエラーで失敗し、
// 失効・偽造したCookieがチャレンジを通せないことを検証する。
func TestSignInTwoFactorCreateValidator_Validate_NoEnabledTwoFactor(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

	// 2FAが登録中 (未有効化) のみのユーザーは有効な2FA無しと数えるため、形式の
	// 整ったコードでも受理できない。
	userID := testutil.NewUserBuilder(t, db).WithEmail("2fa-v-none@example.com").Build()
	testutil.NewUserTwoFactorAuthBuilder(t, db).WithUserID(userID).Build()

	code, err := totp.GenerateCode(testutil.DefaultBuilderTOTPSecret, time.Now())
	if err != nil {
		t.Fatalf("テスト用TOTPコードの生成に失敗: %v", err)
	}

	v := newSignInTwoFactorValidator(t, db)
	err = v.Validate(ctx, validator.SignInTwoFactorCreateValidatorInput{UserID: userID, Code: code})
	ve := model.AsValidationError(err)
	if ve == nil {
		t.Fatalf("Validate()のエラー = %v、期待値 = *model.ValidationError", err)
	}
	if !ve.HasGlobalError() {
		t.Error("フォーム全体のエラーが無い (有効な2FAが無いチャレンジはフォーム全体で失敗すべき)")
	}
}
