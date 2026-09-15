package usecase

import (
	"context"

	"github.com/groobb/groobb/go/internal/model"
)

// EmailChangeNotificationSenderは、ユーザーの旧アドレスにアカウントのメール
// アドレスが変更されたことを通知するメールを描画・送信します。他のメール送信
// インターフェースと同様に利用側である本パッケージで宣言し、email / templatesパッケージを
// importしないようにします。worker.NewClientが具体的な
// email.EmailChangeNotificationSenderを組み立てて注入し、テストではフェイクを注入します。
type EmailChangeNotificationSender interface {
	Send(ctx context.Context, to, newEmail string, locale model.Locale) error
}

// SendEmailChangeNotificationUsecaseはメールアドレス変更通知メールを送信します。
// ワーカー側のUseCaseであり、宛先 (旧アドレス) と新しいアドレスはジョブを投入した
// 呼び出し側で既に確定しているため、ここに検証するものは無く、送信するだけです。
type SendEmailChangeNotificationUsecase struct {
	sender EmailChangeNotificationSender
}

// NewSendEmailChangeNotificationUsecaseは与えられたsenderを背後に持つ
// SendEmailChangeNotificationUsecaseを生成します。
func NewSendEmailChangeNotificationUsecase(sender EmailChangeNotificationSender) *SendEmailChangeNotificationUsecase {
	return &SendEmailChangeNotificationUsecase{sender: sender}
}

// SendEmailChangeNotificationInputは1通のメールアドレス変更通知メール送信の
// 入力です。Emailは宛先 (旧アドレス)、NewEmailはアカウントの変更先アドレスです。
type SendEmailChangeNotificationInput struct {
	Email    string
	NewEmail string
	Locale   model.Locale
}

// Executeは通知メールを送信します。送信失敗はそのまま返し、ワーカーがRiverに
// 伝搬します (Riverがジョブをログ出力・リトライします)。
func (uc *SendEmailChangeNotificationUsecase) Execute(ctx context.Context, input SendEmailChangeNotificationInput) error {
	return uc.sender.Send(ctx, input.Email, input.NewEmail, input.Locale)
}
