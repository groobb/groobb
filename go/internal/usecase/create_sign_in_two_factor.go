package usecase

import (
	"context"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/validator"
)

// CreateSignInTwoFactorUsecaseはサインイン時のTOTPチャレンジを統括します。2段階認証
// バリデーターを保留中ユーザーと送信されたコードに対して実行します。セッションの発行
// (サインインの完了) は、パスワードのステップと同様にハンドラーがCreateSessionUsecaseで
// 行う別ステップです (CreateSignInUsecaseが検証し、その後ハンドラーがセッションを作るのと
// 同じ)。セッション発行はリクエストのデータ (IPとUser-Agent) を必要とするためです。
// コード検証は純粋な読み取りで、ここでは何も永続化しないため、トランザクションは取りません。
type CreateSignInTwoFactorUsecase struct {
	signInTwoFactorValidator *validator.SignInTwoFactorCreateValidator
}

// NewCreateSignInTwoFactorUsecaseはサインイン2段階認証バリデーターから
// CreateSignInTwoFactorUsecaseを構築します。
func NewCreateSignInTwoFactorUsecase(signInTwoFactorValidator *validator.SignInTwoFactorCreateValidator) *CreateSignInTwoFactorUsecase {
	return &CreateSignInTwoFactorUsecase{signInTwoFactorValidator: signInTwoFactorValidator}
}

// CreateSignInTwoFactorInputはExecuteの入力です。UserIDは2段階認証Cookieから
// 解決した保留中ユーザー、Codeは送信されたTOTPコードです。
type CreateSignInTwoFactorInput struct {
	UserID model.UserID
	Code   string
}

// Executeは送信されたTOTPコードを保留中ユーザーの有効な2FA設定に対して検証します。
// バリデーターのエラー (不正・不一致なコードなら *model.ValidationError、システム障害なら素の
// error) は、ハンドラーが分類できるようそのまま返します。成功時はnilを返し、ハンドラーが
// セッションを発行します。
func (uc *CreateSignInTwoFactorUsecase) Execute(ctx context.Context, input CreateSignInTwoFactorInput) error {
	return uc.signInTwoFactorValidator.Validate(ctx, validator.SignInTwoFactorCreateValidatorInput{
		UserID: input.UserID,
		Code:   input.Code,
	})
}
