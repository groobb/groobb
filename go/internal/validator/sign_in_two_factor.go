package validator

import (
	"context"

	"github.com/groobb/groobb/go/internal/auth"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// SignInTwoFactorCreateValidator validates the sign-in TOTP challenge: the
// submitted code's format, and that it matches the pending user's enabled 2FA
// secret. The pending user is the account whose password already passed at the
// sign-in step (held in the two-factor pending cookie), so this validator completes
// the second factor. Verifying a code only reads the stored secret and compares it,
// so per the validation guideline it belongs in the validator rather than the
// UseCase (no failure-time DB write is involved; consuming a recovery code, which
// does write, is the recovery challenge's concern, not this one).
//
// [Ja] SignInTwoFactorCreateValidator はサインイン時の TOTP チャレンジを検証します。
// 送信された TOTP コードの形式と、そのコードが保留中ユーザーの有効な 2FA 設定の secret と
// 一致することです。保留中ユーザーはサインインのステップでパスワードが既に通ったアカウント
// (2 段階認証の pending Cookie に保持) であり、本 validator は第 2 要素を完了させます。
// コード検証は保存済み secret を読んで照合するだけのため、バリデーションガイドラインに従い
// UseCase ではなく validator に属します (失敗時の DB 書き込みを伴わない。書き込みを伴う
// リカバリーコードの消費はリカバリーチャレンジの関心であり、本チャレンジの関心ではない)。
type SignInTwoFactorCreateValidator struct {
	userTwoFactorAuthRepo *repository.UserTwoFactorAuthRepository
}

// NewSignInTwoFactorCreateValidator creates a SignInTwoFactorCreateValidator.
//
// [Ja] NewSignInTwoFactorCreateValidator は SignInTwoFactorCreateValidator を
// 生成します。
func NewSignInTwoFactorCreateValidator(userTwoFactorAuthRepo *repository.UserTwoFactorAuthRepository) *SignInTwoFactorCreateValidator {
	return &SignInTwoFactorCreateValidator{userTwoFactorAuthRepo: userTwoFactorAuthRepo}
}

// SignInTwoFactorCreateValidatorInput is the input to Validate. UserID is the
// pending user resolved from the two-factor cookie (whose password already
// passed), and Code is the TOTP code the user typed from their authenticator app.
//
// [Ja] SignInTwoFactorCreateValidatorInput は Validate の入力です。UserID は 2 段階認証
// Cookie から解決した保留中ユーザー (パスワードが既に通っている)、Code はユーザーが認証
// アプリから入力した TOTP コードです。
type SignInTwoFactorCreateValidatorInput struct {
	UserID model.UserID
	Code   string
}

// Validate checks the code's format and verifies it against the pending user's
// enabled 2FA secret. Format problems (missing, or not six digits) attach to the
// code field. When the pending user has no enabled 2FA — a stale or forged pending
// cookie, or 2FA disabled between the password step and this challenge — it returns
// a form-wide message so the challenge cannot succeed and the user is asked to sign
// in again. A code that does not match the secret is a code-field error. It returns
// nil on success and does not return anything, since issuing the session is keyed by
// the user id the handler already holds. A genuine query failure surfaces as a plain
// error (handled as 500 upstream).
//
// [Ja] Validate はコードの形式を検証し、保留中ユーザーの有効な 2FA 設定の secret に対して
// 照合します。形式の問題 (未入力、または 6 桁でない) は code フィールドに付けます。保留中
// ユーザーに有効な 2FA が無いとき (pending Cookie が失効・不正、またはパスワードのステップと
// 本チャレンジの間に 2FA が無効化された) はフォーム全体のメッセージを返し、チャレンジを成功
// させず、ユーザーに再サインインを促します。secret と一致しないコードは code フィールドの
// エラーです。成功時は nil を返し何も返しません。セッションの発行はハンドラーが既に持つ
// ユーザー id をキーに行うためです。本物のクエリ失敗は素の error として表れます (上流で
// 500 として扱う)。
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
