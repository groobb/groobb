package validator_test

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/validator"
)

// TestSignInCreateValidator_ValidateはDB不要の形式チェック (必須・メール形式・
// パスワード必須) と、DBを要する状態チェックを網羅します。正しい資格情報はユーザーを返し、
// 未知のemail・パスワード資格情報の欠如・誤ったパスワードはいずれも同じ汎用グローバル
// メッセージで失敗し、フォームがどのアカウントが存在するかを漏らさないことを確かめます。
func TestSignInCreateValidator_Validate(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	userRepo := repository.NewUserRepository(db)
	userPasswordRepo := repository.NewUserPasswordRepository(db)
	userTwoFactorAuthRepo := repository.NewUserTwoFactorAuthRepository(db)
	v := validator.NewSignInCreateValidator(userRepo, userPasswordRepo, userTwoFactorAuthRepo)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

	// サインインの対象となる、パスワード付きのアカウントを1つ用意する。
	userID := testutil.NewUserBuilder(t, db).WithEmail("member@example.com").Build()
	testutil.NewUserPasswordBuilder(t, db).WithUserID(userID).WithPassword("password123").Build()

	// パスワードの無いアカウント (例: SSOのみのユーザー) を用意し、パスワードでは
	// サインインできないことを確かめる。
	testutil.NewUserBuilder(t, db).WithEmail("nopass@example.com").Build()

	t.Run("正常系: 正しい資格情報はユーザーを返す (2FA無しなので設定はnil)", func(t *testing.T) {
		output, err := v.Validate(ctx, validator.SignInCreateValidatorInput{
			Email:    "member@example.com",
			Password: "password123",
		})
		if err != nil {
			t.Fatalf("Validate()のエラー = %v、期待値 = nil", err)
		}
		if output.User == nil || output.User.ID != userID {
			t.Fatalf("Validate()のユーザー = %v、期待値はid %v", output.User, userID)
		}
		if output.UserTwoFactorAuth != nil {
			t.Errorf("Validate()のUserTwoFactorAuth = %v、期待値 = nil (2FA未設定のため)", output.UserTwoFactorAuth)
		}
	})

	t.Run("正常系: 大文字違いのemailでもサインインできる (NOCASE照合)", func(t *testing.T) {
		output, err := v.Validate(ctx, validator.SignInCreateValidatorInput{
			Email:    "MEMBER@example.com",
			Password: "password123",
		})
		if err != nil {
			t.Fatalf("Validate()のエラー = %v、期待値 = nil", err)
		}
		if output.User == nil || output.User.ID != userID {
			t.Fatalf("Validate()のユーザー = %v、期待値はid %v", output.User, userID)
		}
	})

	t.Run("正常系: 2FA有効なユーザーは有効な2FA設定を併せて返す", func(t *testing.T) {
		twoFAUserID := testutil.NewUserBuilder(t, db).WithEmail("2fa-on@example.com").Build()
		testutil.NewUserPasswordBuilder(t, db).WithUserID(twoFAUserID).WithPassword("password123").Build()
		testutil.NewUserTwoFactorAuthBuilder(t, db).WithUserID(twoFAUserID).WithEnabled(true).Build()

		output, err := v.Validate(ctx, validator.SignInCreateValidatorInput{
			Email:    "2fa-on@example.com",
			Password: "password123",
		})
		if err != nil {
			t.Fatalf("Validate()のエラー = %v、期待値 = nil", err)
		}
		if output.User == nil || output.User.ID != twoFAUserID {
			t.Fatalf("Validate()のユーザー = %v、期待値はid %v", output.User, twoFAUserID)
		}
		if output.UserTwoFactorAuth == nil {
			t.Fatal("Validate()のUserTwoFactorAuth = nil、期待値は有効な2FA設定")
		}
		if !output.UserTwoFactorAuth.Enabled {
			t.Error("返された2FA設定がenabledでない")
		}
	})

	t.Run("正常系: 登録中 (未有効化) の2FAは無しとして扱う", func(t *testing.T) {
		enrollingUserID := testutil.NewUserBuilder(t, db).WithEmail("2fa-enrolling@example.com").Build()
		testutil.NewUserPasswordBuilder(t, db).WithUserID(enrollingUserID).WithPassword("password123").Build()
		testutil.NewUserTwoFactorAuthBuilder(t, db).WithUserID(enrollingUserID).WithEnabled(false).Build()

		output, err := v.Validate(ctx, validator.SignInCreateValidatorInput{
			Email:    "2fa-enrolling@example.com",
			Password: "password123",
		})
		if err != nil {
			t.Fatalf("Validate()のエラー = %v、期待値 = nil", err)
		}
		if output.UserTwoFactorAuth != nil {
			t.Errorf("Validate() UserTwoFactorAuth = %v, want nil (未有効化のため)", output.UserTwoFactorAuth)
		}
	})

	fieldErrorTests := []struct {
		name      string
		input     validator.SignInCreateValidatorInput
		wantField string
	}{
		{
			name:      "異常系: メールが空",
			input:     validator.SignInCreateValidatorInput{Email: "", Password: "password123"},
			wantField: "email",
		},
		{
			name:      "異常系: メール形式が不正",
			input:     validator.SignInCreateValidatorInput{Email: "not-an-email", Password: "password123"},
			wantField: "email",
		},
		{
			name:      "異常系: パスワードが空",
			input:     validator.SignInCreateValidatorInput{Email: "member@example.com", Password: ""},
			wantField: "password",
		},
	}
	for _, tt := range fieldErrorTests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := v.Validate(ctx, tt.input)
			if output != nil {
				t.Errorf("Validate()の出力 = %v、期待値 = nil", output)
			}
			ve := model.AsValidationError(err)
			if ve == nil {
				t.Fatalf("Validate()のエラー = %v、期待値 = *model.ValidationError", err)
			}
			if !ve.HasFieldError(tt.wantField) {
				t.Errorf("フィールド %q のエラーが無い: %+v", tt.wantField, ve.Fields)
			}
		})
	}

	globalErrorTests := []struct {
		name  string
		input validator.SignInCreateValidatorInput
	}{
		{
			name:  "異常系: 未登録のメール",
			input: validator.SignInCreateValidatorInput{Email: "unknown@example.com", Password: "password123"},
		},
		{
			name:  "異常系: パスワードの無いアカウント",
			input: validator.SignInCreateValidatorInput{Email: "nopass@example.com", Password: "password123"},
		},
		{
			name:  "異常系: パスワードが誤り",
			input: validator.SignInCreateValidatorInput{Email: "member@example.com", Password: "wrongpassword"},
		},
	}
	for _, tt := range globalErrorTests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := v.Validate(ctx, tt.input)
			if output != nil {
				t.Errorf("Validate()の出力 = %v、期待値 = nil", output)
			}
			ve := model.AsValidationError(err)
			if ve == nil {
				t.Fatalf("Validate()のエラー = %v、期待値 = *model.ValidationError", err)
			}
			// 資格情報チェックの失敗は単一のグローバルメッセージのみを報告し、
			// フィールドエラーは出さない。emailかパスワードかを指さないため。
			if !ve.HasGlobalError() {
				t.Errorf("グローバルエラーが無い: %+v", ve)
			}
			if ve.HasFieldError("email") || ve.HasFieldError("password") {
				t.Errorf("資格情報の失敗でフィールドエラーが出ている: %+v", ve.Fields)
			}
		})
	}
}
