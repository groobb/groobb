// Package validator holds Groobb's input validators. A validator checks a
// form's submitted values (format checks, and state checks against the database)
// and reports failures as a *model.ValidationError. Validators are called from
// UseCases, never directly from handlers, so every entry point runs the same
// validation.
//
// [Ja] validator パッケージは Groobb の入力バリデーターを保持します。バリデーターは
// フォームの送信値 (形式チェックと DB に対する状態チェック) を検証し、失敗を
// *model.ValidationError として報告します。バリデーターは Handler から直接ではなく
// UseCase から呼び出され、どの入口でも同じバリデーションが走るようにします。
package validator

import (
	"context"
	"net/mail"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// SignUpCreateValidator validates the sign-up request form: that the email is
// present, well-formed, and not already taken by an existing account.
//
// [Ja] SignUpCreateValidator はサインアップ申請フォームを検証します。email が入力され、
// 形式が正しく、既存アカウントに使われていないことを確認します。
type SignUpCreateValidator struct {
	userRepo *repository.UserRepository
}

// NewSignUpCreateValidator creates a SignUpCreateValidator.
//
// [Ja] NewSignUpCreateValidator は SignUpCreateValidator を生成します。
func NewSignUpCreateValidator(userRepo *repository.UserRepository) *SignUpCreateValidator {
	return &SignUpCreateValidator{userRepo: userRepo}
}

// SignUpCreateValidatorInput is the input to SignUpCreateValidator.Validate.
//
// [Ja] SignUpCreateValidatorInput は SignUpCreateValidator.Validate の入力です。
type SignUpCreateValidatorInput struct {
	Email string
}

// Validate checks the submitted email and returns a *model.ValidationError on
// any input problem, or a plain error on a genuine system failure (e.g. the
// database is unreachable). The duplicate-email check is intentionally explicit
// rather than enumeration-safe: an existing-account message is the established
// sign-up behavior in the sister projects and matches the form the user expects.
// The match is case-insensitive because users.email collates NOCASE.
//
// [Ja] Validate は送信された email を検証し、入力に問題があれば
// *model.ValidationError を、本物のシステム障害 (例: データベースに到達できない) では
// 素の error を返します。重複メールのチェックは列挙攻撃対策で成功扱いにするのではなく、
// 意図的に明示的なエラーにします。既存アカウントを伝えるのは姉妹プロジェクトで確立した
// サインアップの挙動であり、利用者が期待する形に合致します。照合は users.email が
// NOCASE 照合のため大文字小文字を区別しません。
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
