package validator

import (
	"context"
	"errors"

	"github.com/groobb/groobb/go/internal/auth"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// PasswordUpdateValidatorはパスワードリセットの更新フォームを検証します。リセット
// トークンが使えるトークンを指し、選んだパスワードが強度ポリシーを満たし確認が一致する
// ことです。トークンの検証は状態チェック (DBルックアップ) のためトークンリポジトリを
// 持ちます。このチェックは読み取り専用で失敗時にトークンを更新しないため、バリデーション
// ガイドに従いUseCaseではなくvalidatorに置きます。UseCaseは更新成功後にトークンを
// 使用済みとして打刻するだけです。
type PasswordUpdateValidator struct {
	passwordResetTokenRepo *repository.PasswordResetTokenRepository
}

// NewPasswordUpdateValidatorはパスワードリセットトークンリポジトリを使う
// PasswordUpdateValidatorを生成します。
func NewPasswordUpdateValidator(passwordResetTokenRepo *repository.PasswordResetTokenRepository) *PasswordUpdateValidator {
	return &PasswordUpdateValidator{passwordResetTokenRepo: passwordResetTokenRepo}
}

// PasswordUpdateValidatorInputはPasswordUpdateValidator.Validateの入力です。
// Tokenはメールのリンクから来る平文のリセットトークン、Password /
// PasswordConfirmationは新しい資格情報です。
type PasswordUpdateValidatorInput struct {
	Token                string
	Password             string
	PasswordConfirmation string
}

// PasswordUpdateValidateOutputは解決したトークンのidとリセット対象のユーザーを
// 運び、UseCaseが再ルックアップせずにそのユーザーのパスワードを更新しそのトークンを
// 使用済みにできるようにします。
type PasswordUpdateValidateOutput struct {
	TokenID model.PasswordResetTokenID
	UserID  model.UserID
}

// Validateはフォームとトークンを検証し、不正があれば *model.ValidationErrorを
// 返します。形式チェックを先に行います (空トークンはフォーム全体のエラー。パスワードは
// 入力必須で強度ポリシーを満たし確認と一致する必要がある)。不正な送信でトークンルックアップを
// 走らせないためです。フォームが整っているときに限りトークンをダイジェストで解決し、未知・
// 使用済み・期限切れを判定します (それぞれフォーム全体のエラー)。トークンの状態は (1つの
// メッセージに集約せず) 区別します。トークンは列挙できない高エントロピーのランダム値のため、
// リンクが期限切れ・使用済みだとユーザーに伝えても何も漏らさず親切だからです。
func (v *PasswordUpdateValidator) Validate(ctx context.Context, input PasswordUpdateValidatorInput) (*PasswordUpdateValidateOutput, error) {
	ve := model.NewValidationError()

	// トークンが無いのはリンク自体が壊れていることを意味する。ルックアップ対象が
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

	// 確認は空でないパスワードに対してのみ照合する。空パスワードは既に「必須」を
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

// validateTokenはトークンをダイジェストで解決して使えることを確認し、成功時は
// 解決したidを、使えないときはその理由を表すフォーム全体の *model.ValidationErrorを
// 返します。ルックアップの失敗 (本物のDBエラー) は素のerrorとして返し、ハンドラーが
// 500として表面化させます。
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
