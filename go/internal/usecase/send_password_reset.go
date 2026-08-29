package usecase

import (
	"context"

	"github.com/groobb/groobb/go/internal/model"
)

// PasswordResetSender renders and sends a password reset mail. Like
// EmailConfirmationSender the interface is declared here, on the consumer side,
// so this package does not import the email or templates packages; main.go (via
// the worker client) injects the concrete email.PasswordResetSender, and tests
// inject a fake.
//
// [Ja] PasswordResetSender はパスワードリセットメールを描画・送信します。
// EmailConfirmationSender と同様にインターフェースを利用側である本パッケージで宣言し、
// email / templates パッケージを import しないようにします。main.go (worker クライアント
// 経由) が具体的な email.PasswordResetSender を注入し、テストではフェイクを注入します。
type PasswordResetSender interface {
	Send(ctx context.Context, to, resetURL string, locale model.Locale) error
}

// SendPasswordResetUsecase sends a password reset mail. It is the worker-side
// UseCase: the recipient and reset link are already decided by the caller that
// enqueued the job, so there is nothing to validate here, only to send.
//
// [Ja] SendPasswordResetUsecase はパスワードリセットメールを送信します。ワーカー側の
// UseCase であり、宛先とリセットリンクはジョブを投入した呼び出し側で既に確定しているため、
// ここに検証するものは無く、送信するだけです。
type SendPasswordResetUsecase struct {
	sender PasswordResetSender
}

// NewSendPasswordResetUsecase builds a SendPasswordResetUsecase backed by the
// given sender.
//
// [Ja] NewSendPasswordResetUsecase は与えられた sender を背後に持つ
// SendPasswordResetUsecase を生成します。
func NewSendPasswordResetUsecase(sender PasswordResetSender) *SendPasswordResetUsecase {
	return &SendPasswordResetUsecase{sender: sender}
}

// SendPasswordResetInput is the input for sending one password reset mail.
//
// [Ja] SendPasswordResetInput は 1 通のパスワードリセットメール送信の入力です。
type SendPasswordResetInput struct {
	Email    string
	ResetURL string
	Locale   model.Locale
}

// Execute sends the password reset mail. Any send failure is returned as-is so
// the worker propagates it to River, which logs and retries the job.
//
// [Ja] Execute はパスワードリセットメールを送信します。送信失敗はそのまま返し、ワーカーが
// River に伝搬します (River がジョブをログ出力・リトライします)。
func (uc *SendPasswordResetUsecase) Execute(ctx context.Context, input SendPasswordResetInput) error {
	return uc.sender.Send(ctx, input.Email, input.ResetURL, input.Locale)
}
