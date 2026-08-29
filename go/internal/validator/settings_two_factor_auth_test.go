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

// newTwoFactorAuthValidator builds a SettingsTwoFactorAuthCreateValidator over the
// test's own database and creates a user to own the 2FA setting, returning the
// validator and the user ID so a subtest can seed the enrollment row for that
// user.
//
// [Ja] newTwoFactorAuthValidator はテスト専用のデータベース上に
// SettingsTwoFactorAuthCreateValidator を作り、2FA 設定の所有ユーザーを作成する。
// サブテストがそのユーザーの登録行を投入できるよう、validator とユーザー ID を返す。
func newTwoFactorAuthValidator(t *testing.T, db *database.DB) (*validator.SettingsTwoFactorAuthCreateValidator, model.UserID) {
	t.Helper()
	userID := testutil.NewUserBuilder(t, db).Build()
	repo := repository.NewUserTwoFactorAuthRepository(db)
	return validator.NewSettingsTwoFactorAuthCreateValidator(repo), userID
}

// validTOTPCode returns the current TOTP code for DefaultBuilderTOTPSecret, the
// secret a not-yet-enabled enrollment row is seeded with. The ±1 step skew that
// ValidateTOTPCode allows keeps this accepted even across a step boundary between
// generation and verification.
//
// [Ja] validTOTPCode は DefaultBuilderTOTPSecret (未有効化の登録行に投入される secret) に
// 対する現在の TOTP コードを返す。ValidateTOTPCode が許容する ±1 ステップのスキューにより、
// 生成と検証の間でステップ境界を跨いでもこのコードは受理される。
func validTOTPCode(t *testing.T) string {
	t.Helper()
	code, err := totp.GenerateCode(testutil.DefaultBuilderTOTPSecret, time.Now())
	if err != nil {
		t.Fatalf("テスト用 TOTP コードの生成に失敗: %v", err)
	}
	return code
}

// TestSettingsTwoFactorAuthCreateValidator_Validate covers the enable form's
// validation: a correct code against an in-progress enrollment passes; a missing or
// malformed code is a code-field error; no in-progress enrollment (none, or already
// enabled) is a form-wide error; and a well-formed code that does not match the
// secret is a code-field error.
//
// [Ja] TestSettingsTwoFactorAuthCreateValidator_Validate は有効化フォームの検証を網羅する。
// 登録中の設定に対する正しいコードは通り、未入力・不正な形式のコードは code フィールドの
// エラー、登録中の設定が無い (未存在、または既に有効) はフォーム全体のエラー、secret と
// 一致しない整った形式のコードは code フィールドのエラーになる。
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
			t.Fatalf("Validate() error = %v, want nil", err)
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
			t.Fatalf("Validate() error = %v, want *ValidationError", err)
		}
		if !ve.HasFieldError("code") {
			t.Error("code フィールドのエラーが無い")
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
			t.Fatalf("Validate() error = %v, want *ValidationError", err)
		}
		if !ve.HasFieldError("code") {
			t.Error("code フィールドのエラーが無い")
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
			t.Fatalf("Validate() error = %v, want *ValidationError", err)
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
			t.Fatalf("Validate() error = %v, want *ValidationError", err)
		}
		if !ve.HasGlobalError() {
			t.Error("フォーム全体のエラーが無い")
		}
	})

	t.Run("異常系: コードが secret と一致しない", func(t *testing.T) {
		t.Parallel()
		v, userID := newTwoFactorAuthValidator(t, db)
		testutil.NewUserTwoFactorAuthBuilder(t, db).WithUserID(userID).Build()

		// Pick a well-formed code that is deliberately not the current one, so the
		// mismatch (not a format problem) is exercised.
		//
		// [Ja] 整った形式で、意図的に現在のコードではない値を選び、形式の問題ではない不一致を
		// 検証する。
		wrongCode := "000000"
		if wrongCode == validTOTPCode(t) {
			wrongCode = "111111"
		}

		ctx := i18n.SetLocale(context.Background(), model.LocaleJa)
		err := v.Validate(ctx, validator.SettingsTwoFactorAuthCreateValidatorInput{UserID: userID, Code: wrongCode})

		ve := model.AsValidationError(err)
		if ve == nil {
			t.Fatalf("Validate() error = %v, want *ValidationError", err)
		}
		if !ve.HasFieldError("code") {
			t.Error("code フィールドのエラーが無い")
		}
	})
}

