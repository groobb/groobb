package validator

import (
	"context"
	"regexp"

	"github.com/groobb/groobb/go/internal/auth"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// totpCodeRegex matches a six-digit numeric TOTP code, the shape an authenticator
// app produces. It is kept separate from the email-confirmation code regex even
// though the pattern currently matches: the two are unrelated inputs (a TOTP code
// versus an emailed confirmation code) that could diverge, so coupling them would
// be incidental.
//
// [Ja] totpCodeRegex は 6 桁の数字 TOTP コード (認証アプリが生成する形) にマッチします。
// パターンは現状メール確認コードの正規表現と一致しますが、両者は無関係な入力 (TOTP コードと
// メール送信の確認コード) で将来分岐しうるため、共有すると偶然の結合になるので別に定義します。
var totpCodeRegex = regexp.MustCompile(`^\d{6}$`)

// SettingsTwoFactorAuthCreateValidator validates the two-factor authentication
// enable form: the submitted TOTP code's format, and that the code matches the
// in-progress (not-yet-enabled) enrollment's secret. Verification only reads the
// stored secret and compares a code, so per the validation guideline it belongs in
// the validator rather than the UseCase (no failure-time DB write is involved).
// The enrollment row itself is created earlier by the setup (GET) step, so this
// validator resolves it by the signed-in user rather than trusting a secret from
// the client.
//
// [Ja] SettingsTwoFactorAuthCreateValidator は 2 段階認証の有効化フォームを検証します。
// 送信された TOTP コードの形式と、そのコードが登録中 (未有効化) の設定の secret と一致する
// ことです。検証は保存済み secret を読んでコードを照合するだけのため、バリデーション
// ガイドラインに従い UseCase ではなく validator に属します (失敗時の DB 書き込みを伴わない)。
// 登録行自体は先行する設定 (GET) ステップが作成するため、本 validator はクライアント由来の
// secret を信じず、サインイン済みユーザーからそれを解決します。
type SettingsTwoFactorAuthCreateValidator struct {
	userTwoFactorAuthRepo *repository.UserTwoFactorAuthRepository
}

// NewSettingsTwoFactorAuthCreateValidator creates a
// SettingsTwoFactorAuthCreateValidator.
//
// [Ja] NewSettingsTwoFactorAuthCreateValidator は
// SettingsTwoFactorAuthCreateValidator を生成します。
func NewSettingsTwoFactorAuthCreateValidator(userTwoFactorAuthRepo *repository.UserTwoFactorAuthRepository) *SettingsTwoFactorAuthCreateValidator {
	return &SettingsTwoFactorAuthCreateValidator{userTwoFactorAuthRepo: userTwoFactorAuthRepo}
}

// SettingsTwoFactorAuthCreateValidatorInput is the input to Validate. UserID is the
// signed-in user enabling 2FA (established by the session), and Code is the TOTP
// code the user typed from their authenticator app.
//
// [Ja] SettingsTwoFactorAuthCreateValidatorInput は Validate の入力です。UserID は 2FA を
// 有効化するサインイン済みユーザー (セッションで確定する)、Code はユーザーが認証アプリから
// 入力した TOTP コードです。
type SettingsTwoFactorAuthCreateValidatorInput struct {
	UserID model.UserID
	Code   string
}

// Validate checks the code's format and verifies it against the in-progress
// enrollment's secret. Format problems (missing, or not six digits) attach to the
// code field. When no in-progress enrollment exists — the setup step was never run,
// or 2FA is already enabled — it returns a form-wide message so the setup page can
// be re-shown from a fresh secret. A code that does not match the secret is a
// code-field error. It returns nil on success and does not return the enrollment,
// since enabling is keyed by the user id, not the row. A genuine query failure
// surfaces as a plain error (handled as 500 upstream).
//
// [Ja] Validate はコードの形式を検証し、登録中の設定の secret に対して照合します。形式の
// 問題 (未入力、または 6 桁でない) は code フィールドに付けます。登録中の設定が無いとき
// (設定ステップが未実行、または 2FA が既に有効) はフォーム全体のメッセージを返し、設定
// ページを新しい secret から再表示できるようにします。secret と一致しないコードは code
// フィールドのエラーです。成功時は nil を返し、有効化はユーザー id をキーに行うため登録行は
// 返しません。本物のクエリ失敗は素の error として表れます (上流で 500 として扱う)。
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

// SettingsTwoFactorAuthDeleteValidator validates the two-factor authentication
// disable form: it re-authenticates the request with either the account's current
// password or a current TOTP code, requiring one of the two. Disabling 2FA removes a
// security factor, so it is gated by re-authentication (mirroring how withdrawal
// re-checks the current password), guarding against a left-open device or a stolen
// session turning it off. Either proof is accepted because a user protecting their
// account with an authenticator may not have their password to hand, and vice versa.
// It needs the password repository (the digest lives in user_passwords) and the 2FA
// repository (the secret a TOTP code is verified against).
//
// [Ja] SettingsTwoFactorAuthDeleteValidator は 2 段階認証の無効化フォームを検証します。
// リクエストを、アカウントの現在のパスワードか現在の TOTP コードのどちらか一方で再認証し、
// いずれか 1 つを要求します。2FA の無効化はセキュリティ要素の除去にあたるため、再認証を
// ゲートにし (退会が現在のパスワードを再確認するのと同様)、放置端末やセッション盗用による
// 無効化を防ぎます。認証アプリでアカウントを守っているユーザーは手元にパスワードが無いことも
// あり、その逆もあるため、どちらの証明でも受け付けます。パスワードリポジトリ (ダイジェストは
// user_passwords にある) と 2FA リポジトリ (TOTP コードを照合する secret) が必要です。
type SettingsTwoFactorAuthDeleteValidator struct {
	userPasswordRepo      *repository.UserPasswordRepository
	userTwoFactorAuthRepo *repository.UserTwoFactorAuthRepository
}

// NewSettingsTwoFactorAuthDeleteValidator creates a
// SettingsTwoFactorAuthDeleteValidator.
//
// [Ja] NewSettingsTwoFactorAuthDeleteValidator は
// SettingsTwoFactorAuthDeleteValidator を生成します。
func NewSettingsTwoFactorAuthDeleteValidator(
	userPasswordRepo *repository.UserPasswordRepository,
	userTwoFactorAuthRepo *repository.UserTwoFactorAuthRepository,
) *SettingsTwoFactorAuthDeleteValidator {
	return &SettingsTwoFactorAuthDeleteValidator{
		userPasswordRepo:      userPasswordRepo,
		userTwoFactorAuthRepo: userTwoFactorAuthRepo,
	}
}

// SettingsTwoFactorAuthDeleteValidatorInput is the input to Validate. UserID is the
// signed-in user disabling 2FA (established by the session). CurrentPassword and
// Code are the re-authentication values the user submitted; the user fills in one of
// the two.
//
// [Ja] SettingsTwoFactorAuthDeleteValidatorInput は Validate の入力です。UserID は 2FA を
// 無効化するサインイン済みユーザー (セッションで確定する) です。CurrentPassword と Code は
// ユーザーが送信した再認証の値で、どちらか一方を入力します。
type SettingsTwoFactorAuthDeleteValidatorInput struct {
	UserID          model.UserID
	CurrentPassword string
	Code            string
}

// Validate re-authenticates the disable request. It requires at least one of the
// current password or a TOTP code, and passes when either one verifies. Neither
// provided is a form-wide "enter one" error; one or both provided but none verifying
// is a form-wide "incorrect" error. The messages are form-wide rather than
// field-level because the constraint spans both fields (prove with either one), not a
// single field. A genuine query failure surfaces as a plain error (handled as 500
// upstream).
//
// [Ja] Validate は無効化リクエストを再認証します。現在のパスワードか TOTP コードの
// 少なくとも一方を要求し、どちらか一方が検証を通れば成功とします。どちらも未入力なら
// 「いずれかを入力」のフォーム全体のエラー、片方または両方が入力されたが一つも検証を
// 通らなければ「正しくない」のフォーム全体のエラーです。制約が (どちらか一方で証明という)
// 両フィールドにまたがり単一フィールドではないため、メッセージはフィールド単位ではなく
// フォーム全体とします。本物のクエリ失敗は素の error として表れます (上流で 500 として扱う)。
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

// verifyReauth reports whether either supplied credential proves the user's
// identity: the current password against the stored digest, or a well-formed TOTP
// code against the enabled setting's secret. It checks the password first and
// short-circuits, so a correct password needs no 2FA lookup. An empty field is
// skipped (the caller has already rejected both being empty). It returns a plain
// error only on a genuine repository failure.
//
// [Ja] verifyReauth は与えられた資格情報のいずれかがユーザーの身元を証明するかを返します。
// 現在のパスワードを保存済みダイジェストに対して、または整った形式の TOTP コードを有効な
// 設定の secret に対して照合します。パスワードを先に確認して短絡するため、正しいパスワードなら
// 2FA の参照は不要です。空のフィールドはスキップします (両方が空のケースは呼び出し側が既に
// 弾いています)。本物のリポジトリ障害のときだけ素の error を返します。
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
