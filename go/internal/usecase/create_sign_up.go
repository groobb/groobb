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

// CreateSignUpUsecase orchestrates a sign-up request: it validates the email,
// issues an email confirmation code, persists the confirmation, and enqueues the
// mail that delivers the code. It does not create the user; the account is
// created only after the code is verified (a later task), so this step just
// proves the address is reachable.
//
// [Ja] CreateSignUpUsecase はサインアップ申請を統括します。email を検証し、メール確認
// コードを発行し、確認を永続化し、コードを届けるメールを投入します。ユーザーは作成
// しません。アカウントはコード検証後 (後続タスク) に初めて作成されるため、本ステップは
// アドレスに到達可能であることを確認するだけです。
type CreateSignUpUsecase struct {
	signUpValidator       *validator.SignUpCreateValidator
	emailConfirmationRepo *repository.EmailConfirmationRepository
	dispatcher            *dispatcher.Dispatcher
}

// NewCreateSignUpUsecase builds a CreateSignUpUsecase from its validator,
// repository, and dispatcher.
//
// [Ja] NewCreateSignUpUsecase は validator・repository・dispatcher から
// CreateSignUpUsecase を構築します。
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

// CreateSignUpInput is the input to Execute. Locale is the request locale,
// carried so the confirmation mail is rendered in the language the user is
// browsing in.
//
// [Ja] CreateSignUpInput は Execute の入力です。Locale はリクエストのロケールで、確認
// メールをユーザーが閲覧中の言語で描画するために運びます。
type CreateSignUpInput struct {
	Email  string
	Locale string
}

// CreateSignUpOutput carries the created confirmation so the handler can store
// its id in the handoff cookie for the code-entry step.
//
// [Ja] CreateSignUpOutput は作成された確認を運び、ハンドラーがコード入力ステップの
// ための受け渡し Cookie にその id を保存できるようにします。
type CreateSignUpOutput struct {
	EmailConfirmation *model.EmailConfirmation
}

// Execute validates the input and then issues the confirmation. Validation runs
// first so an invalid or duplicate email returns a *model.ValidationError
// without creating any row.
//
// [Ja] Execute は入力を検証してから確認を発行します。バリデーションを先に走らせ、
// 不正または重複の email では行を作らずに *model.ValidationError を返します。
func (uc *CreateSignUpUsecase) Execute(ctx context.Context, input CreateSignUpInput) (*CreateSignUpOutput, error) {
	if err := uc.signUpValidator.Validate(ctx, validator.SignUpCreateValidatorInput{
		Email: input.Email,
	}); err != nil {
		return nil, err
	}

	return uc.createSignUp(ctx, input)
}

// createSignUp generates the code, persists the confirmation, and enqueues the
// mail. A single INSERT means no transaction is needed; the code generation
// before it is the logic this step orchestrates.
//
// [Ja] createSignUp はコードを生成し、確認を永続化し、メールを投入します。INSERT が
// 1 つのためトランザクションは不要で、その前段のコード生成が本ステップの統括する
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

	// Enqueue the confirmation mail. A failure here is surfaced as an AppError, not
	// swallowed: if the code can never be delivered, advancing the user to the
	// code-entry step would strand them waiting for a mail that will not arrive.
	// Returning the error lets the handler keep them on the sign-up form to retry.
	// The internal cause and the affected email are attached for logging only.
	//
	// [Ja] 確認メールを投入する。ここでの失敗は握り潰さず AppError として表面化する。
	// コードを届けられないのにユーザーをコード入力ステップへ進めると、届かないメールを
	// 待ち続けて手詰まりになるため。エラーを返すことでハンドラーはユーザーをサインアップ
	// フォームに留めて再申請させられる。内部原因と対象 email はログ用にのみ添える。
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
