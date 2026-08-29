package validator_test

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/validator"
)

// TestPasswordResetCreateValidator_Validate checks the email format rules: a
// missing or malformed email is a field error, while a well-formed address
// passes—including one that does not belong to any account, since the validator
// deliberately does not check existence (that would enable enumeration).
//
// [Ja] TestPasswordResetCreateValidator_Validate は email 形式のルールを確認します。
// 未入力または形式不正はフィールドエラーで、形式の正しいアドレスは通過します。どの
// アカウントにも属さないアドレスを含めて通過するのは、バリデーターが意図的に存在チェックを
// しない (列挙を可能にするため) からです。
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
			name:    "正常系: 形式の正しい email",
			email:   "user@example.com",
			wantErr: false,
		},
		{
			name:    "正常系: 未登録でも形式が正しければ通過する (列挙攻撃対策)",
			email:   "never-registered@example.com",
			wantErr: false,
		},
		{
			name:      "異常系: email が空",
			email:     "",
			wantErr:   true,
			wantField: "email",
		},
		{
			name:      "異常系: email の形式が不正",
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
					t.Fatalf("Validate() error = %v, want nil", err)
				}
				return
			}

			ve := model.AsValidationError(err)
			if ve == nil {
				t.Fatalf("Validate() error = %v, want *model.ValidationError", err)
			}
			if !ve.HasFieldError(tt.wantField) {
				t.Errorf("expected a field error on %q", tt.wantField)
			}
		})
	}
}
