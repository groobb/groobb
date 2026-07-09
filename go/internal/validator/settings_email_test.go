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

// TestSettingsEmailUpdateValidator_Validate covers the format checks (new email
// required and well-formed, current password required) that need no database, and
// the state checks that do: a valid new address with the correct current password
// passes, while an unchanged address, an address taken by another account, a wrong
// current password, and an account without a password credential each fail with
// the field error naming the offending input.
//
// [Ja] TestSettingsEmailUpdateValidator_Validate は DB 不要の形式チェック (新しい email の
// 必須・形式、現在のパスワードの必須) と、DB を要する状態チェックを網羅します。有効な新しい
// アドレスと正しい現在のパスワードは通り、未変更のアドレス・別アカウントに使われている
// アドレス・誤った現在のパスワード・パスワード資格情報の無いアカウントは、いずれも該当する
// 入力を指すフィールドエラーで失敗することを確かめます。
func TestSettingsEmailUpdateValidator_Validate(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	userRepo := repository.NewUserRepository(query.New(db)).WithTx(tx)
	userPasswordRepo := repository.NewUserPasswordRepository(query.New(db)).WithTx(tx)
	v := validator.NewSettingsEmailUpdateValidator(userRepo, userPasswordRepo)
	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)

	// The requesting account: a current email and a matching password.
	//
	// [Ja] 申請アカウント: 現在の email と一致するパスワードを持つ。
	userID := testutil.NewUserBuilder(t, tx).WithEmail("member@example.com").Build()
	testutil.NewUserPasswordBuilder(t, tx).WithUserID(userID).WithPassword("password123").Build()

	// Another account whose email the requester must not be able to switch to.
	//
	// [Ja] 申請者が切り替えられてはならない email を持つ別アカウント。
	testutil.NewUserBuilder(t, tx).WithEmail("taken@example.com").Build()

	// An account with no password credential (e.g. an SSO-only user).
	//
	// [Ja] パスワード資格情報の無いアカウント (例: SSO のみのユーザー)。
	noPassUserID := testutil.NewUserBuilder(t, tx).WithEmail("nopass@example.com").Build()

	t.Run("正常系: 有効な新メールと正しい現在パスワードは通る", func(t *testing.T) {
		err := v.Validate(ctx, validator.SettingsEmailUpdateValidatorInput{
			UserID:          userID,
			NewEmail:        "brand-new@example.com",
			CurrentPassword: "password123",
		})
		if err != nil {
			t.Fatalf("Validate() error = %v, want nil", err)
		}
	})

	fieldErrorTests := []struct {
		name      string
		input     validator.SettingsEmailUpdateValidatorInput
		wantField string
	}{
		{
			name:      "異常系: 新メールが空",
			input:     validator.SettingsEmailUpdateValidatorInput{UserID: userID, NewEmail: "", CurrentPassword: "password123"},
			wantField: "email",
		},
		{
			name:      "異常系: 新メールの形式が不正",
			input:     validator.SettingsEmailUpdateValidatorInput{UserID: userID, NewEmail: "not-an-email", CurrentPassword: "password123"},
			wantField: "email",
		},
		{
			name:      "異常系: 現在パスワードが空",
			input:     validator.SettingsEmailUpdateValidatorInput{UserID: userID, NewEmail: "brand-new@example.com", CurrentPassword: ""},
			wantField: "current_password",
		},
		{
			name:      "異常系: 現在のアドレスと同じ (大文字違いも citext で同一)",
			input:     validator.SettingsEmailUpdateValidatorInput{UserID: userID, NewEmail: "MEMBER@example.com", CurrentPassword: "password123"},
			wantField: "email",
		},
		{
			name:      "異常系: 別アカウントが使用中のアドレス",
			input:     validator.SettingsEmailUpdateValidatorInput{UserID: userID, NewEmail: "taken@example.com", CurrentPassword: "password123"},
			wantField: "email",
		},
		{
			name:      "異常系: 現在パスワードが誤り",
			input:     validator.SettingsEmailUpdateValidatorInput{UserID: userID, NewEmail: "brand-new@example.com", CurrentPassword: "wrongpassword"},
			wantField: "current_password",
		},
		{
			name:      "異常系: パスワード資格情報の無いアカウント",
			input:     validator.SettingsEmailUpdateValidatorInput{UserID: noPassUserID, NewEmail: "brand-new@example.com", CurrentPassword: "password123"},
			wantField: "current_password",
		},
	}
	for _, tt := range fieldErrorTests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.Validate(ctx, tt.input)
			ve := model.AsValidationError(err)
			if ve == nil {
				t.Fatalf("Validate() error = %v, want *model.ValidationError", err)
			}
			if !ve.HasFieldError(tt.wantField) {
				t.Errorf("フィールド %q のエラーが無い: %+v", tt.wantField, ve.Fields)
			}
		})
	}
}
