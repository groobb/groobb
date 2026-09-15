package usecase

import (
	"context"

	"github.com/groobb/groobb/go/internal/model"
)

// PasswordResetSenderはパスワードリセットメールを描画・送信します。
// EmailConfirmationSenderと同様にインターフェースを利用側である本パッケージで宣言し、
// email / templatesパッケージをimportしないようにします。worker.NewClientが具体的な
// email.PasswordResetSenderを組み立てて注入し、テストではフェイクを注入します。
type PasswordResetSender interface {
	Send(ctx context.Context, to, resetURL string, locale model.Locale) error
}

// SendPasswordResetUsecaseはパスワードリセットメールを送信します。ワーカー側の
// UseCaseであり、宛先とリセットリンクはジョブを投入した呼び出し側で既に確定しているため、
// ここに検証するものは無く、送信するだけです。
type SendPasswordResetUsecase struct {
	sender PasswordResetSender
}

// NewSendPasswordResetUsecaseは与えられたsenderを背後に持つ
// SendPasswordResetUsecaseを生成します。
func NewSendPasswordResetUsecase(sender PasswordResetSender) *SendPasswordResetUsecase {
	return &SendPasswordResetUsecase{sender: sender}
}

// SendPasswordResetInputは1通のパスワードリセットメール送信の入力です。
type SendPasswordResetInput struct {
	Email    string
	ResetURL string
	Locale   model.Locale
}

// Executeはパスワードリセットメールを送信します。送信失敗はそのまま返し、ワーカーが
// Riverに伝搬します (Riverがジョブをログ出力・リトライします)。
func (uc *SendPasswordResetUsecase) Execute(ctx context.Context, input SendPasswordResetInput) error {
	return uc.sender.Send(ctx, input.Email, input.ResetURL, input.Locale)
}
