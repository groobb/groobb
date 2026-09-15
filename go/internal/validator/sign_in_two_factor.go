package validator

import (
	"context"

	"github.com/groobb/groobb/go/internal/auth"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// SignInTwoFactorCreateValidatorはサインイン時のTOTPチャレンジを検証します。
// 送信されたTOTPコードの形式と、そのコードが保留中ユーザーの有効な2FA設定のsecretと
// 一致することです。保留中ユーザーはサインインのステップでパスワードが既に通ったアカウント
// (2段階認証のpending Cookieに保持) であり、本validatorは第2要素を完了させます。
// コード検証は保存済みsecretを読んで照合するだけのため、バリデーションガイドラインに従い
// UseCaseではなくvalidatorに属します (失敗時のDB書き込みを伴わない。書き込みを伴う
// リカバリーコードの消費はリカバリーチャレンジの関心であり、本チャレンジの関心ではない)。
type SignInTwoFactorCreateValidator struct {
	userTwoFactorAuthRepo *repository.UserTwoFactorAuthRepository
}

// NewSignInTwoFactorCreateValidatorはSignInTwoFactorCreateValidatorを
// 生成します。
func NewSignInTwoFactorCreateValidator(userTwoFactorAuthRepo *repository.UserTwoFactorAuthRepository) *SignInTwoFactorCreateValidator {
	return &SignInTwoFactorCreateValidator{userTwoFactorAuthRepo: userTwoFactorAuthRepo}
}

// SignInTwoFactorCreateValidatorInputはValidateの入力です。UserIDは2段階認証
// Cookieから解決した保留中ユーザー (パスワードが既に通っている)、Codeはユーザーが認証
// アプリから入力したTOTPコードです。
type SignInTwoFactorCreateValidatorInput struct {
	UserID model.UserID
	Code   string
}

// Validateはコードの形式を検証し、保留中ユーザーの有効な2FA設定のsecretに対して
// 照合します。形式の問題 (未入力、または6桁でない) はcodeフィールドに付けます。保留中
// ユーザーに有効な2FAが無いとき (pending Cookieが失効・不正、またはパスワードのステップと
// 本チャレンジの間に2FAが無効化された) はフォーム全体のメッセージを返し、チャレンジを成功
// させず、ユーザーに再サインインを促します。secretと一致しないコードはcodeフィールドの
// エラーです。成功時はnilを返し何も返しません。セッションの発行はハンドラーが既に持つ
// ユーザーidをキーに行うためです。本物のクエリ失敗は素のerrorとして表れます (上流で
// 500として扱う)。
func (v *SignInTwoFactorCreateValidator) Validate(ctx context.Context, input SignInTwoFactorCreateValidatorInput) error {
	ve := model.NewValidationError()

	if input.Code == "" {
		ve.AddField("code", i18n.T(ctx, "validation_required"))
		return ve
	}

	if !totpCodeRegex.MatchString(input.Code) {
		ve.AddField("code", i18n.T(ctx, "validation_totp_code_invalid_format"))
		return ve
	}

	twoFactorAuth, err := v.userTwoFactorAuthRepo.FindEnabledByUserID(ctx, input.UserID)
	if err != nil {
		return err
	}
	if twoFactorAuth == nil {
		ve.AddGlobal(i18n.T(ctx, "validation_two_factor_challenge_invalid"))
		return ve
	}

	if !auth.ValidateTOTPCode(twoFactorAuth.Secret, input.Code) {
		ve.AddField("code", i18n.T(ctx, "validation_totp_code_incorrect"))
		return ve
	}

	return nil
}
