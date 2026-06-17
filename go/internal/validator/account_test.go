package validator_test

import (
	"context"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/validator"
)

// TestAccountCreateValidator_Validate covers the password-policy checks (no
// database needed): a valid password and matching confirmation succeed, while an
// empty, too-short, or too-long password, an empty or non-matching confirmation,
// each fail with a field error on the expected field.
//
// [Ja] TestAccountCreateValidator_Validate はパスワードポリシーのチェック (DB 不要) を
// 網羅します。有効なパスワードと一致する確認は成功し、空・短すぎ・長すぎのパスワード、
// 空または不一致の確認は、いずれも期待するフィールドのフィールドエラーで失敗します。
func TestAccountCreateValidator_Validate(t *testing.T) {
	t.Parallel()

	v := validator.NewAccountCreateValidator()
	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)

	tests := []struct {
		name          string
		input         validator.AccountCreateValidatorInput
		wantErr       bool
		expectedField string
	}{
		{
			name:    "正常系: 有効なパスワードと一致する確認",
			input:   validator.AccountCreateValidatorInput{Password: "password123", PasswordConfirmation: "password123"},
			wantErr: false,
		},
		{
			name:          "異常系: パスワードが空",
			input:         validator.AccountCreateValidatorInput{Password: "", PasswordConfirmation: ""},
			wantErr:       true,
			expectedField: "password",
		},
		{
			name:          "異常系: パスワードが短すぎる",
			input:         validator.AccountCreateValidatorInput{Password: "short", PasswordConfirmation: "short"},
			wantErr:       true,
			expectedField: "password",
		},
		{
			name:          "異常系: パスワードが長すぎる",
			input:         validator.AccountCreateValidatorInput{Password: strings.Repeat("a", 73), PasswordConfirmation: strings.Repeat("a", 73)},
			wantErr:       true,
			expectedField: "password",
		},
		{
			name:          "異常系: 確認が空",
			input:         validator.AccountCreateValidatorInput{Password: "password123", PasswordConfirmation: ""},
			wantErr:       true,
			expectedField: "password_confirmation",
		},
		{
			name:          "異常系: 確認が一致しない",
			input:         validator.AccountCreateValidatorInput{Password: "password123", PasswordConfirmation: "password456"},
			wantErr:       true,
			expectedField: "password_confirmation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := v.Validate(ctx, tt.input)

			if !tt.wantErr {
				if err != nil {
					t.Fatalf("Validate() error = %v, want nil", err)
				}
				return
			}

			ve := model.AsValidationError(err)
			if ve == nil {
				t.Fatalf("Validate() error = %v, want *model.ValidationError", err)
			}
			if !ve.HasFieldError(tt.expectedField) {
				t.Errorf("%q フィールドのエラーが無い: %+v", tt.expectedField, ve.Fields)
			}
		})
	}
}
