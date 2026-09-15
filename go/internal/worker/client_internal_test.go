package worker

import (
	"testing"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/email"
)

// TestNewEmailSenderSelectsConfiguredTransportは、設定により実行時に使われる
// Senderの具象型が選択されることを検証する。プロバイダー未設定時の互換性のための
// デフォルトも含む。
func TestNewEmailSenderSelectsConfiguredTransport(t *testing.T) {
	t.Parallel()

	t.Run("SMTP", func(t *testing.T) {
		t.Parallel()

		sender, err := newEmailSender(&config.Config{
			EmailProvider: config.EmailProviderSMTP,
		})
		if err != nil {
			t.Fatalf("newEmailSender()が想定外のエラーを返した: %v", err)
		}

		if _, ok := sender.(*email.SMTPSender); !ok {
			t.Errorf("newEmailSender()の戻り値の型 = %T、期待値 = *email.SMTPSender", sender)
		}
	})

	t.Run("Resend", func(t *testing.T) {
		t.Parallel()

		sender, err := newEmailSender(&config.Config{
			EmailProvider: config.EmailProviderResend,
		})
		if err != nil {
			t.Fatalf("newEmailSender()が想定外のエラーを返した: %v", err)
		}

		if _, ok := sender.(*email.ResendSender); !ok {
			t.Errorf("newEmailSender()の戻り値の型 = %T、期待値 = *email.ResendSender", sender)
		}
	})

	t.Run("未設定ならResendになる", func(t *testing.T) {
		t.Parallel()

		sender, err := newEmailSender(&config.Config{})
		if err != nil {
			t.Fatalf("newEmailSender()が想定外のエラーを返した: %v", err)
		}

		if _, ok := sender.(*email.ResendSender); !ok {
			t.Errorf("newEmailSender()の戻り値の型 = %T、期待値 = *email.ResendSender", sender)
		}
	})
}
