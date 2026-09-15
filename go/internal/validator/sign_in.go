package validator

import (
	"context"
	"net/mail"

	"github.com/groobb/groobb/go/internal/auth"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// SignInCreateValidatorはサインインフォームを検証します。emailとパスワードが
// 入力され形式が正しく、それらがパスワードの一致するアカウントを指すことです。emailで
// アカウントを引くためのユーザーリポジトリと、そのアカウントの資格情報を取るための
// パスワードリポジトリを必要とします。パスワードダイジェストはusersではなく
// user_passwordsにあるためです。
type SignInCreateValidator struct {
	userRepo              *repository.UserRepository
	userPasswordRepo      *repository.UserPasswordRepository
	userTwoFactorAuthRepo *repository.UserTwoFactorAuthRepository
}

// NewSignInCreateValidatorはSignInCreateValidatorを生成します。2段階認証
// リポジトリにより、認証されたユーザーと併せてそのアカウントで2FAが有効かを報告でき、
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

// SignInCreateValidateOutputはサインイン検証の成功結果です。認証されたユーザーと、
// 2段階認証が有効な場合はその設定 (無ければnil) を持ちます。サインインフローは
// UserTwoFactorAuthを見て、そのままサインインさせるかTOTP / リカバリーコードの
// チャレンジへ迂回させるかを決めます。
type SignInCreateValidateOutput struct {
	User              *model.User
	UserTwoFactorAuth *model.UserTwoFactorAuth
}

// SignInCreateValidatorInputはSignInCreateValidator.Validateの入力です。
type SignInCreateValidatorInput struct {
	Email    string
	Password string
}

// Validateは送信された資格情報を検証し、成功時は認証されたユーザーと、有効な
// 2段階認証設定 (アカウントに無ければnil) を併せて返します。入力に問題があれば
// *model.ValidationErrorを、本物のシステム障害 (例: データベースに到達できない) では素の
// errorを返します。形式の問題 (emailの未入力・不正、パスワードの未入力) はフィールド別に
// 報告します。資格情報チェックの失敗 (未知のemail・パスワードの無いアカウント・誤った
// パスワード) は、どちらが誤りかを明かさない単一のグローバルメッセージで意図的に報告し、
// フォームがアカウント列挙のオラクルにならないようにします。emailの照合はusers.emailが
// NOCASE照合のため大文字小文字を区別しません。
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
	// 未知のemailは誤ったパスワードと同じ汎用メッセージを報告し、サインインの失敗が
	// そのemailが登録済みかどうかを開示しないようにする。
	if user == nil {
		ve.AddGlobal(i18n.T(ctx, "validation_credentials_invalid"))
		return nil, ve
	}

	password, err := v.userPasswordRepo.FindByUserID(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	// パスワード資格情報が無い (例: SSOのみのアカウント) 場合はパスワードでは
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

	// パスワードが一致した後にのみ、このアカウントで2FAが有効かを引く (登録中で
	// 未有効化の設定は無しと数える)。非nilの設定は、サインインフローに対し、そのまま
	// サインインさせる代わりにセッションを保留してTOTP / リカバリーコードのチャレンジを
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

// validateEmailはemailが未入力または不正な形式のときにフィールドエラーを記録
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

// validatePasswordはパスワードが未入力のときにフィールドエラーを記録します。
// 長さポリシーはサインインでは強制しません。資格情報は保存済みダイジェストとの一致だけで
// 受理・拒否されるため、ここでポリシーを再チェックすると、ポリシー変更前のパスワードを
// 持つアカウントを締め出すリスクがあるだけです。
func (v *SignInCreateValidator) validatePassword(ctx context.Context, ve *model.ValidationError, password string) {
	if password == "" {
		ve.AddField("password", i18n.T(ctx, "validation_required"))
	}
}
