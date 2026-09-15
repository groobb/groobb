package validator_test

import (
	"context"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/validator"
)

// newTwoFactorAuthValidatorはテスト専用のデータベース上に
// SettingsTwoFactorAuthCreateValidatorを作り、2FA設定の所有ユーザーを作成する。
// サブテストがそのユーザーの登録行を投入できるよう、validatorとユーザーIDを返す。
func newTwoFactorAuthValidator(t *testing.T, db *database.DB) (*validator.SettingsTwoFactorAuthCreateValidator, model.UserID) {
	t.Helper()
	userID := testutil.NewUserBuilder(t, db).Build()
	repo := repository.NewUserTwoFactorAuthRepository(db)
	return validator.NewSettingsTwoFactorAuthCreateValidator(repo), userID
}

// validTOTPCodeはDefaultBuilderTOTPSecret (未有効化の登録行に投入されるsecret) に
// 対する現在のTOTPコードを返す。ValidateTOTPCodeが許容する ±1ステップのスキューにより、
// 生成と検証の間でステップ境界を跨いでもこのコードは受理される。
func validTOTPCode(t *testing.T) string {
	t.Helper()
	code, err := totp.GenerateCode(testutil.DefaultBuilderTOTPSecret, time.Now())
	if err != nil {
		t.Fatalf("テスト用TOTPコードの生成に失敗: %v", err)
	}
	return code
}

// TestSettingsTwoFactorAuthCreateValidator_Validateは有効化フォームの検証を網羅する。
// 登録中の設定に対する正しいコードは通り、未入力・不正な形式のコードはcodeフィールドの
// エラー、登録中の設定が無い (未存在、または既に有効) はフォーム全体のエラー、secretと
// 一致しない整った形式のコードはcodeフィールドのエラーになる。
func TestSettingsTwoFactorAuthCreateValidator_Validate(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	t.Run("正常系: 登録中の設定に対する正しいコード", func(t *testing.T) {
		t.Parallel()
		v, userID := newTwoFactorAuthValidator(t, db)
		testutil.NewUserTwoFactorAuthBuilder(t, db).WithUserID(userID).Build()

		ctx := i18n.SetLocale(context.Background(), model.LocaleJa)
		err := v.Validate(ctx, validator.SettingsTwoFactorAuthCreateValidatorInput{
			UserID: userID,
			Code:   validTOTPCode(t),
		})
		if err != nil {
			t.Fatalf("Validate()のエラー = %v、期待値 = nil", err)
		}
	})

	t.Run("異常系: コードが空", func(t *testing.T) {
		t.Parallel()
		v, userID := newTwoFactorAuthValidator(t, db)
		testutil.NewUserTwoFactorAuthBuilder(t, db).WithUserID(userID).Build()

		ctx := i18n.SetLocale(context.Background(), model.LocaleJa)
		err := v.Validate(ctx, validator.SettingsTwoFactorAuthCreateValidatorInput{UserID: userID, Code: ""})

		ve := model.AsValidationError(err)
		if ve == nil {
			t.Fatalf("Validate()のエラー = %v、期待値 = *ValidationError", err)
		}
		if !ve.HasFieldError("code") {
			t.Error("codeフィールドのエラーが無い")
		}
	})

	t.Run("異常系: コードの形式が不正", func(t *testing.T) {
		t.Parallel()
		v, userID := newTwoFactorAuthValidator(t, db)
		testutil.NewUserTwoFactorAuthBuilder(t, db).WithUserID(userID).Build()

		ctx := i18n.SetLocale(context.Background(), model.LocaleJa)
		err := v.Validate(ctx, validator.SettingsTwoFactorAuthCreateValidatorInput{UserID: userID, Code: "12ab5"})

		ve := model.AsValidationError(err)
		if ve == nil {
			t.Fatalf("Validate()のエラー = %v、期待値 = *ValidationError", err)
		}
		if !ve.HasFieldError("code") {
			t.Error("codeフィールドのエラーが無い")
		}
	})

	t.Run("異常系: 登録中の設定が無い", func(t *testing.T) {
		t.Parallel()
		db := testutil.SetupDB(t)
		v, userID := newTwoFactorAuthValidator(t, db)

		ctx := i18n.SetLocale(context.Background(), model.LocaleJa)
		err := v.Validate(ctx, validator.SettingsTwoFactorAuthCreateValidatorInput{UserID: userID, Code: "123456"})

		ve := model.AsValidationError(err)
		if ve == nil {
			t.Fatalf("Validate()のエラー = %v、期待値 = *ValidationError", err)
		}
		if !ve.HasGlobalError() {
			t.Error("フォーム全体のエラーが無い")
		}
	})

	t.Run("異常系: 既に有効", func(t *testing.T) {
		t.Parallel()
		v, userID := newTwoFactorAuthValidator(t, db)
		testutil.NewUserTwoFactorAuthBuilder(t, db).WithUserID(userID).WithEnabled(true).Build()

		ctx := i18n.SetLocale(context.Background(), model.LocaleJa)
		err := v.Validate(ctx, validator.SettingsTwoFactorAuthCreateValidatorInput{UserID: userID, Code: validTOTPCode(t)})

		ve := model.AsValidationError(err)
		if ve == nil {
			t.Fatalf("Validate()のエラー = %v、期待値 = *ValidationError", err)
		}
		if !ve.HasGlobalError() {
			t.Error("フォーム全体のエラーが無い")
		}
	})

	t.Run("異常系: コードがsecretと一致しない", func(t *testing.T) {
		t.Parallel()
		v, userID := newTwoFactorAuthValidator(t, db)
		testutil.NewUserTwoFactorAuthBuilder(t, db).WithUserID(userID).Build()

		// 整った形式で、意図的に現在のコードではない値を選び、形式の問題ではない不一致を
		// 検証する。
		wrongCode := "000000"
		if wrongCode == validTOTPCode(t) {
			wrongCode = "111111"
		}

		ctx := i18n.SetLocale(context.Background(), model.LocaleJa)
		err := v.Validate(ctx, validator.SettingsTwoFactorAuthCreateValidatorInput{UserID: userID, Code: wrongCode})

		ve := model.AsValidationError(err)
		if ve == nil {
			t.Fatalf("Validate()のエラー = %v、期待値 = *ValidationError", err)
		}
		if !ve.HasFieldError("code") {
			t.Error("codeフィールドのエラーが無い")
		}
	})
}

