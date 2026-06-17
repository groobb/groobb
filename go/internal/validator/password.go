package validator

import (
	"context"
	"errors"

	"github.com/groobb/groobb/go/internal/auth"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// PasswordUpdateValidator validates the password-reset update form: the reset
// token names a usable token and the chosen password meets the strength policy
// with a matching confirmation. It holds the token repository because verifying
// the token is a state check (a database lookup). The check is read-only — it
// never updates the token on failure — so per the validation guide it belongs in
// the validator rather than the UseCase; the UseCase only stamps the token used
// after a successful update.
//
// [Ja] PasswordUpdateValidator はパスワードリセットの更新フォームを検証します。リセット
// トークンが使えるトークンを指し、選んだパスワードが強度ポリシーを満たし確認が一致する
// ことです。トークンの検証は状態チェック (DB ルックアップ) のためトークンリポジトリを
// 持ちます。このチェックは読み取り専用で失敗時にトークンを更新しないため、バリデーション
// ガイドに従い UseCase ではなく validator に置きます。UseCase は更新成功後にトークンを
// 使用済みとして打刻するだけです。
type PasswordUpdateValidator struct {
	passwordResetTokenRepo *repository.PasswordResetTokenRepository
}

// NewPasswordUpdateValidator creates a PasswordUpdateValidator backed by the
// password reset token repository.
//
// [Ja] NewPasswordUpdateValidator はパスワードリセットトークンリポジトリを使う
// PasswordUpdateValidator を生成します。
func NewPasswordUpdateValidator(passwordResetTokenRepo *repository.PasswordResetTokenRepository) *PasswordUpdateValidator {
	return &PasswordUpdateValidator{passwordResetTokenRepo: passwordResetTokenRepo}
}

// PasswordUpdateValidatorInput is the input to PasswordUpdateValidator.Validate.
// Token is the plaintext reset token from the emailed link; Password and
// PasswordConfirmation are the new credential.
//
// [Ja] PasswordUpdateValidatorInput は PasswordUpdateValidator.Validate の入力です。
// Token はメールのリンクから来る平文のリセットトークン、Password /
// PasswordConfirmation は新しい資格情報です。
type PasswordUpdateValidatorInput struct {
	Token                string
	Password             string
	PasswordConfirmation string
}

// PasswordUpdateValidateOutput carries the resolved token's id and the user it
// resets, so the UseCase can update that user's password and mark that token used
// without looking either up again.
//
// [Ja] PasswordUpdateValidateOutput は解決したトークンの id とリセット対象のユーザーを
// 運び、UseCase が再ルックアップせずにそのユーザーのパスワードを更新しそのトークンを
// 使用済みにできるようにします。
type PasswordUpdateValidateOutput struct {
	TokenID model.PasswordResetTokenID
	UserID  model.UserID
}

// Validate checks the form and the token, returning a *model.ValidationError when
// anything is invalid. Format checks run first (an empty token is a form-wide
// error; the password must be present, meet the strength policy, and match its
// confirmation), since a malformed submission should not trigger a token lookup.
// Only when the form is well-formed is the token resolved by its digest and
// checked for being unknown, already used, or expired — each reported as a
// form-wide error. The token states are distinguished (rather than collapsed into
// one message) because the token is a high-entropy random value that cannot be
// enumerated, so telling the user the link expired or was already used is helpful
// without leaking anything.
//
// [Ja] Validate はフォームとトークンを検証し、不正があれば *model.ValidationError を
// 返します。形式チェックを先に行います (空トークンはフォーム全体のエラー。パスワードは
// 入力必須で強度ポリシーを満たし確認と一致する必要がある)。不正な送信でトークンルックアップを
// 走らせないためです。フォームが整っているときに限りトークンをダイジェストで解決し、未知・
// 使用済み・期限切れを判定します (それぞれフォーム全体のエラー)。トークンの状態は (1 つの
// メッセージに集約せず) 区別します。トークンは列挙できない高エントロピーのランダム値のため、
// リンクが期限切れ・使用済みだとユーザーに伝えても何も漏らさず親切だからです。
func (v *PasswordUpdateValidator) Validate(ctx context.Context, input PasswordUpdateValidatorInput) (*PasswordUpdateValidateOutput, error) {
	ve := model.NewValidationError()

	// A missing token means the link itself is broken; report it form-wide and
	// stop, since there is nothing to look up.
	//
	// [Ja] トークンが無いのはリンク自体が壊れていることを意味する。ルックアップ対象が
	// 無いため、フォーム全体に報告して中断する。
	if input.Token == "" {
		ve.AddGlobal(i18n.T(ctx, "validation_token_invalid"))
		return nil, ve
	}

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
		return nil, ve
	}

	return v.validateToken(ctx, input.Token)
}

// validateToken resolves the token by its digest and confirms it is usable,
// returning the resolved ids on success or a form-wide *model.ValidationError
// describing why it is not. A lookup failure (a genuine database error) is
// returned as a bare error so the handler surfaces it as a 500.
//
// [Ja] validateToken はトークンをダイジェストで解決して使えることを確認し、成功時は
// 解決した id を、使えないときはその理由を表すフォーム全体の *model.ValidationError を
// 返します。ルックアップの失敗 (本物の DB エラー) は素の error として返し、ハンドラーが
// 500 として表面化させます。
func (v *PasswordUpdateValidator) validateToken(ctx context.Context, token string) (*PasswordUpdateValidateOutput, error) {
	resetToken, err := v.passwordResetTokenRepo.FindByTokenDigest(ctx, auth.HashToken(token))
	if err != nil {
		return nil, err
	}

	ve := model.NewValidationError()
	switch {
	case resetToken == nil:
		ve.AddGlobal(i18n.T(ctx, "validation_token_invalid"))
		return nil, ve
	case resetToken.IsUsed():
		ve.AddGlobal(i18n.T(ctx, "validation_token_used"))
		return nil, ve
	case resetToken.IsExpired():
		ve.AddGlobal(i18n.T(ctx, "validation_token_expired"))
		return nil, ve
	}

	return &PasswordUpdateValidateOutput{
		TokenID: resetToken.ID,
		UserID:  resetToken.UserID,
	}, nil
}
