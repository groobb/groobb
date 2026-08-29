package usecase

import (
	"context"

	"github.com/groobb/groobb/go/internal/model"
)

// EmailChangeNotificationSender renders and sends the mail notifying a user's old
// address that their account email was changed. Like the other mail-sender
// interfaces it is declared here, on the consumer side, so this package does not
// import the email or templates packages; main.go (via the worker client) injects
// the concrete email.EmailChangeNotificationSender, and tests inject a fake.
//
// [Ja] EmailChangeNotificationSender は、ユーザーの旧アドレスにアカウントのメール
// アドレスが変更されたことを通知するメールを描画・送信します。他のメール送信
// インターフェースと同様に利用側である本パッケージで宣言し、email / templates パッケージを
// import しないようにします。main.go (worker クライアント経由) が具体的な
// email.EmailChangeNotificationSender を注入し、テストではフェイクを注入します。
type EmailChangeNotificationSender interface {
	Send(ctx context.Context, to, newEmail string, locale model.Locale) error
}

// SendEmailChangeNotificationUsecase sends the email-change notification mail. It
// is the worker-side UseCase: the recipient (the old address) and the new address
// are already decided by the caller that enqueued the job, so there is nothing to
// validate here, only to send.
//
// [Ja] SendEmailChangeNotificationUsecase はメールアドレス変更通知メールを送信します。
// ワーカー側の UseCase であり、宛先 (旧アドレス) と新しいアドレスはジョブを投入した
// 呼び出し側で既に確定しているため、ここに検証するものは無く、送信するだけです。
type SendEmailChangeNotificationUsecase struct {
	sender EmailChangeNotificationSender
}

// NewSendEmailChangeNotificationUsecase builds a SendEmailChangeNotificationUsecase
// backed by the given sender.
//
// [Ja] NewSendEmailChangeNotificationUsecase は与えられた sender を背後に持つ
// SendEmailChangeNotificationUsecase を生成します。
func NewSendEmailChangeNotificationUsecase(sender EmailChangeNotificationSender) *SendEmailChangeNotificationUsecase {
	return &SendEmailChangeNotificationUsecase{sender: sender}
}

// SendEmailChangeNotificationInput is the input for sending one email-change
// notification mail. Email is the recipient (the old address); NewEmail is the
// address the account was changed to.
//
// [Ja] SendEmailChangeNotificationInput は 1 通のメールアドレス変更通知メール送信の
// 入力です。Email は宛先 (旧アドレス)、NewEmail はアカウントの変更先アドレスです。
type SendEmailChangeNotificationInput struct {
	Email    string
	NewEmail string
	Locale   model.Locale
}

// Execute sends the notification mail. Any send failure is returned as-is so the
// worker propagates it to River, which logs and retries the job.
//
// [Ja] Execute は通知メールを送信します。送信失敗はそのまま返し、ワーカーが River に
// 伝搬します (River がジョブをログ出力・リトライします)。
func (uc *SendEmailChangeNotificationUsecase) Execute(ctx context.Context, input SendEmailChangeNotificationInput) error {
	return uc.sender.Send(ctx, input.Email, input.NewEmail, input.Locale)
}
