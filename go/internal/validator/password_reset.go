package validator

import (
	"context"
	"net/mail"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
)

// PasswordResetCreateValidatorはパスワードリセット申請フォームを検証します。
// emailが入力され、形式が正しいことを確認します。そのメールのアカウントが存在するかは
// 意図的に検証しません。ここで存在を明かすと攻撃者が登録済みアドレスを列挙できてしまうため、
// 存在チェックは省き、メールが未知のときはUseCaseが静かに何もせず、レスポンスは同じに
// 保ちます。同じ理由でリポジトリも不要です。
type PasswordResetCreateValidator struct{}

// NewPasswordResetCreateValidatorはPasswordResetCreateValidatorを生成します。
func NewPasswordResetCreateValidator() *PasswordResetCreateValidator {
	return &PasswordResetCreateValidator{}
}

// PasswordResetCreateValidatorInputは
// PasswordResetCreateValidator.Validateの入力です。
type PasswordResetCreateValidatorInput struct {
	Email string
}

// Validateは送信されたemailを検証し、未入力または形式不正のとき
// *model.ValidationErrorを返します。DBアクセスが無いため返すのはバリデーションエラー
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
