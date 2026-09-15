// validatorパッケージはGroobbの入力バリデーターを保持します。バリデーターは
// フォームの送信値 (形式チェックとDBに対する状態チェック) を検証し、失敗を
// *model.ValidationErrorとして報告します。バリデーターはHandlerから直接ではなく
// UseCaseから呼び出され、どの入口でも同じバリデーションが走るようにします。
package validator

import (
	"context"
	"net/mail"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// SignUpCreateValidatorはサインアップ申請フォームを検証します。emailが入力され、
// 形式が正しく、既存アカウントに使われていないことを確認します。
type SignUpCreateValidator struct {
	userRepo *repository.UserRepository
}

// NewSignUpCreateValidatorはSignUpCreateValidatorを生成します。
func NewSignUpCreateValidator(userRepo *repository.UserRepository) *SignUpCreateValidator {
	return &SignUpCreateValidator{userRepo: userRepo}
}

// SignUpCreateValidatorInputはSignUpCreateValidator.Validateの入力です。
type SignUpCreateValidatorInput struct {
	Email string
}

// Validateは送信されたemailを検証し、入力に問題があれば
// *model.ValidationErrorを、本物のシステム障害 (例: データベースに到達できない) では
// 素のerrorを返します。重複メールのチェックは列挙攻撃対策で成功扱いにするのではなく、
// 意図的に明示的なエラーにします。既存アカウントを伝えるのは姉妹プロジェクトで確立した
// サインアップの挙動であり、利用者が期待する形に合致します。照合はusers.emailが
// NOCASE照合のため大文字小文字を区別しません。
func (v *SignUpCreateValidator) Validate(ctx context.Context, input SignUpCreateValidatorInput) error {
	ve := model.NewValidationError()

	if input.Email == "" {
		ve.AddField("email", i18n.T(ctx, "validation_required"))
		return ve
	}

	if _, err := mail.ParseAddress(input.Email); err != nil {
		ve.AddField("email", i18n.T(ctx, "validation_email_invalid"))
		return ve
	}

	user, err := v.userRepo.FindByEmail(ctx, input.Email)
	if err != nil {
		return err
	}
	if user != nil {
		ve.AddField("email", i18n.T(ctx, "validation_email_already_taken"))
		return ve
	}

	return nil
}
