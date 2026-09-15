package usecase

import (
	"context"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/validator"
)

// CreateSignInUsecaseはサインインフォームを統括します。サインインバリデーターを
// 実行し、認証されたユーザーを返します。セッションの発行 (サインイン) は、アカウント作成と
// 同様にオーケストレーションUseCaseがユーザーを返し、その後ハンドラーがセッションを作る
// 別ステップです。認証は純粋な読み取り (バリデーターがユーザーを引くだけで、ここでは何も
// 永続化しない) のため、トランザクションは取りません。
type CreateSignInUsecase struct {
	signInValidator *validator.SignInCreateValidator
}

// NewCreateSignInUsecaseはサインインバリデーターからCreateSignInUsecaseを
// 構築します。
func NewCreateSignInUsecase(signInValidator *validator.SignInCreateValidator) *CreateSignInUsecase {
	return &CreateSignInUsecase{signInValidator: signInValidator}
}

// CreateSignInInputはExecuteの入力です。送信されたemailとパスワードです。
type CreateSignInInput struct {
	Email    string
	Password string
}

// CreateSignInOutputは認証されたユーザーと、アカウントで2段階認証が有効な場合は
// その設定を運びます。ハンドラーはUserTwoFactorAuthがnilのときにそのユーザーの
// セッションを発行し、そうでなければサインインせずTOTP / リカバリーコードのチャレンジへ
// 迂回させます。
type CreateSignInOutput struct {
	User              *model.User
	UserTwoFactorAuth *model.UserTwoFactorAuth
}

// Executeは送信された資格情報を検証し、認証されたユーザーを返します。バリデーター
// のエラー (不正入力なら *model.ValidationError、システム障害なら素のerror) は、
// ハンドラーが分類できるようそのまま返します。
func (uc *CreateSignInUsecase) Execute(ctx context.Context, input CreateSignInInput) (*CreateSignInOutput, error) {
	output, err := uc.signInValidator.Validate(ctx, validator.SignInCreateValidatorInput{
		Email:    input.Email,
		Password: input.Password,
	})
	if err != nil {
		return nil, err
	}

	return &CreateSignInOutput{
		User:              output.User,
		UserTwoFactorAuth: output.UserTwoFactorAuth,
	}, nil
}
