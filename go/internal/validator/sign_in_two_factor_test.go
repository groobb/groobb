package validator_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pquerna/otp/totp"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/query"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/validator"
)

// newSignInTwoFactorValidator builds a SignInTwoFactorCreateValidator over the
// test transaction so its 2FA lookup runs inside the rolled-back transaction.
//
// [Ja] newSignInTwoFactorValidator はテスト用トランザクション上に
// SignInTwoFactorCreateValidator を組み立て、その 2FA ルックアップがロールバックされる
// トランザクション内で走るようにする。
func newSignInTwoFactorValidator(t *testing.T, db *pgxpool.Pool, tx pgx.Tx) *validator.SignInTwoFactorCreateValidator {
	t.Helper()
	repo := repository.NewUserTwoFactorAuthRepository(query.New(db)).WithTx(tx)
	return validator.NewSignInTwoFactorCreateValidator(repo)
}

// TestSignInTwoFactorCreateValidator_Validate_Success verifies that a correct TOTP
// code for a user with enabled 2FA passes with no error.
//
// [Ja] TestSignInTwoFactorCreateValidator_Validate_Success は、2FA が有効なユーザーの
// 正しい TOTP コードがエラーなしで通ることを検証する。
func TestSignInTwoFactorCreateValidator_Validate_Success(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)

	userID := testutil.NewUserBuilder(t, tx).WithEmail("2fa-v@example.com").Build()
	testutil.NewUserTwoFactorAuthBuilder(t, tx).WithUserID(userID).WithEnabled(true).Build()

	code, err := totp.GenerateCode(testutil.DefaultBuilderTOTPSecret, time.Now())
	if err != nil {
		t.Fatalf("テスト用 TOTP コードの生成に失敗: %v", err)
	}

	v := newSignInTwoFactorValidator(t, db, tx)
	if err := v.Validate(ctx, validator.SignInTwoFactorCreateValidatorInput{UserID: userID, Code: code}); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}

// TestSignInTwoFactorCreateValidator_Validate_FieldErrors verifies that missing,
// malformed, and incorrect codes each surface as a code-field ValidationError,
// leaving the challenge unpassed.
//
// [Ja] TestSignInTwoFactorCreateValidator_Validate_FieldErrors は、コードの未入力・
// 形式不正・不一致がそれぞれ code フィールドの ValidationError として表れ、チャレンジが
// 通らないことを検証する。
func TestSignInTwoFactorCreateValidator_Validate_FieldErrors(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)

	userID := testutil.NewUserBuilder(t, tx).WithEmail("2fa-v-bad@example.com").Build()
	testutil.NewUserTwoFactorAuthBuilder(t, tx).WithUserID(userID).WithEnabled(true).Build()

	validCode, err := totp.GenerateCode(testutil.DefaultBuilderTOTPSecret, time.Now())
	if err != nil {
		t.Fatalf("テスト用 TOTP コードの生成に失敗: %v", err)
	}
	// A well-formed code deliberately not equal to the current one.
	//
	// [Ja] 整った形式で、意図的に現在のコードと等しくない値。
	wrongCode := "000000"
	if wrongCode == validCode {
		wrongCode = "111111"
	}

	tests := []struct {
		name string
		code string
	}{
		{name: "未入力", code: ""},
		{name: "形式不正", code: "abc"},
		{name: "不一致", code: wrongCode},
	}

	v := newSignInTwoFactorValidator(t, db, tx)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := v.Validate(ctx, validator.SignInTwoFactorCreateValidatorInput{UserID: userID, Code: tt.code})
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

// TestSignInTwoFactorCreateValidator_Validate_NoEnabledTwoFactor verifies that a
// pending user with no enabled 2FA (an enrolling-only row, or none at all) fails
// with a form-wide error, so a stale or forged cookie cannot pass the challenge.
//
// [Ja] TestSignInTwoFactorCreateValidator_Validate_NoEnabledTwoFactor は、有効な 2FA を
// 持たない保留中ユーザー (登録中のみの行、または全く無い) がフォーム全体のエラーで失敗し、
// 失効・偽造した Cookie がチャレンジを通せないことを検証する。
func TestSignInTwoFactorCreateValidator_Validate_NoEnabledTwoFactor(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)

	// A user whose 2FA is only enrolling (not enabled) counts as no enabled 2FA, so
	// the still-well-formed code cannot be accepted.
	//
	// [Ja] 2FA が登録中 (未有効化) のみのユーザーは有効な 2FA 無しと数えるため、形式の
	// 整ったコードでも受理できない。
	userID := testutil.NewUserBuilder(t, tx).WithEmail("2fa-v-none@example.com").Build()
	testutil.NewUserTwoFactorAuthBuilder(t, tx).WithUserID(userID).Build()

	code, err := totp.GenerateCode(testutil.DefaultBuilderTOTPSecret, time.Now())
	if err != nil {
		t.Fatalf("テスト用 TOTP コードの生成に失敗: %v", err)
	}

	v := newSignInTwoFactorValidator(t, db, tx)
	err = v.Validate(ctx, validator.SignInTwoFactorCreateValidatorInput{UserID: userID, Code: code})
	ve := model.AsValidationError(err)
	if ve == nil {
		t.Fatalf("Validate() error = %v, want *model.ValidationError", err)
	}
	if !ve.HasGlobalError() {
		t.Error("フォーム全体のエラーが無い (有効な 2FA が無いチャレンジはフォーム全体で失敗すべき)")
	}
}
