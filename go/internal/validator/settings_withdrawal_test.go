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

// TestSettingsWithdrawalDeleteValidator_Validate covers the required check (current
// password present) that needs no database and the state check that does: the
// correct current password passes, while a missing current password, a wrong one,
// and an account with no password credential each fail with a current_password
// field error.
//
// [Ja] TestSettingsWithdrawalDeleteValidator_Validate は DB 不要の必須チェック (現在の
// パスワードの入力) と、DB を要する状態チェックを網羅します。正しい現在のパスワードは
// 通り、未入力・誤り・パスワード資格情報の無いアカウントは、いずれも current_password の
// フィールドエラーで失敗することを確かめます。
func TestSettingsWithdrawalDeleteValidator_Validate(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	userPasswordRepo := repository.NewUserPasswordRepository(db)
	v := validator.NewSettingsWithdrawalDeleteValidator(userPasswordRepo)
	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)

	// The requesting account: a matching password.
	//
	// [Ja] 申請アカウント: 一致するパスワードを持つ。
	userID := testutil.NewUserBuilder(t, db).Build()
	testutil.NewUserPasswordBuilder(t, db).WithUserID(userID).WithPassword("password123").Build()

	// An account with no password credential (e.g. an SSO-only user).
	//
	// [Ja] パスワード資格情報の無いアカウント (例: SSO のみのユーザー)。
	noPassUserID := testutil.NewUserBuilder(t, db).Build()

	t.Run("正常系: 正しい現在パスワードは通る", func(t *testing.T) {
		err := v.Validate(ctx, validator.SettingsWithdrawalDeleteValidatorInput{
			UserID:          userID,
			CurrentPassword: "password123",
		})
		if err != nil {
			t.Fatalf("Validate() error = %v, want nil", err)
		}
	})

	fieldErrorTests := []struct {
		name      string
		input     validator.SettingsWithdrawalDeleteValidatorInput
		wantField string
	}{
		{
			name:      "異常系: 現在パスワードが空",
			input:     validator.SettingsWithdrawalDeleteValidatorInput{UserID: userID, CurrentPassword: ""},
			wantField: "current_password",
		},
		{
			name:      "異常系: 現在パスワードが誤り",
			input:     validator.SettingsWithdrawalDeleteValidatorInput{UserID: userID, CurrentPassword: "wrongpassword"},
			wantField: "current_password",
		},
		{
			name:      "異常系: パスワード資格情報の無いアカウント",
			input:     validator.SettingsWithdrawalDeleteValidatorInput{UserID: noPassUserID, CurrentPassword: "password123"},
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
