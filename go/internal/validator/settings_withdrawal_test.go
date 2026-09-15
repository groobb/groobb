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

// TestSettingsWithdrawalDeleteValidator_ValidateはDB不要の必須チェック (現在の
// パスワードの入力) と、DBを要する状態チェックを網羅します。正しい現在のパスワードは
// 通り、未入力・誤り・パスワード資格情報の無いアカウントは、いずれもcurrent_passwordの
// フィールドエラーで失敗することを確かめます。
func TestSettingsWithdrawalDeleteValidator_Validate(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	userPasswordRepo := repository.NewUserPasswordRepository(db)
	v := validator.NewSettingsWithdrawalDeleteValidator(userPasswordRepo)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

	// 申請アカウント: 一致するパスワードを持つ。
	userID := testutil.NewUserBuilder(t, db).Build()
	testutil.NewUserPasswordBuilder(t, db).WithUserID(userID).WithPassword("password123").Build()

	// パスワード資格情報の無いアカウント (例: SSOのみのユーザー)。
	noPassUserID := testutil.NewUserBuilder(t, db).Build()

	t.Run("正常系: 正しい現在パスワードは通る", func(t *testing.T) {
		err := v.Validate(ctx, validator.SettingsWithdrawalDeleteValidatorInput{
			UserID:          userID,
			CurrentPassword: "password123",
		})
		if err != nil {
			t.Fatalf("Validate()のエラー = %v、期待値 = nil", err)
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
				t.Fatalf("Validate()のエラー = %v、期待値 = *model.ValidationError", err)
			}
			if !ve.HasFieldError(tt.wantField) {
				t.Errorf("フィールド %q のエラーが無い: %+v", tt.wantField, ve.Fields)
			}
		})
	}
}
