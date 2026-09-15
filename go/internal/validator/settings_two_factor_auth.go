package validator

import (
	"context"
	"regexp"

	"github.com/groobb/groobb/go/internal/auth"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// totpCodeRegexは6桁の数字TOTPコード (認証アプリが生成する形) にマッチします。
// パターンは現状メール確認コードの正規表現と一致しますが、両者は無関係な入力 (TOTPコードと
// メール送信の確認コード) で将来分岐しうるため、共有すると偶然の結合になるので別に定義します。
var totpCodeRegex = regexp.MustCompile(`^\d{6}$`)

// SettingsTwoFactorAuthCreateValidatorは2段階認証の有効化フォームを検証します。
// 送信されたTOTPコードの形式と、そのコードが登録中 (未有効化) の設定のsecretと一致する
// ことです。検証は保存済みsecretを読んでコードを照合するだけのため、バリデーション
// ガイドラインに従いUseCaseではなくvalidatorに属します (失敗時のDB書き込みを伴わない)。
// 登録行自体は先行する設定 (GET) ステップが作成するため、本validatorはクライアント由来の
// secretを信じず、サインイン済みユーザーからそれを解決します。
type SettingsTwoFactorAuthCreateValidator struct {
	userTwoFactorAuthRepo *repository.UserTwoFactorAuthRepository
}

// NewSettingsTwoFactorAuthCreateValidatorは
// SettingsTwoFactorAuthCreateValidatorを生成します。
func NewSettingsTwoFactorAuthCreateValidator(userTwoFactorAuthRepo *repository.UserTwoFactorAuthRepository) *SettingsTwoFactorAuthCreateValidator {
	return &SettingsTwoFactorAuthCreateValidator{userTwoFactorAuthRepo: userTwoFactorAuthRepo}
}

// SettingsTwoFactorAuthCreateValidatorInputはValidateの入力です。UserIDは2FAを
// 有効化するサインイン済みユーザー (セッションで確定する)、Codeはユーザーが認証アプリから
// 入力したTOTPコードです。
type SettingsTwoFactorAuthCreateValidatorInput struct {
	UserID model.UserID
	Code   string
}

// Validateはコードの形式を検証し、登録中の設定のsecretに対して照合します。形式の
// 問題 (未入力、または6桁でない) はcodeフィールドに付けます。登録中の設定が無いとき
// (設定ステップが未実行、または2FAが既に有効) はフォーム全体のメッセージを返し、設定
// ページを新しいsecretから再表示できるようにします。secretと一致しないコードはcode
// フィールドのエラーです。成功時はnilを返し、有効化はユーザーidをキーに行うため登録行は
// 返しません。本物のクエリ失敗は素のerrorとして表れます (上流で500として扱う)。
func (v *SettingsTwoFactorAuthCreateValidator) Validate(ctx context.Context, input SettingsTwoFactorAuthCreateValidatorInput) error {
	ve := model.NewValidationError()

	if input.Code == "" {
		ve.AddField("code", i18n.T(ctx, "validation_required"))
		return ve
	}

	if !totpCodeRegex.MatchString(input.Code) {
		ve.AddField("code", i18n.T(ctx, "validation_totp_code_invalid_format"))
		return ve
	}

	twoFactorAuth, err := v.userTwoFactorAuthRepo.FindByUserID(ctx, input.UserID)
	if err != nil {
		return err
	}
	if twoFactorAuth == nil || twoFactorAuth.Enabled {
		ve.AddGlobal(i18n.T(ctx, "validation_totp_setup_invalid"))
		return ve
	}

	if !auth.ValidateTOTPCode(twoFactorAuth.Secret, input.Code) {
		ve.AddField("code", i18n.T(ctx, "validation_totp_code_incorrect"))
		return ve
	}

	return nil
}

// SettingsTwoFactorAuthDeleteValidatorは2段階認証の無効化フォームを検証します。
// リクエストを、アカウントの現在のパスワードか現在のTOTPコードのどちらか一方で再認証し、
// いずれか1つを要求します。2FAの無効化はセキュリティ要素の除去にあたるため、再認証を
// ゲートにし (退会が現在のパスワードを再確認するのと同様)、放置端末やセッション盗用による
// 無効化を防ぎます。認証アプリでアカウントを守っているユーザーは手元にパスワードが無いことも
// あり、その逆もあるため、どちらの証明でも受け付けます。パスワードリポジトリ (ダイジェストは
// user_passwordsにある) と2FAリポジトリ (TOTPコードを照合するsecret) が必要です。
type SettingsTwoFactorAuthDeleteValidator struct {
	userPasswordRepo      *repository.UserPasswordRepository
	userTwoFactorAuthRepo *repository.UserTwoFactorAuthRepository
}

// NewSettingsTwoFactorAuthDeleteValidatorは
// SettingsTwoFactorAuthDeleteValidatorを生成します。
func NewSettingsTwoFactorAuthDeleteValidator(
	userPasswordRepo *repository.UserPasswordRepository,
	userTwoFactorAuthRepo *repository.UserTwoFactorAuthRepository,
) *SettingsTwoFactorAuthDeleteValidator {
	return &SettingsTwoFactorAuthDeleteValidator{
		userPasswordRepo:      userPasswordRepo,
		userTwoFactorAuthRepo: userTwoFactorAuthRepo,
	}
}

// SettingsTwoFactorAuthDeleteValidatorInputはValidateの入力です。UserIDは2FAを
// 無効化するサインイン済みユーザー (セッションで確定する) です。CurrentPasswordとCodeは
// ユーザーが送信した再認証の値で、どちらか一方を入力します。
type SettingsTwoFactorAuthDeleteValidatorInput struct {
	UserID          model.UserID
	CurrentPassword string
	Code            string
}

// Validateは無効化リクエストを再認証します。現在のパスワードかTOTPコードの
// 少なくとも一方を要求し、どちらか一方が検証を通れば成功とします。どちらも未入力なら
// 「いずれかを入力」のフォーム全体のエラー、片方または両方が入力されたが一つも検証を
// 通らなければ「正しくない」のフォーム全体のエラーです。制約が (どちらか一方で証明という)
// 両フィールドにまたがり単一フィールドではないため、メッセージはフィールド単位ではなく
// フォーム全体とします。本物のクエリ失敗は素のerrorとして表れます (上流で500として扱う)。
func (v *SettingsTwoFactorAuthDeleteValidator) Validate(ctx context.Context, input SettingsTwoFactorAuthDeleteValidatorInput) error {
	ve := model.NewValidationError()

	if input.CurrentPassword == "" && input.Code == "" {
		ve.AddGlobal(i18n.T(ctx, "validation_two_factor_disable_reauth_required"))
		return ve
	}

	verified, err := v.verifyReauth(ctx, input)
	if err != nil {
		return err
	}
	if !verified {
		ve.AddGlobal(i18n.T(ctx, "validation_two_factor_disable_reauth_incorrect"))
		return ve
	}

	return nil
}

// verifyReauthは与えられた資格情報のいずれかがユーザーの身元を証明するかを返します。
// 現在のパスワードを保存済みダイジェストに対して、または整った形式のTOTPコードを有効な
// 設定のsecretに対して照合します。パスワードを先に確認して短絡するため、正しいパスワードなら
// 2FAの参照は不要です。空のフィールドはスキップします (両方が空のケースは呼び出し側が既に
// 弾いています)。本物のリポジトリ障害のときだけ素のerrorを返します。
func (v *SettingsTwoFactorAuthDeleteValidator) verifyReauth(ctx context.Context, input SettingsTwoFactorAuthDeleteValidatorInput) (bool, error) {
	if input.CurrentPassword != "" {
		password, err := v.userPasswordRepo.FindByUserID(ctx, input.UserID)
		if err != nil {
			return false, err
		}
		if password != nil && auth.CheckPassword(password.PasswordDigest, input.CurrentPassword) == nil {
			return true, nil
		}
	}

	if input.Code != "" && totpCodeRegex.MatchString(input.Code) {
		twoFactorAuth, err := v.userTwoFactorAuthRepo.FindEnabledByUserID(ctx, input.UserID)
		if err != nil {
			return false, err
		}
		if twoFactorAuth != nil && auth.ValidateTOTPCode(twoFactorAuth.Secret, input.Code) {
			return true, nil
		}
	}

	return false, nil
}
