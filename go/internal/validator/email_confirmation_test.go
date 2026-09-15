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

// TestEmailConfirmationCreateValidator_ValidateはDB不要の形式チェック (入力済み
// の6桁数字コード) と、DBを要する状態チェックを網羅する。idのアクティブな確認は
// コードが一致するかどうかに関わらず返り (コード照合はUseCaseへ移動)、未知のid・
// 期限切れ・試行回数超過はいずれもフォーム全体のメッセージで失敗する。
func TestEmailConfirmationCreateValidator_Validate(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	repo := repository.NewEmailConfirmationRepository(db)
	v := validator.NewEmailConfirmationCreateValidator(repo)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

	// 既知の保存コードを持つアクティブな確認。形式エラーのケースもこのidを使うが、
	// DBルックアップの前に失敗するため保存済みのコードは関係しない。
	activeID := testutil.NewEmailConfirmationBuilder(t, db).WithCode("123456").Build()

	t.Run("正常系: 形式が正しければ有効な確認を返す", func(t *testing.T) {
		got, err := v.Validate(ctx, validator.EmailConfirmationCreateValidatorInput{ID: activeID, Code: "123456"})
		if err != nil {
			t.Fatalf("Validate()のエラー = %v、期待値 = nil", err)
		}
		if got == nil || got.ID != activeID {
			t.Fatalf("アクティブな確認を返すはず: %+v", got)
		}
	})

	t.Run("正常系: コードが保存値と一致しなくても有効な確認を返す (照合はUseCaseの責務)", func(t *testing.T) {
		got, err := v.Validate(ctx, validator.EmailConfirmationCreateValidatorInput{ID: activeID, Code: "000000"})
		if err != nil {
			t.Fatalf("Validate() error = %v, want nil (照合はしないため)", err)
		}
		if got == nil || got.ID != activeID {
			t.Fatalf("コード不一致でもアクティブな確認を返すはず: %+v", got)
		}
	})

	formatCases := []struct {
		name string
		code string
	}{
		{name: "異常系: コードが空", code: ""},
		{name: "異常系: 数字以外を含む", code: "12a456"},
		{name: "異常系: 桁数が足りない", code: "12345"},
		{name: "異常系: 桁数が多い", code: "1234567"},
	}
	for _, tc := range formatCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := v.Validate(ctx, validator.EmailConfirmationCreateValidatorInput{ID: activeID, Code: tc.code})
			if got != nil {
				t.Errorf("形式エラー時はnilを返すはず: %+v", got)
			}
			ve := model.AsValidationError(err)
			if ve == nil {
				t.Fatalf("Validate()のエラー = %v、期待値 = *model.ValidationError", err)
			}
			if !ve.HasFieldError("code") {
				t.Errorf("codeフィールドのエラーが無い: %+v", ve.Fields)
			}
		})
	}

	t.Run("異常系: 有効な確認が存在しないid", func(t *testing.T) {
		got, err := v.Validate(ctx, validator.EmailConfirmationCreateValidatorInput{
			ID:   model.EmailConfirmationID(testutil.UnusedID),
			Code: "123456",
		})
		if got != nil {
			t.Errorf("未存在時はnilを返すはず: %+v", got)
		}
		ve := model.AsValidationError(err)
		if ve == nil {
			t.Fatalf("Validate()のエラー = %v、期待値 = *model.ValidationError", err)
		}
		if !ve.HasGlobalError() {
			t.Errorf("フォーム全体のエラーが無い: %+v", ve.Global)
		}
	})

	t.Run("異常系: 期限切れの確認はグローバルエラー", func(t *testing.T) {
		expiredID := testutil.NewEmailConfirmationBuilder(t, db).
			WithCode("654321").
			WithStartedAt(time.Now().Add(-16 * time.Minute)).
			Build()

		got, err := v.Validate(ctx, validator.EmailConfirmationCreateValidatorInput{ID: expiredID, Code: "654321"})
		if got != nil {
			t.Errorf("期限切れ時はnilを返すはず: %+v", got)
		}
		ve := model.AsValidationError(err)
		if ve == nil {
			t.Fatalf("Validate()のエラー = %v、期待値 = *model.ValidationError", err)
		}
		if !ve.HasGlobalError() {
			t.Errorf("フォーム全体のエラーが無い: %+v", ve.Global)
		}
	})

	t.Run("異常系: 試行回数を使い切った確認はグローバルエラー", func(t *testing.T) {
		exhaustedID := testutil.NewEmailConfirmationBuilder(t, db).
			WithCode("654321").
			WithFailedAttemptsCount(5).
			Build()

		got, err := v.Validate(ctx, validator.EmailConfirmationCreateValidatorInput{ID: exhaustedID, Code: "654321"})
		if got != nil {
			t.Errorf("試行回数超過時はnilを返すはず: %+v", got)
		}
		ve := model.AsValidationError(err)
		if ve == nil {
			t.Fatalf("Validate()のエラー = %v、期待値 = *model.ValidationError", err)
		}
		if !ve.HasGlobalError() {
			t.Errorf("フォーム全体のエラーが無い: %+v", ve.Global)
		}
	})
}
