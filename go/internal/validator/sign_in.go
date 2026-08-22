package validator

import (
	"context"
	"net/mail"

	"github.com/groobb/groobb/go/internal/auth"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// SignInCreateValidator validates the sign-in form: that the email and password
// are present and well-formed, and that they identify an account whose password
// matches. It needs the user repository to look up the account by email and the
// password repository to fetch that account's credential, since the password
// digest lives in user_passwords (not on users).
//
// [Ja] SignInCreateValidator はサインインフォームを検証します。email とパスワードが
// 入力され形式が正しく、それらがパスワードの一致するアカウントを指すことです。email で
// アカウントを引くためのユーザーリポジトリと、そのアカウントの資格情報を取るための
// パスワードリポジトリを必要とします。パスワードダイジェストは users ではなく
// user_passwords にあるためです。
type SignInCreateValidator struct {
	userRepo              *repository.UserRepository
	userPasswordRepo      *repository.UserPasswordRepository
	userTwoFactorAuthRepo *repository.UserTwoFactorAuthRepository
}

// NewSignInCreateValidator creates a SignInCreateValidator. The two-factor repo
// lets it report, alongside the authenticated user, whether that account has 2FA
// enabled, so the sign-in flow can require a challenge before issuing a session.
//
// [Ja] NewSignInCreateValidator は SignInCreateValidator を生成します。2 段階認証
// リポジトリにより、認証されたユーザーと併せてそのアカウントで 2FA が有効かを報告でき、
// サインインフローがセッション発行の前にチャレンジを要求できます。
func NewSignInCreateValidator(
	userRepo *repository.UserRepository,
	userPasswordRepo *repository.UserPasswordRepository,
	userTwoFactorAuthRepo *repository.UserTwoFactorAuthRepository,
) *SignInCreateValidator {
	return &SignInCreateValidator{
		userRepo:              userRepo,
		userPasswordRepo:      userPasswordRepo,
		userTwoFactorAuthRepo: userTwoFactorAuthRepo,
	}
}

// SignInCreateValidateOutput is the result of a successful sign-in validation:
// the authenticated user, and their enabled two-factor setting when they have one
// (nil otherwise). The sign-in flow uses UserTwoFactorAuth to decide whether to
// sign in outright or divert to a TOTP / recovery-code challenge.
//
// [Ja] SignInCreateValidateOutput はサインイン検証の成功結果です。認証されたユーザーと、
// 2 段階認証が有効な場合はその設定 (無ければ nil) を持ちます。サインインフローは
// UserTwoFactorAuth を見て、そのままサインインさせるか TOTP / リカバリーコードの
// チャレンジへ迂回させるかを決めます。
type SignInCreateValidateOutput struct {
	User              *model.User
	UserTwoFactorAuth *model.UserTwoFactorAuth
}

// SignInCreateValidatorInput is the input to SignInCreateValidator.Validate.
//
// [Ja] SignInCreateValidatorInput は SignInCreateValidator.Validate の入力です。
type SignInCreateValidatorInput struct {
	Email    string
	Password string
}

// Validate checks the submitted credentials and, on success, returns the
// authenticated user together with their enabled two-factor setting (nil when the
// account has none). It returns a *model.ValidationError on any input problem, or
// a plain error on a genuine system failure (e.g. the database is unreachable).
// Format problems (missing or malformed email, missing password) are reported per
// field. A failed credential check — unknown email, an account without a
// password, or a wrong password — is deliberately reported with a single global
// message that does not reveal which of the two was wrong, so the form does not
// become an account-enumeration oracle. The email match is case-insensitive
// because users.email collates NOCASE.
//
// [Ja] Validate は送信された資格情報を検証し、成功時は認証されたユーザーと、有効な
// 2 段階認証設定 (アカウントに無ければ nil) を併せて返します。入力に問題があれば
// *model.ValidationError を、本物のシステム障害 (例: データベースに到達できない) では素の
// error を返します。形式の問題 (email の未入力・不正、パスワードの未入力) はフィールド別に
// 報告します。資格情報チェックの失敗 (未知の email・パスワードの無いアカウント・誤った
// パスワード) は、どちらが誤りかを明かさない単一のグローバルメッセージで意図的に報告し、
// フォームがアカウント列挙のオラクルにならないようにします。email の照合は users.email が
// NOCASE 照合のため大文字小文字を区別しません。
func (v *SignInCreateValidator) Validate(ctx context.Context, input SignInCreateValidatorInput) (*SignInCreateValidateOutput, error) {
	ve := model.NewValidationError()

	v.validateEmail(ctx, ve, input.Email)
	v.validatePassword(ctx, ve, input.Password)

	if ve.HasErrors() {
		return nil, ve
	}

	user, err := v.userRepo.FindByEmail(ctx, input.Email)
	if err != nil {
		return nil, err
	}
	// An unknown email reports the same generic message as a wrong password, so a
	// failed sign-in does not disclose whether the email is registered.
	//
	// [Ja] 未知の email は誤ったパスワードと同じ汎用メッセージを報告し、サインインの失敗が
	// その email が登録済みかどうかを開示しないようにする。
	if user == nil {
		ve.AddGlobal(i18n.T(ctx, "validation_credentials_invalid"))
		return nil, ve
	}

	password, err := v.userPasswordRepo.FindByUserID(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	// No password credential (e.g. an SSO-only account) cannot sign in with a
	// password; report the same generic message rather than revealing the account
	// exists but lacks a password.
	//
	// [Ja] パスワード資格情報が無い (例: SSO のみのアカウント) 場合はパスワードでは
	// サインインできない。アカウントは在るがパスワードが無いと明かす代わりに、同じ汎用
	// メッセージを報告する。
	if password == nil {
		ve.AddGlobal(i18n.T(ctx, "validation_credentials_invalid"))
		return nil, ve
	}

	if err := auth.CheckPassword(password.PasswordDigest, input.Password); err != nil {
		ve.AddGlobal(i18n.T(ctx, "validation_credentials_invalid"))
		return nil, ve
	}

	// Only once the password matches, look up whether this account has 2FA enabled
	// (an enrolling-but-not-enabled setting counts as none). A non-nil setting
	// tells the sign-in flow to hold the session and require a TOTP / recovery-code
	// challenge rather than signing in outright. Looking it up here, past the
	// credential check, keeps the query off the path of a failed sign-in.
	//
	// [Ja] パスワードが一致した後にのみ、このアカウントで 2FA が有効かを引く (登録中で
	// 未有効化の設定は無しと数える)。非 nil の設定は、サインインフローに対し、そのまま
	// サインインさせる代わりにセッションを保留して TOTP / リカバリーコードのチャレンジを
	// 要求すべきことを伝える。資格情報チェックを通った後にここで引くことで、失敗した
	// サインインの経路ではこのクエリを走らせない。
	twoFactorAuth, err := v.userTwoFactorAuthRepo.FindEnabledByUserID(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	return &SignInCreateValidateOutput{
		User:              user,
		UserTwoFactorAuth: twoFactorAuth,
	}, nil
}

// validateEmail records a field error when the email is missing or malformed.
//
// [Ja] validateEmail は email が未入力または不正な形式のときにフィールドエラーを記録
// します。
func (v *SignInCreateValidator) validateEmail(ctx context.Context, ve *model.ValidationError, email string) {
	if email == "" {
		ve.AddField("email", i18n.T(ctx, "validation_required"))
		return
	}

	if _, err := mail.ParseAddress(email); err != nil {
		ve.AddField("email", i18n.T(ctx, "validation_email_invalid"))
	}
}

// validatePassword records a field error when the password is missing. The
// length policy is not enforced on sign-in: a credential is accepted or rejected
// solely by matching the stored digest, so re-checking the policy here would only
// risk locking out an account whose password predates a policy change.
//
// [Ja] validatePassword はパスワードが未入力のときにフィールドエラーを記録します。
// 長さポリシーはサインインでは強制しません。資格情報は保存済みダイジェストとの一致だけで
// 受理・拒否されるため、ここでポリシーを再チェックすると、ポリシー変更前のパスワードを
// 持つアカウントを締め出すリスクがあるだけです。
func (v *SignInCreateValidator) validatePassword(ctx context.Context, ve *model.ValidationError, password string) {
	if password == "" {
		ve.AddField("password", i18n.T(ctx, "validation_required"))
	}
}
