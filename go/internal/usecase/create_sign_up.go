package usecase

import (
	"context"
	"fmt"

	"github.com/groobb/groobb/go/internal/auth"
	"github.com/groobb/groobb/go/internal/dispatcher"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/validator"
)

// CreateSignUpUsecaseはサインアップ申請を統括します。emailを検証し、メール確認
// コードを発行し、確認を永続化し、コードを届けるメールを投入します。ユーザーは作成
// しません。アカウントはコード検証後のステップで作成されるため、本ステップは
// アドレスに到達可能であることを確認するだけです。
type CreateSignUpUsecase struct {
	signUpValidator       *validator.SignUpCreateValidator
	emailConfirmationRepo *repository.EmailConfirmationRepository
	dispatcher            *dispatcher.Dispatcher
}

// NewCreateSignUpUsecaseはvalidator・repository・dispatcherから
// CreateSignUpUsecaseを構築します。
func NewCreateSignUpUsecase(
	signUpValidator *validator.SignUpCreateValidator,
	emailConfirmationRepo *repository.EmailConfirmationRepository,
	dispatcher *dispatcher.Dispatcher,
) *CreateSignUpUsecase {
	return &CreateSignUpUsecase{
		signUpValidator:       signUpValidator,
		emailConfirmationRepo: emailConfirmationRepo,
		dispatcher:            dispatcher,
	}
}

// CreateSignUpInputはExecuteの入力です。Localeはリクエストのロケールで、確認
// メールをユーザーが閲覧中の言語で描画するために運びます。
type CreateSignUpInput struct {
	Email  string
	Locale model.Locale
}

// CreateSignUpOutputは作成された確認を運び、ハンドラーがコード入力ステップの
// ための受け渡しCookieにそのidを保存できるようにします。
type CreateSignUpOutput struct {
	EmailConfirmation *model.EmailConfirmation
}

// Executeは入力を検証してから確認を発行します。バリデーションを先に走らせ、
// 不正または重複のemailでは行を作らずに *model.ValidationErrorを返します。
func (uc *CreateSignUpUsecase) Execute(ctx context.Context, input CreateSignUpInput) (*CreateSignUpOutput, error) {
	if err := uc.signUpValidator.Validate(ctx, validator.SignUpCreateValidatorInput{
		Email: input.Email,
	}); err != nil {
		return nil, err
	}

	return uc.createSignUp(ctx, input)
}

// createSignUpはコードを生成し、確認を永続化し、メールを投入します。INSERTが
// 1つのためトランザクションは不要で、その前段のコード生成が本ステップの統括する
// ロジックです。
func (uc *CreateSignUpUsecase) createSignUp(ctx context.Context, input CreateSignUpInput) (*CreateSignUpOutput, error) {
	code, err := auth.GenerateConfirmationCode()
	if err != nil {
		return nil, fmt.Errorf("確認コードの生成に失敗: %w", err)
	}

	confirmation, err := uc.emailConfirmationRepo.Create(ctx, repository.CreateEmailConfirmationInput{
		Email: input.Email,
		Event: model.EmailConfirmationEventSignUp,
		Code:  code,
	})
	if err != nil {
		return nil, fmt.Errorf("メール確認の作成に失敗: %w", err)
	}

	// 確認メールを投入する。ここでの失敗は握り潰さずAppErrorとして表面化する。
	// コードを届けられないのにユーザーをコード入力ステップへ進めると、届かないメールを
	// 待ち続けて手詰まりになるため。エラーを返すことでハンドラーはユーザーをサインアップ
	// フォームに留めて再申請させられる。内部原因と対象emailはログ用にのみ添える。
	if err := uc.dispatcher.EnqueueEmailConfirmation(ctx, input.Email, code, input.Locale); err != nil {
		return nil, &model.AppError{
			Code:     model.AppErrCodeInternal,
			UserMsg:  i18n.T(ctx, "validation_email_delivery_failed"),
			Internal: fmt.Errorf("確認メールのジョブ投入に失敗: %w", err),
			Metadata: map[string]string{"email": input.Email},
		}
	}

	return &CreateSignUpOutput{EmailConfirmation: confirmation}, nil
}
