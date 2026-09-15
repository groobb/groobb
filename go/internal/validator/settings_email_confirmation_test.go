package validator_test

import (
	"context"
	"testing"
	"time"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/validator"
)

// TestSettingsEmailConfirmationCreateValidator_ValidateはDB不要の形式チェック
// (コードの必須・6桁) と、DBを要する状態チェックを網羅します。ユーザーのアクティブな
// メール変更の確認は返され、空・形式不正のコードはcodeフィールドエラーで、確認が無い・
// 期限切れ・試行超過・別イベントの場合はフォーム全体のメッセージで失敗することを
// 確かめます。
func TestSettingsEmailConfirmationCreateValidator_Validate(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	emailConfirmationRepo := repository.NewEmailConfirmationRepository(db)
	v := validator.NewSettingsEmailConfirmationCreateValidator(emailConfirmationRepo)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

	userID := testutil.NewUserBuilder(t, db).WithEmail("member@example.com").Build()

	t.Run("正常系: アクティブなメール変更確認が返る", func(t *testing.T) {
		confirmationID := testutil.NewEmailConfirmationBuilder(t, db).
			WithUserID(userID).
			WithEvent(model.EmailConfirmationEventEmailChange).
			WithEmail("new@example.com").
			WithCode("123456").
			Build()

		confirmation, err := v.Validate(ctx, validator.SettingsEmailConfirmationCreateValidatorInput{
			UserID: userID,
			Code:   "123456",
		})
		if err != nil {
			t.Fatalf("Validate()のエラー = %v、期待値 = nil", err)
		}
		if confirmation == nil || confirmation.ID != confirmationID {
			t.Fatalf("Validate()の確認 = %v、期待値はid %v", confirmation, confirmationID)
		}
		// validatorはコードを照合しない。送信コードが異なってもアクティブな確認を
		// 返す (照合はUseCaseの仕事)。
		if _, err := v.Validate(ctx, validator.SettingsEmailConfirmationCreateValidatorInput{UserID: userID, Code: "000000"}); err != nil {
			t.Errorf("異なるコードでのValidate()のエラー = %v、期待値 = nil (validatorはコードを照合しない)", err)
		}
	})

	fieldErrorTests := []struct {
		name string
		code string
	}{
		{name: "異常系: コードが空", code: ""},
		{name: "異常系: コードが数字でない", code: "abcdef"},
		{name: "異常系: コードの桁数が不正", code: "12345"},
	}
	for _, tt := range fieldErrorTests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := v.Validate(ctx, validator.SettingsEmailConfirmationCreateValidatorInput{UserID: userID, Code: tt.code})
			ve := model.AsValidationError(err)
			if ve == nil {
				t.Fatalf("Validate()のエラー = %v、期待値 = *model.ValidationError", err)
			}
			if !ve.HasFieldError("code") {
				t.Errorf("codeフィールドのエラーが無い: %+v", ve.Fields)
			}
		})
	}

	globalErrorTests := []struct {
		name  string
		build func(t *testing.T) model.UserID
	}{
		{
			name: "異常系: 保留中の確認が無い",
			build: func(t *testing.T) model.UserID {
				return testutil.NewUserBuilder(t, db).WithEmail("nopending@example.com").Build()
			},
		},
		{
			name: "異常系: 確認が期限切れ",
			build: func(t *testing.T) model.UserID {
				id := testutil.NewUserBuilder(t, db).WithEmail("expired@example.com").Build()
				testutil.NewEmailConfirmationBuilder(t, db).
					WithUserID(id).
					WithEvent(model.EmailConfirmationEventEmailChange).
					WithCode("123456").
					WithStartedAt(time.Now().Add(-16 * time.Minute)).
					Build()
				return id
			},
		},
		{
			name: "異常系: 試行回数を使い切った確認",
			build: func(t *testing.T) model.UserID {
				id := testutil.NewUserBuilder(t, db).WithEmail("exhausted@example.com").Build()
				testutil.NewEmailConfirmationBuilder(t, db).
					WithUserID(id).
					WithEvent(model.EmailConfirmationEventEmailChange).
					WithCode("123456").
					WithFailedAttemptsCount(5).
					Build()
				return id
			},
		},
		{
			name: "異常系: サインアップの確認 (event違い) は引かない",
			build: func(t *testing.T) model.UserID {
				id := testutil.NewUserBuilder(t, db).WithEmail("signup@example.com").Build()
				testutil.NewEmailConfirmationBuilder(t, db).
					WithUserID(id).
					WithEvent(model.EmailConfirmationEventSignUp).
					WithCode("123456").
					Build()
				return id
			},
		},
	}
	for _, tt := range globalErrorTests {
		t.Run(tt.name, func(t *testing.T) {
			id := tt.build(t)
			_, err := v.Validate(ctx, validator.SettingsEmailConfirmationCreateValidatorInput{UserID: id, Code: "123456"})
			ve := model.AsValidationError(err)
			if ve == nil {
				t.Fatalf("Validate()のエラー = %v、期待値 = *model.ValidationError", err)
			}
			if !ve.HasGlobalError() {
				t.Errorf("フォーム全体のエラーが無い: %+v", ve.Global)
			}
		})
	}
}
