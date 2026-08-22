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

// TestSignInCreateValidator_Validate covers the format checks (required and
// well-formed email, required password) that need no database, and the state
// checks that do: a correct credential returns the user, while an unknown email,
// a missing password credential, and a wrong password all fail with the same
// generic global message so the form does not leak which accounts exist.
//
// [Ja] TestSignInCreateValidator_Validate は DB 不要の形式チェック (必須・メール形式・
// パスワード必須) と、DB を要する状態チェックを網羅します。正しい資格情報はユーザーを返し、
// 未知の email・パスワード資格情報の欠如・誤ったパスワードはいずれも同じ汎用グローバル
// メッセージで失敗し、フォームがどのアカウントが存在するかを漏らさないことを確かめます。
func TestSignInCreateValidator_Validate(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	userRepo := repository.NewUserRepository(db)
	userPasswordRepo := repository.NewUserPasswordRepository(db)
	userTwoFactorAuthRepo := repository.NewUserTwoFactorAuthRepository(db)
	v := validator.NewSignInCreateValidator(userRepo, userPasswordRepo, userTwoFactorAuthRepo)
	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)

	// Seed an account with a password to sign in against.
	//
	// [Ja] サインインの対象となる、パスワード付きのアカウントを 1 つ用意する。
	userID := testutil.NewUserBuilder(t, db).WithEmail("member@example.com").Build()
	testutil.NewUserPasswordBuilder(t, db).WithUserID(userID).WithPassword("password123").Build()

	// Seed an account without any password (e.g. an SSO-only user) to assert it
	// cannot sign in with a password.
	//
	// [Ja] パスワードの無いアカウント (例: SSO のみのユーザー) を用意し、パスワードでは
	// サインインできないことを確かめる。
	testutil.NewUserBuilder(t, db).WithEmail("nopass@example.com").Build()

	t.Run("正常系: 正しい資格情報はユーザーを返す (2FA 無しなので設定は nil)", func(t *testing.T) {
		output, err := v.Validate(ctx, validator.SignInCreateValidatorInput{
			Email:    "member@example.com",
			Password: "password123",
		})
		if err != nil {
			t.Fatalf("Validate() error = %v, want nil", err)
		}
		if output.User == nil || output.User.ID != userID {
			t.Fatalf("Validate() user = %v, want id %v", output.User, userID)
		}
		if output.UserTwoFactorAuth != nil {
			t.Errorf("Validate() UserTwoFactorAuth = %v, want nil (2FA 未設定のため)", output.UserTwoFactorAuth)
		}
	})

	t.Run("正常系: 大文字違いの email でもサインインできる (NOCASE 照合)", func(t *testing.T) {
		output, err := v.Validate(ctx, validator.SignInCreateValidatorInput{
			Email:    "MEMBER@example.com",
			Password: "password123",
		})
		if err != nil {
			t.Fatalf("Validate() error = %v, want nil", err)
		}
		if output.User == nil || output.User.ID != userID {
			t.Fatalf("Validate() user = %v, want id %v", output.User, userID)
		}
	})

	t.Run("正常系: 2FA 有効なユーザーは有効な 2FA 設定を併せて返す", func(t *testing.T) {
		twoFAUserID := testutil.NewUserBuilder(t, db).WithEmail("2fa-on@example.com").Build()
		testutil.NewUserPasswordBuilder(t, db).WithUserID(twoFAUserID).WithPassword("password123").Build()
		testutil.NewUserTwoFactorAuthBuilder(t, db).WithUserID(twoFAUserID).WithEnabled(true).Build()

		output, err := v.Validate(ctx, validator.SignInCreateValidatorInput{
			Email:    "2fa-on@example.com",
			Password: "password123",
		})
		if err != nil {
			t.Fatalf("Validate() error = %v, want nil", err)
		}
		if output.User == nil || output.User.ID != twoFAUserID {
			t.Fatalf("Validate() user = %v, want id %v", output.User, twoFAUserID)
		}
		if output.UserTwoFactorAuth == nil {
			t.Fatal("Validate() UserTwoFactorAuth = nil, want 有効な 2FA 設定")
		}
		if !output.UserTwoFactorAuth.Enabled {
			t.Error("返された 2FA 設定が enabled でない")
		}
	})

	t.Run("正常系: 登録中 (未有効化) の 2FA は無しとして扱う", func(t *testing.T) {
		enrollingUserID := testutil.NewUserBuilder(t, db).WithEmail("2fa-enrolling@example.com").Build()
		testutil.NewUserPasswordBuilder(t, db).WithUserID(enrollingUserID).WithPassword("password123").Build()
		testutil.NewUserTwoFactorAuthBuilder(t, db).WithUserID(enrollingUserID).WithEnabled(false).Build()

		output, err := v.Validate(ctx, validator.SignInCreateValidatorInput{
			Email:    "2fa-enrolling@example.com",
			Password: "password123",
		})
		if err != nil {
			t.Fatalf("Validate() error = %v, want nil", err)
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
				t.Errorf("Validate() output = %v, want nil", output)
			}
			ve := model.AsValidationError(err)
			if ve == nil {
				t.Fatalf("Validate() error = %v, want *model.ValidationError", err)
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
				t.Errorf("Validate() output = %v, want nil", output)
			}
			ve := model.AsValidationError(err)
			if ve == nil {
				t.Fatalf("Validate() error = %v, want *model.ValidationError", err)
			}
			// A failed credential check reports a single global message and no
			// field error, so it does not point at email vs password.
			//
			// [Ja] 資格情報チェックの失敗は単一のグローバルメッセージのみを報告し、
			// フィールドエラーは出さない。email かパスワードかを指さないため。
			if !ve.HasGlobalError() {
				t.Errorf("グローバルエラーが無い: %+v", ve)
			}
			if ve.HasFieldError("email") || ve.HasFieldError("password") {
				t.Errorf("資格情報の失敗でフィールドエラーが出ている: %+v", ve.Fields)
			}
		})
	}
}
