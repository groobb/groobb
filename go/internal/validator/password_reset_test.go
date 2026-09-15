package validator_test

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/validator"
)

// TestPasswordResetCreateValidator_Validateはemail形式のルールを確認します。
// 未入力または形式不正はフィールドエラーで、形式の正しいアドレスは通過します。どの
// アカウントにも属さないアドレスを含めて通過するのは、登録の有無を明かして
// 列挙攻撃を可能にしないよう、バリデーターが意図的に存在チェックを省くためです。
func TestPasswordResetCreateValidator_Validate(t *testing.T) {
	t.Parallel()

	v := validator.NewPasswordResetCreateValidator()

	tests := []struct {
		name      string
		email     string
		wantErr   bool
		wantField string
	}{
		{
			name:    "正常系: 形式の正しいemail",
			email:   "user@example.com",
			wantErr: false,
		},
		{
			name:    "正常系: 未登録でも形式が正しければ通過する (列挙攻撃対策)",
			email:   "never-registered@example.com",
			wantErr: false,
		},
		{
			name:      "異常系: emailが空",
			email:     "",
			wantErr:   true,
			wantField: "email",
		},
		{
			name:      "異常系: emailの形式が不正",
			email:     "not-an-email",
			wantErr:   true,
			wantField: "email",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := i18n.SetLocale(context.Background(), model.LocaleJa)
			err := v.Validate(ctx, validator.PasswordResetCreateValidatorInput{Email: tt.email})

			if !tt.wantErr {
				if err != nil {
					t.Fatalf("Validate()のエラー = %v、期待値 = nil", err)
				}
				return
			}

			ve := model.AsValidationError(err)
			if ve == nil {
				t.Fatalf("Validate()のエラー = %v、期待値 = *model.ValidationError", err)
			}
			if !ve.HasFieldError(tt.wantField) {
				t.Errorf("フィールド %q のエラーが無い", tt.wantField)
			}
		})
	}
}
