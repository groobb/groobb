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

// TestSignUpCreateValidator_ValidateはDB不要の形式チェック (必須・メール形式)
// と、DBを要する重複メールの状態チェックを網羅します。
func TestSignUpCreateValidator_Validate(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	userRepo := repository.NewUserRepository(db)
	v := validator.NewSignUpCreateValidator(userRepo)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

	// 重複ケースが衝突する相手を作るため、既存アカウントを1つ用意する。形式
	// ケースは存在しないアドレスを使う。
	testutil.NewUserBuilder(t, db).WithEmail("taken@example.com").Build()

	tests := []struct {
		name      string
		email     string
		wantErr   bool
		wantField string
	}{
		{name: "正常系: 未登録の有効なメール", email: "new@example.com", wantErr: false},
		{name: "異常系: メールが空", email: "", wantErr: true, wantField: "email"},
		{name: "異常系: メール形式が不正", email: "not-an-email", wantErr: true, wantField: "email"},
		{name: "異常系: 既に使われているメール", email: "taken@example.com", wantErr: true, wantField: "email"},
		{name: "異常系: 大文字違いでも重複扱い (NOCASE照合)", email: "TAKEN@example.com", wantErr: true, wantField: "email"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.Validate(ctx, validator.SignUpCreateValidatorInput{Email: tt.email})

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
				t.Errorf("フィールド %q のエラーが無い: %+v", tt.wantField, ve.Fields)
			}
		})
	}
}
