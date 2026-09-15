package validator

import (
	"context"
	"regexp"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// recoveryCodeRegexは1つのリカバリーコード (auth.GenerateRecoveryCodesが生成する
// 形である8文字の小文字英数字) にマッチします。形式不正な入力を (より高コストな) 配列内
// 存在チェックの前で弾き、フィールド別の形式メッセージを実際のコードの見た目に揃えます。
var recoveryCodeRegex = regexp.MustCompile(`^[a-z0-9]{8}$`)

// SignInTwoFactorRecoveryCreateValidatorはサインイン時のリカバリーコードチャレンジを
// 検証します。送信されたコードの形式と、それが保留中ユーザーの保存済みでまだ未使用の
// リカバリーコードの1つであることです。保留中ユーザーはサインインのステップでパスワードが
// 既に通ったアカウント (2段階認証のpending Cookieに保持) であり、本validatorは認証アプリを
// 使えないときに第2要素を完了させます。配列内存在チェックは純粋な読み取り (コードを保存済みの
// 集合と照合する) のため、バリデーションガイドラインに従いvalidatorに属します。一致した
// コードの消費 (DB書き込み) はリカバリーUseCaseのトランザクションの関心であり、本validator
// は解決した設定を返してUseCaseが再読み込みしなくて済むようにします。
type SignInTwoFactorRecoveryCreateValidator struct {
	userTwoFactorAuthRepo *repository.UserTwoFactorAuthRepository
}

// NewSignInTwoFactorRecoveryCreateValidatorは
// SignInTwoFactorRecoveryCreateValidatorを生成します。
func NewSignInTwoFactorRecoveryCreateValidator(userTwoFactorAuthRepo *repository.UserTwoFactorAuthRepository) *SignInTwoFactorRecoveryCreateValidator {
	return &SignInTwoFactorRecoveryCreateValidator{userTwoFactorAuthRepo: userTwoFactorAuthRepo}
}

// SignInTwoFactorRecoveryCreateValidatorInputはValidateの入力です。UserIDは
// 2段階認証Cookieから解決した保留中ユーザー (パスワードが既に通っている)、Codeは
// ユーザーが保存済みのバックアップコードから入力したリカバリーコードです。
type SignInTwoFactorRecoveryCreateValidatorInput struct {
	UserID model.UserID
	Code   string
}

// Validateはコードの形式を検証し、それが保留中ユーザーの保存済みリカバリーコードの
// 1つであることを検証します。成功時は解決した2FA設定を返し、UseCaseが使用済みコードを
// そこから消費できるようにします。形式の問題 (未入力、または期待する8文字の小文字英数字で
// ない) はcodeフィールドに付けます。保留中ユーザーに有効な2FAが無いとき (pending Cookieが
// 失効・不正、またはパスワードのステップと本チャレンジの間に2FAが無効化された) はフォーム
// 全体のメッセージを返し、チャレンジを成功させず、ユーザーに再サインインを促します。形式は
// 整っているが保存済みコードに含まれないコードはcodeフィールドのエラーです。本物のクエリ
// 失敗は素のerrorとして表れます (上流で500として扱う)。
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

// containsRecoveryCodeはcodeが保存済みリカバリーコードに含まれるかを返します。
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
