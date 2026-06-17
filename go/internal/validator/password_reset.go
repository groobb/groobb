package validator

import (
	"context"
	"net/mail"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
)

// PasswordResetCreateValidator validates the password reset request form: that
// the email is present and well-formed. It deliberately does NOT check whether an
// account with that email exists. Revealing existence here would let an attacker
// enumerate registered addresses, so the existence check is skipped and the
// UseCase quietly does nothing when the email is unknown while the response stays
// the same. It needs no repository for the same reason.
//
// [Ja] PasswordResetCreateValidator はパスワードリセット申請フォームを検証します。
// email が入力され、形式が正しいことを確認します。そのメールのアカウントが存在するかは
// 意図的に検証しません。ここで存在を明かすと攻撃者が登録済みアドレスを列挙できてしまうため、
// 存在チェックは省き、メールが未知のときは UseCase が静かに何もせず、レスポンスは同じに
// 保ちます。同じ理由でリポジトリも不要です。
type PasswordResetCreateValidator struct{}

// NewPasswordResetCreateValidator creates a PasswordResetCreateValidator.
//
// [Ja] NewPasswordResetCreateValidator は PasswordResetCreateValidator を生成します。
func NewPasswordResetCreateValidator() *PasswordResetCreateValidator {
	return &PasswordResetCreateValidator{}
}

// PasswordResetCreateValidatorInput is the input to
// PasswordResetCreateValidator.Validate.
//
// [Ja] PasswordResetCreateValidatorInput は
// PasswordResetCreateValidator.Validate の入力です。
type PasswordResetCreateValidatorInput struct {
	Email string
}

// Validate checks the submitted email and returns a *model.ValidationError when
// it is missing or malformed. There is no database access, so the only error it
// returns is the validation error; a well-formed but unregistered address passes.
//
// [Ja] Validate は送信された email を検証し、未入力または形式不正のとき
// *model.ValidationError を返します。DB アクセスが無いため返すのはバリデーションエラー
// だけで、形式は正しいが未登録のアドレスは通過します。
func (v *PasswordResetCreateValidator) Validate(ctx context.Context, input PasswordResetCreateValidatorInput) error {
	ve := model.NewValidationError()

	if input.Email == "" {
		ve.AddField("email", i18n.T(ctx, "validation_required"))
		return ve
	}

	if _, err := mail.ParseAddress(input.Email); err != nil {
		ve.AddField("email", i18n.T(ctx, "validation_email_invalid"))
		return ve
	}

	return nil
}