// newTwoFactorAuthDeleteValidator builds a SettingsTwoFactorAuthDeleteValidator over
// the test's own database and creates a user to own the credentials, returning the
// validator and the user ID so a subtest can seed the enabled 2FA setting and a
// password for that user.
//
// [Ja] newTwoFactorAuthDeleteValidator はテスト専用のデータベース上に
// SettingsTwoFactorAuthDeleteValidator を作り、資格情報の所有ユーザーを作成する。
// サブテストが有効な 2FA 設定とパスワードを投入できるよう、validator とユーザー ID を返す。
func newTwoFactorAuthDeleteValidator(t *testing.T, db *database.DB) (*validator.SettingsTwoFactorAuthDeleteValidator, model.UserID) {
	t.Helper()
	userID := testutil.NewUserBuilder(t, db).Build()
	userPasswordRepo := repository.NewUserPasswordRepository(db)
	twoFactorRepo := repository.NewUserTwoFactorAuthRepository(db)
	return validator.NewSettingsTwoFactorAuthDeleteValidator(userPasswordRepo, twoFactorRepo), userID
}

// TestSettingsTwoFactorAuthDeleteValidator_Validate covers the disable form's
// re-authentication: the correct current password or a correct current TOTP code
// passes; neither provided is a form-wide "enter one" error; and a wrong password or
// a wrong code is a form-wide "incorrect" error.
//
// [Ja] TestSettingsTwoFactorAuthDeleteValidator_Validate は無効化フォームの再認証を網羅する。
// 正しい現在のパスワードか正しい現在の TOTP コードは通り、どちらも未入力はフォーム全体の
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
			t.Fatalf("Validate() error = %v, want nil", err)
		}
	})

	t.Run("正常系: 正しい TOTP コード", func(t *testing.T) {
		t.Parallel()
		v, userID := newTwoFactorAuthDeleteValidator(t, db)
		testutil.NewUserTwoFactorAuthBuilder(t, db).WithUserID(userID).WithEnabled(true).Build()

		ctx := i18n.SetLocale(context.Background(), model.LocaleJa)
		err := v.Validate(ctx, validator.SettingsTwoFactorAuthDeleteValidatorInput{
			UserID: userID,
			Code:   validTOTPCode(t),
		})
		if err != nil {
			t.Fatalf("Validate() error = %v, want nil", err)
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
			t.Fatalf("Validate() error = %v, want *ValidationError", err)
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
			t.Fatalf("Validate() error = %v, want *ValidationError", err)
		}
		if !ve.HasGlobalError() {
			t.Error("フォーム全体のエラーが無い")
		}
	})

	t.Run("異常系: 誤った TOTP コード", func(t *testing.T) {
		t.Parallel()
		v, userID := newTwoFactorAuthDeleteValidator(t, db)
		testutil.NewUserTwoFactorAuthBuilder(t, db).WithUserID(userID).WithEnabled(true).Build()

		// A well-formed code deliberately not equal to the current one, so the
		// mismatch (not a format problem) is exercised.
		//
		// [Ja] 整った形式で、意図的に現在のコードではない値。形式の問題ではない不一致を検証する。
		wrongCode := "000000"
		if wrongCode == validTOTPCode(t) {
			wrongCode = "111111"
		}

		ctx := i18n.SetLocale(context.Background(), model.LocaleJa)
		err := v.Validate(ctx, validator.SettingsTwoFactorAuthDeleteValidatorInput{UserID: userID, Code: wrongCode})

		ve := model.AsValidationError(err)
		if ve == nil {
			t.Fatalf("Validate() error = %v, want *ValidationError", err)
		}
		if !ve.HasGlobalError() {
			t.Error("フォーム全体のエラーが無い")
		}
	})
}
