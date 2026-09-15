package validator_test

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/validator"
)

// seededRecoveryCodesはバリデーターテストが登録する既知のリカバリーコードで、
// それぞれバリデーターが受理する8文字の小文字英数字の形式です。
var seededRecoveryCodes = []string{"abcd1234", "efgh5678"}

// newSignInTwoFactorRecoveryValidatorはテスト専用のデータベース上に
// SignInTwoFactorRecoveryCreateValidatorを組み立て、その2FAルックアップがそこへ
// 仕込んだ行を読むようにする。
func newSignInTwoFactorRecoveryValidator(t *testing.T, db *database.DB) *validator.SignInTwoFactorRecoveryCreateValidator {
	t.Helper()
	repo := repository.NewUserTwoFactorAuthRepository(db)
	return validator.NewSignInTwoFactorRecoveryCreateValidator(repo)
}

// TestSignInTwoFactorRecoveryCreateValidator_Validate_Successは、2FAが有効な
// ユーザーの保存済みリカバリーコードが通り、解決した設定を返す (UseCaseが使用済みコードを
// 消費できるよう) ことを検証する。
func TestSignInTwoFactorRecoveryCreateValidator_Validate_Success(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

	userID := testutil.NewUserBuilder(t, db).WithEmail("2fa-rc-v@example.com").Build()
	testutil.NewUserTwoFactorAuthBuilder(t, db).
		WithUserID(userID).
		WithEnabled(true).
		WithRecoveryCodes(seededRecoveryCodes).
		Build()

	v := newSignInTwoFactorRecoveryValidator(t, db)
	out, err := v.Validate(ctx, validator.SignInTwoFactorRecoveryCreateValidatorInput{UserID: userID, Code: "abcd1234"})
	if err != nil {
		t.Fatalf("Validate()のエラー = %v、期待値 = nil", err)
	}
	if out == nil {
		t.Fatal("Validate() が設定を返していない (成功時は解決した設定を返すべき)")
	}
	if out.UserID != userID {
		t.Errorf("out.UserID = %v、期待値 = %v", out.UserID, userID)
	}
}

// TestSignInTwoFactorRecoveryCreateValidator_Validate_FieldErrorsは、コードの
// 未入力・形式不正・未知 (形式は整うが未保存) がそれぞれcodeフィールドの
// ValidationErrorとして表れ、チャレンジが通らないことを検証する。
func TestSignInTwoFactorRecoveryCreateValidator_Validate_FieldErrors(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

	userID := testutil.NewUserBuilder(t, db).WithEmail("2fa-rc-v-bad@example.com").Build()
	testutil.NewUserTwoFactorAuthBuilder(t, db).
		WithUserID(userID).
		WithEnabled(true).
		WithRecoveryCodes(seededRecoveryCodes).
		Build()

	tests := []struct {
		name string
		code string
	}{
		{name: "未入力", code: ""},
		// 大文字は意図的に形式不正: 保存済みコードは小文字のため、配列内存在の
		// 比較より前に形式正規表現が弾く。
		{name: "形式不正", code: "ABCD1234"},
		{name: "未知のコード", code: "zzzz9999"},
	}

	v := newSignInTwoFactorRecoveryValidator(t, db)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			out, err := v.Validate(ctx, validator.SignInTwoFactorRecoveryCreateValidatorInput{UserID: userID, Code: tt.code})
			if out != nil {
				t.Error("失敗時は設定を返すべきでない")
			}
			ve := model.AsValidationError(err)
			if ve == nil {
				t.Fatalf("Validate()のエラー = %v、期待値 = *model.ValidationError", err)
			}
			if !ve.HasFieldError("code") {
				t.Error("codeフィールドのエラーが無い")
			}
		})
	}
}

// TestSignInTwoFactorRecoveryCreateValidator_Validate_NoEnabledTwoFactorは、
// 有効な2FAを持たない保留中ユーザー (登録中のみの行、または全く無い) がフォーム全体の
// エラーで失敗し、失効・偽造したCookieがチャレンジを通せないことを検証する。
func TestSignInTwoFactorRecoveryCreateValidator_Validate_NoEnabledTwoFactor(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

	// 2FAが登録中 (未有効化) のみのユーザーは有効な2FA無しと数えるため、形式の
	// 整ったコードでも受理できない。
	userID := testutil.NewUserBuilder(t, db).WithEmail("2fa-rc-v-none@example.com").Build()
	testutil.NewUserTwoFactorAuthBuilder(t, db).WithUserID(userID).Build()

	v := newSignInTwoFactorRecoveryValidator(t, db)
	out, err := v.Validate(ctx, validator.SignInTwoFactorRecoveryCreateValidatorInput{UserID: userID, Code: "abcd1234"})
	if out != nil {
		t.Error("有効な2FAが無いのに設定を返している")
	}
	ve := model.AsValidationError(err)
	if ve == nil {
		t.Fatalf("Validate()のエラー = %v、期待値 = *model.ValidationError", err)
	}
	if !ve.HasGlobalError() {
		t.Error("フォーム全体のエラーが無い (有効な2FAが無いチャレンジはフォーム全体で失敗すべき)")
	}
}
