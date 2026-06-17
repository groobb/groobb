package validator_test

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/query"
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

	db, tx := testutil.SetupTx(t)
	userRepo := repository.NewUserRepository(query.New(db)).WithTx(tx)
	userPasswordRepo := repository.NewUserPasswordRepository(query.New(db)).WithTx(tx)
	v := validator.NewSignInCreateValidator(userRepo, userPasswordRepo)
	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)

	// Seed an account with a password to sign in against.
	//
	// [Ja] サインインの対象となる、パスワード付きのアカウントを 1 つ用意する。
	userID := testutil.NewUserBuilder(t, tx).WithEmail("member@example.com").Build()
	testutil.NewUserPasswordBuilder(t, tx).WithUserID(userID).WithPassword("password123").Build()

	// Seed an account without any password (e.g. an SSO-only user) to assert it
	// cannot sign in with a password.
	//
	// [Ja] パスワードの無いアカウント (例: SSO のみのユーザー) を用意し、パスワードでは
	// サインインできないことを確かめる。
	testutil.NewUserBuilder(t, tx).WithEmail("nopass@example.com").Build()

	t.Run("正常系: 正しい資格情報はユーザーを返す", func(t *testing.T) {
		user, err := v.Validate(ctx, validator.SignInCreateValidatorInput{
			Email:    "member@example.com",
			Password: "password123",
		})
		if err != nil {
			t.Fatalf("Validate() error = %v, want nil", err)
		}
		if user == nil || user.ID != userID {
			t.Fatalf("Validate() user = %v, want id %v", user, userID)
		}
	})

	t.Run("正常系: 大文字違いの email でもサインインできる (citext)", func(t *testing.T) {
		user, err := v.Validate(ctx, validator.SignInCreateValidatorInput{
			Email:    "MEMBER@example.com",
			Password: "password123",
		})
		if err != nil {
			t.Fatalf("Validate() error = %v, want nil", err)
		}
		if user == nil || user.ID != userID {
			t.Fatalf("Validate() user = %v, want id %v", user, userID)
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
			user, err := v.Validate(ctx, tt.input)
			if user != nil {
				t.Errorf("Validate() user = %v, want nil", user)
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
			user, err := v.Validate(ctx, tt.input)
			if user != nil {
				t.Errorf("Validate() user = %v, want nil", user)
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
