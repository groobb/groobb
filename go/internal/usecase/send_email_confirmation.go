// Package usecase holds Groobb's application-layer orchestrators. A UseCase
// coordinates data access, authorization, validation, business logic, and
// persistence on behalf of a Handler or Worker, which stay thin adapters.
//
// [Ja] usecase パッケージは Groobb のアプリケーション層のオーケストレーターを保持
// します。UseCase は Handler や Worker (薄い Adapter のまま) に代わって、データ
// アクセス・認可・バリデーション・ビジネスロジック・永続化を統括します。
package usecase

import (
	"context"

	"github.com/groobb/groobb/go/internal/model"
)

// EmailConfirmationSender renders and sends a confirmation code mail. The
// interface is declared here, on the consumer side, so this package does not
// import the email or templates packages; main.go injects the concrete
// email.ConfirmationSender, and tests inject a fake.
//
// [Ja] EmailConfirmationSender は確認コードのメールを描画・送信します。インターフェースは
// 利用側である本パッケージで宣言し、email / templates パッケージを import しないように
// します。main.go が具体的な email.ConfirmationSender を注入し、テストではフェイクを
// 注入します。
type EmailConfirmationSender interface {
	Send(ctx context.Context, to, code string, locale model.Locale) error
}

// SendEmailConfirmationUsecase sends a confirmation code mail. It is the
// worker-side UseCase: the email and code are already decided by the caller that
// enqueued the job, so there is nothing to validate here, only to send.
//
// [Ja] SendEmailConfirmationUsecase は確認コードのメールを送信します。ワーカー側の
// UseCase であり、email と code はジョブを投入した呼び出し側で既に確定しているため、
// ここに検証するものは無く、送信するだけです。
type SendEmailConfirmationUsecase struct {
	sender EmailConfirmationSender
}

// NewSendEmailConfirmationUsecase builds a SendEmailConfirmationUsecase backed by
// the given sender.
//
// [Ja] NewSendEmailConfirmationUsecase は与えられた sender を背後に持つ
// SendEmailConfirmationUsecase を生成します。
func NewSendEmailConfirmationUsecase(sender EmailConfirmationSender) *SendEmailConfirmationUsecase {
	return &SendEmailConfirmationUsecase{sender: sender}
}

// SendEmailConfirmationInput is the input for sending one confirmation mail.
//
// [Ja] SendEmailConfirmationInput は 1 通の確認メール送信の入力です。
type SendEmailConfirmationInput struct {
	Email  string
	Code   string
	Locale model.Locale
}

// Execute sends the confirmation mail. Any send failure is returned as-is so the
// worker propagates it to River, which logs and retries the job.
//
// [Ja] Execute は確認メールを送信します。送信失敗はそのまま返し、ワーカーが River に
// 伝搬します (River がジョブをログ出力・リトライします)。
func (uc *SendEmailConfirmationUsecase) Execute(ctx context.Context, input SendEmailConfirmationInput) error {
	return uc.sender.Send(ctx, input.Email, input.Code, input.Locale)
}
