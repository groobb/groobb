package usecase

import (
	"context"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/validator"
)

// CreateSignInTwoFactorUsecase orchestrates the sign-in TOTP challenge: it runs the
// two-factor validator against the pending user and the submitted code. Issuing the
// session (completing the sign-in) is a separate step the handler runs with
// CreateSessionUsecase — mirroring the password step, where CreateSignInUsecase
// validates and the handler then creates the session — because session creation
// needs request data (the IP and user agent). It takes no transaction because
// verifying the code is a pure read; nothing is persisted here.
//
// [Ja] CreateSignInTwoFactorUsecase はサインイン時の TOTP チャレンジを統括します。2 段階認証
// バリデーターを保留中ユーザーと送信されたコードに対して実行します。セッションの発行
// (サインインの完了) は、パスワードのステップと同様にハンドラーが CreateSessionUsecase で
// 行う別ステップです (CreateSignInUsecase が検証し、その後ハンドラーがセッションを作るのと
// 同じ)。セッション発行はリクエストのデータ (IP と User-Agent) を必要とするためです。
// コード検証は純粋な読み取りで、ここでは何も永続化しないため、トランザクションは取りません。
type CreateSignInTwoFactorUsecase struct {
	signInTwoFactorValidator *validator.SignInTwoFactorCreateValidator
}

// NewCreateSignInTwoFactorUsecase builds a CreateSignInTwoFactorUsecase from the
// sign-in two-factor validator.
//
// [Ja] NewCreateSignInTwoFactorUsecase はサインイン 2 段階認証バリデーターから
// CreateSignInTwoFactorUsecase を構築します。
func NewCreateSignInTwoFactorUsecase(signInTwoFactorValidator *validator.SignInTwoFactorCreateValidator) *CreateSignInTwoFactorUsecase {
	return &CreateSignInTwoFactorUsecase{signInTwoFactorValidator: signInTwoFactorValidator}
}

// CreateSignInTwoFactorInput is the input to Execute. UserID is the pending user
// resolved from the two-factor cookie, and Code is the submitted TOTP code.
//
// [Ja] CreateSignInTwoFactorInput は Execute の入力です。UserID は 2 段階認証 Cookie から
// 解決した保留中ユーザー、Code は送信された TOTP コードです。
type CreateSignInTwoFactorInput struct {
	UserID model.UserID
	Code   string
}

// Execute validates the submitted TOTP code against the pending user's enabled 2FA
// setting. The validator's error (a *model.ValidationError for a bad or non-matching
// code, or a plain error for a system failure) is returned unchanged for the handler
// to classify; on success it returns nil and the handler issues the session.
//
// [Ja] Execute は送信された TOTP コードを保留中ユーザーの有効な 2FA 設定に対して検証します。
// バリデーターのエラー (不正・不一致なコードなら *model.ValidationError、システム障害なら素の
// error) は、ハンドラーが分類できるようそのまま返します。成功時は nil を返し、ハンドラーが
// セッションを発行します。
func (uc *CreateSignInTwoFactorUsecase) Execute(ctx context.Context, input CreateSignInTwoFactorInput) error {
	return uc.signInTwoFactorValidator.Validate(ctx, validator.SignInTwoFactorCreateValidatorInput{
		UserID: input.UserID,
		Code:   input.Code,
	})
}
