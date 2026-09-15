// usecaseパッケージはGroobbのアプリケーション層のオーケストレーターを保持
// します。UseCaseはHandlerやWorker (薄いAdapterのまま) に代わって、データ
// アクセス・認可・バリデーション・ビジネスロジック・永続化を統括します。
package usecase

import (
	"context"

	"github.com/groobb/groobb/go/internal/model"
)

// EmailConfirmationSenderは確認コードのメールを描画・送信します。インターフェースは
// 利用側である本パッケージで宣言し、email / templatesパッケージをimportしないように
// します。worker.NewClientが具体的なemail.ConfirmationSenderを組み立てて注入し、
// テストではフェイクを注入します。
type EmailConfirmationSender interface {
	Send(ctx context.Context, to, code string, locale model.Locale) error
}

// SendEmailConfirmationUsecaseは確認コードのメールを送信します。ワーカー側の
// UseCaseであり、emailとcodeはジョブを投入した呼び出し側で既に確定しているため、
// ここに検証するものは無く、送信するだけです。
type SendEmailConfirmationUsecase struct {
	sender EmailConfirmationSender
}

// NewSendEmailConfirmationUsecaseは与えられたsenderを背後に持つ
// SendEmailConfirmationUsecaseを生成します。
func NewSendEmailConfirmationUsecase(sender EmailConfirmationSender) *SendEmailConfirmationUsecase {
	return &SendEmailConfirmationUsecase{sender: sender}
}

// SendEmailConfirmationInputは1通の確認メール送信の入力です。
type SendEmailConfirmationInput struct {
	Email  string
	Code   string
	Locale model.Locale
}

// Executeは確認メールを送信します。送信失敗はそのまま返し、ワーカーがRiverに
// 伝搬します (Riverがジョブをログ出力・リトライします)。
func (uc *SendEmailConfirmationUsecase) Execute(ctx context.Context, input SendEmailConfirmationInput) error {
	return uc.sender.Send(ctx, input.Email, input.Code, input.Locale)
}
