package validator

import (
	"context"
	"regexp"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// recoveryCodeRegex matches a single recovery code: eight lowercase-alphanumeric
// characters, the shape auth.GenerateRecoveryCodes produces. It gates malformed
// input before the (more expensive) membership lookup and keeps the field-level
// format message aligned with what a real code looks like.
//
// [Ja] recoveryCodeRegex は 1 つのリカバリーコード (auth.GenerateRecoveryCodes が生成する
// 形である 8 文字の小文字英数字) にマッチします。形式不正な入力を (より高コストな) 配列内
// 存在チェックの前で弾き、フィールド別の形式メッセージを実際のコードの見た目に揃えます。
var recoveryCodeRegex = regexp.MustCompile(`^[a-z0-9]{8}$`)

// SignInTwoFactorRecoveryCreateValidator validates the sign-in recovery-code
// challenge: the submitted code's format, and that it is one of the pending user's
// stored, still-unused recovery codes. The pending user is the account whose
// password already passed at the sign-in step (held in the two-factor pending
// cookie), so this validator completes the second factor when the authenticator
// app is unavailable. Membership is a pure read (comparing the code against the
// stored set), so per the validation guideline it belongs in the validator;
// consuming the matched code (a DB write) is the recovery UseCase's transactional
// concern, and this validator returns the resolved setting so the UseCase does not
// re-read it.
//
// [Ja] SignInTwoFactorRecoveryCreateValidator はサインイン時のリカバリーコードチャレンジを
// 検証します。送信されたコードの形式と、それが保留中ユーザーの保存済みでまだ未使用の
// リカバリーコードの 1 つであることです。保留中ユーザーはサインインのステップでパスワードが
// 既に通ったアカウント (2 段階認証の pending Cookie に保持) であり、本 validator は認証アプリを
// 使えないときに第 2 要素を完了させます。配列内存在チェックは純粋な読み取り (コードを保存済みの
// 集合と照合する) のため、バリデーションガイドラインに従い validator に属します。一致した
// コードの消費 (DB 書き込み) はリカバリー UseCase のトランザクションの関心であり、本 validator
// は解決した設定を返して UseCase が再読み込みしなくて済むようにします。
type SignInTwoFactorRecoveryCreateValidator struct {
	userTwoFactorAuthRepo *repository.UserTwoFactorAuthRepository
}

// NewSignInTwoFactorRecoveryCreateValidator creates a
// SignInTwoFactorRecoveryCreateValidator.
//
// [Ja] NewSignInTwoFactorRecoveryCreateValidator は
// SignInTwoFactorRecoveryCreateValidator を生成します。
func NewSignInTwoFactorRecoveryCreateValidator(userTwoFactorAuthRepo *repository.UserTwoFactorAuthRepository) *SignInTwoFactorRecoveryCreateValidator {
	return &SignInTwoFactorRecoveryCreateValidator{userTwoFactorAuthRepo: userTwoFactorAuthRepo}
}

// SignInTwoFactorRecoveryCreateValidatorInput is the input to Validate. UserID is
// the pending user resolved from the two-factor cookie (whose password already
// passed), and Code is the recovery code the user typed from their saved backup
// codes.
//
// [Ja] SignInTwoFactorRecoveryCreateValidatorInput は Validate の入力です。UserID は
// 2 段階認証 Cookie から解決した保留中ユーザー (パスワードが既に通っている)、Code は
// ユーザーが保存済みのバックアップコードから入力したリカバリーコードです。
type SignInTwoFactorRecoveryCreateValidatorInput struct {
	UserID model.UserID
	Code   string
}

// Validate checks the code's format and verifies it is one of the pending user's
// stored recovery codes, returning the resolved 2FA setting on success so the
// UseCase can consume the used code from it. Format problems (missing, or not the
// expected eight lowercase-alphanumeric characters) attach to the code field. When
// the pending user has no enabled 2FA — a stale or forged pending cookie, or 2FA
// disabled between the password step and this challenge — it returns a form-wide
// message so the challenge cannot succeed and the user is asked to sign in again. A
// well-formed code that is not among the stored codes is a code-field error. A
// genuine query failure surfaces as a plain error (handled as 500 upstream).
//
// [Ja] Validate はコードの形式を検証し、それが保留中ユーザーの保存済みリカバリーコードの
// 1 つであることを検証します。成功時は解決した 2FA 設定を返し、UseCase が使用済みコードを
// そこから消費できるようにします。形式の問題 (未入力、または期待する 8 文字の小文字英数字で
// ない) は code フィールドに付けます。保留中ユーザーに有効な 2FA が無いとき (pending Cookie が
// 失効・不正、またはパスワードのステップと本チャレンジの間に 2FA が無効化された) はフォーム
// 全体のメッセージを返し、チャレンジを成功させず、ユーザーに再サインインを促します。形式は
// 整っているが保存済みコードに含まれないコードは code フィールドのエラーです。本物のクエリ
// 失敗は素の error として表れます (上流で 500 として扱う)。
func (v *SignInTwoFactorRecoveryCreateValidator) Validate(ctx context.Context, input SignInTwoFactorRecoveryCreateValidatorInput) (*model.UserTwoFactorAuth, error) {
	ve := model.NewValidationError()

	if input.Code == "" {
		ve.AddField("code", i18n.T(ctx, "validation_required"))
		return nil, ve
	}

	if !recoveryCodeRegex.MatchString(input.Code) {
		ve.AddField("code", i18n.T(ctx, "validation_recovery_code_invalid_format"))
		return nil, ve
	}

	twoFactorAuth, err := v.userTwoFactorAuthRepo.FindEnabledByUserID(ctx, input.UserID)
	if err != nil {
		return nil, err
	}
	if twoFactorAuth == nil {
		ve.AddGlobal(i18n.T(ctx, "validation_two_factor_challenge_invalid"))
		return nil, ve
	}

	if !containsRecoveryCode(twoFactorAuth.RecoveryCodes, input.Code) {
		ve.AddField("code", i18n.T(ctx, "validation_recovery_code_incorrect"))
		return nil, ve
	}

	return twoFactorAuth, nil
}

// containsRecoveryCode reports whether code is present in the stored recovery
// codes. The comparison is exact: recovery codes are shown and stored verbatim, so
// there is no normalization to reconcile.
//
// [Ja] containsRecoveryCode は code が保存済みリカバリーコードに含まれるかを返します。
// 比較は完全一致です。リカバリーコードは表示も保存もそのままの形で行うため、揃えるべき
// 正規化はありません。
func containsRecoveryCode(codes []string, code string) bool {
	for _, c := range codes {
		if c == code {
			return true
		}
	}
	return false
}