// newTwoFactorAuthDeleteValidatorはテスト専用のデータベース上に
// SettingsTwoFactorAuthDeleteValidatorを作り、資格情報の所有ユーザーを作成する。
// サブテストが有効な2FA設定とパスワードを投入できるよう、validatorとユーザーIDを返す。
func newTwoFactorAuthDeleteValidator(t *testing.T, db *database.DB) (*validator.SettingsTwoFactorAuthDeleteValidator, model.UserID) {
	t.Helper()
	userID := testutil.NewUserBuilder(t, db).Build()
	userPasswordRepo := repository.NewUserPasswordRepository(db)
	twoFactorRepo := repository.NewUserTwoFactorAuthRepository(db)
	return validator.NewSettingsTwoFactorAuthDeleteValidator(userPasswordRepo, twoFactorRepo), userID
}

// TestSettingsTwoFactorAuthDeleteValidator_Validateは無効化フォームの再認証を網羅する。
// 正しい現在のパスワードか正しい現在のTOTPコードは通り、どちらも未入力はフォーム全体の
// 「いずれかを入力」エラー、誤ったパスワードや誤ったコードはフォーム全体の「正しくない」エラーに
// なる。
func TestSettingsTwoFactorAuthDeleteValidator_Validate(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	t.Run("正常系: 正しい現在のパスワード", func(t *testing.T) {
		t.Parallel()
		v, userID := newTwoFactorAuthDeleteValidator(t, db)
		testutil.NewUserTwoFactorAuthBuilder(t, db).WithUserID(userID).WithEnabled(true).Build()
		testutil.NewUserPasswordBuilder(t, db).WithUserID(userID).Build()

		ctx := i18n.SetLocale(context.Background(), model.LocaleJa)
		err := v.Validate(ctx, validator.SettingsTwoFactorAuthDeleteValidatorInput{
			UserID:          userID,
			CurrentPassword: testutil.DefaultBuilderPassword,
		})
		if err != nil {
			t.Fatalf("Validate()のエラー = %v、期待値 = nil", err)
		}
	})

	t.Run("正常系: 正しいTOTPコード", func(t *testing.T) {
		t.Parallel()
		v, userID := newTwoFactorAuthDeleteValidator(t, db)
		testutil.NewUserTwoFactorAuthBuilder(t, db).WithUserID(userID).WithEnabled(true).Build()

		ctx := i18n.SetLocale(context.Background(), model.LocaleJa)
		err := v.Validate(ctx, validator.SettingsTwoFactorAuthDeleteValidatorInput{
			UserID: userID,
			Code:   validTOTPCode(t),
		})
		if err != nil {
			t.Fatalf("Validate()のエラー = %v、期待値 = nil", err)
		}
	})

	t.Run("異常系: どちらも未入力", func(t *testing.T) {
		t.Parallel()
		v, userID := newTwoFactorAuthDeleteValidator(t, db)
		testutil.NewUserTwoFactorAuthBuilder(t, db).WithUserID(userID).WithEnabled(true).Build()

		ctx := i18n.SetLocale(context.Background(), model.LocaleJa)
		err := v.Validate(ctx, validator.SettingsTwoFactorAuthDeleteValidatorInput{UserID: userID})

		ve := model.AsValidationError(err)
		if ve == nil {
			t.Fatalf("Validate()のエラー = %v、期待値 = *ValidationError", err)
		}
		if !ve.HasGlobalError() {
			t.Error("フォーム全体のエラーが無い")
		}
	})

	t.Run("異常系: 誤った現在のパスワード", func(t *testing.T) {
		t.Parallel()
		v, userID := newTwoFactorAuthDeleteValidator(t, db)
		testutil.NewUserTwoFactorAuthBuilder(t, db).WithUserID(userID).WithEnabled(true).Build()
		testutil.NewUserPasswordBuilder(t, db).WithUserID(userID).Build()

		ctx := i18n.SetLocale(context.Background(), model.LocaleJa)
		err := v.Validate(ctx, validator.SettingsTwoFactorAuthDeleteValidatorInput{
			UserID:          userID,
			CurrentPassword: "wrongpassword",
		})

		ve := model.AsValidationError(err)
		if ve == nil {
			t.Fatalf("Validate()のエラー = %v、期待値 = *ValidationError", err)
		}
		if !ve.HasGlobalError() {
			t.Error("フォーム全体のエラーが無い")
		}
	})

	t.Run("異常系: 誤ったTOTPコード", func(t *testing.T) {
		t.Parallel()
		v, userID := newTwoFactorAuthDeleteValidator(t, db)
		testutil.NewUserTwoFactorAuthBuilder(t, db).WithUserID(userID).WithEnabled(true).Build()

		// 整った形式で、意図的に現在のコードではない値。形式の問題ではない不一致を検証する。
		wrongCode := "000000"
		if wrongCode == validTOTPCode(t) {
			wrongCode = "111111"
		}

		ctx := i18n.SetLocale(context.Background(), model.LocaleJa)
		err := v.Validate(ctx, validator.SettingsTwoFactorAuthDeleteValidatorInput{UserID: userID, Code: wrongCode})

		ve := model.AsValidationError(err)
		if ve == nil {
			t.Fatalf("Validate()のエラー = %v、期待値 = *ValidationError", err)
		}
		if !ve.HasGlobalError() {
			t.Error("フォーム全体のエラーが無い")
		}
	})
}
