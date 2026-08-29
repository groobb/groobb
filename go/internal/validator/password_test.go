package validator_test

import (
	"context"
	"testing"
	"time"

	"github.com/groobb/groobb/go/internal/auth"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/validator"
)

// TestPasswordUpdateValidator_Validate_Success verifies that a usable token plus
// a valid, matching password passes and returns the token's id and the user it
// resets.
//
// [Ja] TestPasswordUpdateValidator_Validate_Success は、使えるトークンと有効で一致する
// パスワードが通過し、トークンの id とリセット対象のユーザーを返すことを検証する。
func TestPasswordUpdateValidator_Validate_Success(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	v := validator.NewPasswordUpdateValidator(repository.NewPasswordResetTokenRepository(db))
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

	userID := testutil.NewUserBuilder(t, db).Build()
	rawToken := "valid-raw-token"
	tokenID := testutil.NewPasswordResetTokenBuilder(t, db).
		WithUserID(userID).
		WithTokenDigest(auth.HashToken(rawToken)).
		Build()

	out, err := v.Validate(ctx, validator.PasswordUpdateValidatorInput{
		Token:                rawToken,
		Password:             "newpassword123",
		PasswordConfirmation: "newpassword123",
	})
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if out == nil {
		t.Fatal("Validate() output = nil")
	}
	if out.TokenID != tokenID {
		t.Errorf("out.TokenID = %v, want %v", out.TokenID, tokenID)
	}
	if out.UserID != userID {
		t.Errorf("out.UserID = %v, want %v", out.UserID, userID)
	}
}

// TestPasswordUpdateValidator_Validate_FormatErrors verifies the format checks:
// an empty token is a form-wide error, and password problems (empty, too short,
// mismatch) are field errors. A format failure short-circuits before any token
// lookup, so these cases need no seeded token.
//
// [Ja] TestPasswordUpdateValidator_Validate_FormatErrors は形式チェックを検証する。
// 空トークンはフォーム全体のエラー、パスワードの問題 (空・短すぎ・不一致) はフィールド
// エラーである。形式の失敗はトークンルックアップの前に短絡するため、これらはトークンの
// 仕込みを要しない。
func TestPasswordUpdateValidator_Validate_FormatErrors(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	v := validator.NewPasswordUpdateValidator(repository.NewPasswordResetTokenRepository(db))
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

	tests := []struct {
		name       string
		input      validator.PasswordUpdateValidatorInput
		wantGlobal bool
		wantField  string
	}{
		{
			name:       "トークンが空ならフォーム全体のエラー",
			input:      validator.PasswordUpdateValidatorInput{Token: "", Password: "newpassword123", PasswordConfirmation: "newpassword123"},
			wantGlobal: true,
		},
		{
			name:      "パスワードが空ならフィールドエラー",
			input:     validator.PasswordUpdateValidatorInput{Token: "tok", Password: "", PasswordConfirmation: ""},
			wantField: "password",
		},
		{
			name:      "パスワードが短すぎるとフィールドエラー",
			input:     validator.PasswordUpdateValidatorInput{Token: "tok", Password: "short", PasswordConfirmation: "short"},
			wantField: "password",
		},
		{
			name:      "確認が一致しないとフィールドエラー",
			input:     validator.PasswordUpdateValidatorInput{Token: "tok", Password: "newpassword123", PasswordConfirmation: "different456"},
			wantField: "password_confirmation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			out, err := v.Validate(ctx, tt.input)
			if out != nil {
				t.Errorf("Validate() output = %v, want nil", out)
			}
			ve := model.AsValidationError(err)
			if ve == nil {
				t.Fatalf("Validate() error = %v, want *model.ValidationError", err)
			}
			if tt.wantGlobal && !ve.HasGlobalError() {
				t.Error("フォーム全体のエラーを期待したが無い")
			}
			if tt.wantField != "" && !ve.HasFieldError(tt.wantField) {
				t.Errorf("フィールド %q のエラーを期待したが無い", tt.wantField)
			}
		})
	}
}

// TestPasswordUpdateValidator_Validate_TokenStates verifies that an unknown,
// already-used, or expired token each fails as a form-wide error even when the
// password itself is valid, so the user is told the link is unusable.
//
// [Ja] TestPasswordUpdateValidator_Validate_TokenStates は、パスワード自体が有効でも、
// 未知・使用済み・期限切れのトークンがそれぞれフォーム全体のエラーで失敗し、リンクが
// 使えないことをユーザーに伝えることを検証する。
func TestPasswordUpdateValidator_Validate_TokenStates(t *testing.T) {
	t.Parallel()

	t.Run("未知のトークンはフォーム全体のエラー", func(t *testing.T) {
		t.Parallel()
		db := testutil.SetupDB(t)
		v := validator.NewPasswordUpdateValidator(repository.NewPasswordResetTokenRepository(db))
		ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

		out, err := v.Validate(ctx, validator.PasswordUpdateValidatorInput{
			Token:                "no-such-token",
			Password:             "newpassword123",
			PasswordConfirmation: "newpassword123",
		})
		assertGlobalTokenError(t, out, err)
	})

	t.Run("使用済みトークンはフォーム全体のエラー", func(t *testing.T) {
		t.Parallel()
		db := testutil.SetupDB(t)
		v := validator.NewPasswordUpdateValidator(repository.NewPasswordResetTokenRepository(db))
		ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

		userID := testutil.NewUserBuilder(t, db).Build()
		rawToken := "used-raw-token"
		testutil.NewPasswordResetTokenBuilder(t, db).
			WithUserID(userID).
			WithTokenDigest(auth.HashToken(rawToken)).
			WithUsedAt(time.Now()).
			Build()

		out, err := v.Validate(ctx, validator.PasswordUpdateValidatorInput{
			Token:                rawToken,
			Password:             "newpassword123",
			PasswordConfirmation: "newpassword123",
		})
		assertGlobalTokenError(t, out, err)
	})

	t.Run("期限切れトークンはフォーム全体のエラー", func(t *testing.T) {
		t.Parallel()
		db := testutil.SetupDB(t)
		v := validator.NewPasswordUpdateValidator(repository.NewPasswordResetTokenRepository(db))
		ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

		userID := testutil.NewUserBuilder(t, db).Build()
		rawToken := "expired-raw-token"
		testutil.NewPasswordResetTokenBuilder(t, db).
			WithUserID(userID).
			WithTokenDigest(auth.HashToken(rawToken)).
			WithExpiresAt(time.Now().Add(-time.Hour)).
			Build()

		out, err := v.Validate(ctx, validator.PasswordUpdateValidatorInput{
			Token:                rawToken,
			Password:             "newpassword123",
			PasswordConfirmation: "newpassword123",
		})
		assertGlobalTokenError(t, out, err)
	})
}

// assertGlobalTokenError asserts that Validate returned no output and a
// *model.ValidationError carrying a form-wide (token) error.
//
// [Ja] assertGlobalTokenError は Validate が出力を返さず、フォーム全体の (トークン)
// エラーを持つ *model.ValidationError を返したことをアサートする。
func assertGlobalTokenError(t *testing.T, out *validator.PasswordUpdateValidateOutput, err error) {
	t.Helper()

	if out != nil {
		t.Errorf("Validate() output = %v, want nil", out)
	}
	ve := model.AsValidationError(err)
	if ve == nil {
		t.Fatalf("Validate() error = %v, want *model.ValidationError", err)
	}
	if !ve.HasGlobalError() {
		t.Error("トークンエラーはフォーム全体のエラーで報告されるはず")
	}
}
