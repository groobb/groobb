package usecase

import (
	"context"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/validator"
)

// CreateSignInUsecase orchestrates the sign-in form: it runs the sign-in
// validator and returns the authenticated user. Issuing the session (signing the
// user in) is a separate step the handler runs with CreateSessionUsecase, mirroring
// account creation where the orchestration UseCase returns the user and the handler
// then creates the session. It takes no transaction because authenticating is a
// pure read (the validator looks the user up; nothing is persisted here).
//
// [Ja] CreateSignInUsecase はサインインフォームを統括します。サインインバリデーターを
// 実行し、認証されたユーザーを返します。セッションの発行 (サインイン) は、アカウント作成と
// 同様にオーケストレーション UseCase がユーザーを返し、その後ハンドラーがセッションを作る
// 別ステップです。認証は純粋な読み取り (バリデーターがユーザーを引くだけで、ここでは何も
// 永続化しない) のため、トランザクションは取りません。
type CreateSignInUsecase struct {
	signInValidator *validator.SignInCreateValidator
}

// NewCreateSignInUsecase builds a CreateSignInUsecase from the sign-in validator.
//
// [Ja] NewCreateSignInUsecase はサインインバリデーターから CreateSignInUsecase を
// 構築します。
func NewCreateSignInUsecase(signInValidator *validator.SignInCreateValidator) *CreateSignInUsecase {
	return &CreateSignInUsecase{signInValidator: signInValidator}
}

// CreateSignInInput is the input to Execute: the submitted email and password.
//
// [Ja] CreateSignInInput は Execute の入力です。送信された email とパスワードです。
type CreateSignInInput struct {
	Email    string
	Password string
}

// CreateSignInOutput carries the authenticated user so the handler can issue a
// session for it.
//
// [Ja] CreateSignInOutput は認証されたユーザーを運び、ハンドラーがそのユーザーの
// セッションを発行できるようにします。
type CreateSignInOutput struct {
	User *model.User
}

// Execute validates the submitted credentials and returns the authenticated
// user. The validator's error (a *model.ValidationError for bad input, or a plain
// error for a system failure) is returned unchanged for the handler to classify.
//
// [Ja] Execute は送信された資格情報を検証し、認証されたユーザーを返します。バリデーター
// のエラー (不正入力なら *model.ValidationError、システム障害なら素の error) は、
// ハンドラーが分類できるようそのまま返します。
func (uc *CreateSignInUsecase) Execute(ctx context.Context, input CreateSignInInput) (*CreateSignInOutput, error) {
	user, err := uc.signInValidator.Validate(ctx, validator.SignInCreateValidatorInput{
		Email:    input.Email,
		Password: input.Password,
	})
	if err != nil {
		return nil, err
	}

	return &CreateSignInOutput{User: user}, nil
}
