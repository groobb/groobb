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

// seededRecoveryCodes are the known recovery codes a validator test enrolls, each
// in the eight-lowercase-alphanumeric format the validator accepts.
//
// [Ja] seededRecoveryCodes はバリデーターテストが登録する既知のリカバリーコードで、
// それぞれバリデーターが受理する 8 文字の小文字英数字の形式です。
var seededRecoveryCodes = []string{"abcd1234", "efgh5678"}

// newSignInTwoFactorRecoveryValidator builds a
// SignInTwoFactorRecoveryCreateValidator over the test's own database so its 2FA
// lookup reads the rows the test seeded there.
//
// [Ja] newSignInTwoFactorRecoveryValidator はテスト専用のデータベース上に
// SignInTwoFactorRecoveryCreateValidator を組み立て、その 2FA ルックアップがそこへ
// 仕込んだ行を読むようにする。
func newSignInTwoFactorRecoveryValidator(t *testing.T, db *database.DB) *validator.SignInTwoFactorRecoveryCreateValidator {
	t.Helper()
	repo := repository.NewUserTwoFactorAuthRepository(db)
	return validator.NewSignInTwoFactorRecoveryCreateValidator(repo)
}

// TestSignInTwoFactorRecoveryCreateValidator_Validate_Success verifies that a
// stored recovery code for a user with enabled 2FA passes and returns the resolved
// setting (so the UseCase can consume the used code).
//
// [Ja] TestSignInTwoFactorRecoveryCreateValidator_Validate_Success は、2FA が有効な
// ユーザーの保存済みリカバリーコードが通り、解決した設定を返す (UseCase が使用済みコードを
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
		t.Fatalf("Validate() error = %v, want nil", err)
	}
	if out == nil {
		t.Fatal("Validate() が設定を返していない (成功時は解決した設定を返すべき)")
	}
	if out.UserID != userID {
		t.Errorf("out.UserID = %v, want %v", out.UserID, userID)
	}
}

// TestSignInTwoFactorRecoveryCreateValidator_Validate_FieldErrors verifies that
// missing, malformed, and unknown (well-formed but not stored) codes each surface
// as a code-field ValidationError, leaving the challenge unpassed.
//
// [Ja] TestSignInTwoFactorRecoveryCreateValidator_Validate_FieldErrors は、コードの
// 未入力・形式不正・未知 (形式は整うが未保存) がそれぞれ code フィールドの
// ValidationError として表れ、チャレンジが通らないことを検証する。
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
		// Uppercase is deliberately malformed: stored codes are lowercase, so the
		// format regex rejects it before any membership comparison.
		//
		// [Ja] 大文字は意図的に形式不正: 保存済みコードは小文字のため、配列内存在の
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
				t.Fatalf("Validate() error = %v, want *model.ValidationError", err)
			}
			if !ve.HasFieldError("code") {
				t.Error("code フィールドのエラーが無い")
			}
		})
	}
}

// TestSignInTwoFactorRecoveryCreateValidator_Validate_NoEnabledTwoFactor verifies
// that a pending user with no enabled 2FA (an enrolling-only row, or none at all)
// fails with a form-wide error, so a stale or forged cookie cannot pass the
// challenge.
//
// [Ja] TestSignInTwoFactorRecoveryCreateValidator_Validate_NoEnabledTwoFactor は、
// 有効な 2FA を持たない保留中ユーザー (登録中のみの行、または全く無い) がフォーム全体の
// エラーで失敗し、失効・偽造した Cookie がチャレンジを通せないことを検証する。
func TestSignInTwoFactorRecoveryCreateValidator_Validate_NoEnabledTwoFactor(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

	// A user whose 2FA is only enrolling (not enabled) counts as no enabled 2FA, so
	// the still-well-formed code cannot be accepted.
	//
	// [Ja] 2FA が登録中 (未有効化) のみのユーザーは有効な 2FA 無しと数えるため、形式の
	// 整ったコードでも受理できない。
	userID := testutil.NewUserBuilder(t, db).WithEmail("2fa-rc-v-none@example.com").Build()
	testutil.NewUserTwoFactorAuthBuilder(t, db).WithUserID(userID).Build()

	v := newSignInTwoFactorRecoveryValidator(t, db)
	out, err := v.Validate(ctx, validator.SignInTwoFactorRecoveryCreateValidatorInput{UserID: userID, Code: "abcd1234"})
	if out != nil {
		t.Error("有効な 2FA が無いのに設定を返している")
	}
	ve := model.AsValidationError(err)
	if ve == nil {
		t.Fatalf("Validate() error = %v, want *model.ValidationError", err)
	}
	if !ve.HasGlobalError() {
		t.Error("フォーム全体のエラーが無い (有効な 2FA が無いチャレンジはフォーム全体で失敗すべき)")
	}
}
