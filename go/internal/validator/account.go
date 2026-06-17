package validator

import (
	"context"
	"errors"

	"github.com/groobb/groobb/go/internal/auth"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
)

// AccountCreateValidator validates the account-creation form: the chosen
// password meets the strength policy and the confirmation field matches it. It
// holds no dependencies because every check is a pure format check (no database
// state): the email is not validated here since it comes from a verified
// confirmation, not user input.
//
// [Ja] AccountCreateValidator はアカウント作成フォームを検証します。選んだパスワードが
// 強度ポリシーを満たし、確認フィールドが一致することです。すべて形式チェック (DB の状態を
// 見ない) のため依存は持ちません。email はユーザー入力ではなく検証済みの確認から来るため、
// ここでは検証しません。
type AccountCreateValidator struct{}

// NewAccountCreateValidator creates an AccountCreateValidator.
//
// [Ja] NewAccountCreateValidator は AccountCreateValidator を生成します。
func NewAccountCreateValidator() *AccountCreateValidator {
	return &AccountCreateValidator{}
}

// AccountCreateValidatorInput is the input to AccountCreateValidator.Validate.
//
// [Ja] AccountCreateValidatorInput は AccountCreateValidator.Validate の入力です。
type AccountCreateValidatorInput struct {
	Password             string
	PasswordConfirmation string
}

// Validate checks the password and its confirmation, returning a
// *model.ValidationError when either is invalid. An empty password reports a
// required error; otherwise the strength policy (length) is enforced via the
// auth sentinel errors, translated here. The confirmation must be present and
// equal to the password, with the mismatch reported on the confirmation field.
//
// [Ja] Validate はパスワードとその確認を検証し、いずれかが不正なら
// *model.ValidationError を返します。空パスワードは必須エラーを、そうでなければ強度
// ポリシー (長さ) を auth の sentinel error 経由で強制し、ここで翻訳します。確認は
// 入力必須でパスワードと等しい必要があり、不一致は確認フィールドに報告します。
func (v *AccountCreateValidator) Validate(ctx context.Context, input AccountCreateValidatorInput) error {
	ve := model.NewValidationError()

	if input.Password == "" {
		ve.AddField("password", i18n.T(ctx, "validation_required"))
	} else {
		switch err := auth.ValidatePasswordStrength(input.Password); {
		case errors.Is(err, auth.ErrPasswordTooShort):
			ve.AddField("password", i18n.T(ctx, "validation_password_too_short"))
		case errors.Is(err, auth.ErrPasswordTooLong):
			ve.AddField("password", i18n.T(ctx, "validation_password_too_long"))
		}
	}

	// Check the confirmation only against a non-empty password: an empty password
	// already reported "required", and flagging a mismatch on top would be noise.
	//
	// [Ja] 確認は空でないパスワードに対してのみ照合する。空パスワードは既に「必須」を
	// 報告済みで、その上に不一致まで出すのはノイズになる。
	if input.PasswordConfirmation == "" {
		ve.AddField("password_confirmation", i18n.T(ctx, "validation_required"))
	} else if input.Password != "" && input.Password != input.PasswordConfirmation {
		ve.AddField("password_confirmation", i18n.T(ctx, "validation_password_mismatch"))
	}

	if ve.HasErrors() {
		return ve
	}

	return nil
}
