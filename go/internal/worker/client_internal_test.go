package worker

import (
	"testing"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/email"
)

// TestNewEmailSenderSelectsConfiguredTransport verifies the configuration
// selects the concrete sender used at runtime, including the compatibility
// default for an unset provider.
//
// [Ja] TestNewEmailSenderSelectsConfiguredTransport は、設定により実行時に使われる
// Sender の具象型が選択されることを検証する。プロバイダー未設定時の互換性のための
// デフォルトも含む。
func TestNewEmailSenderSelectsConfiguredTransport(t *testing.T) {
	t.Parallel()

	t.Run("SMTP", func(t *testing.T) {
		t.Parallel()

		sender, err := newEmailSender(&config.Config{
			EmailProvider: config.EmailProviderSMTP,
		})
		if err != nil {
			t.Fatalf("newEmailSender() returned an unexpected error: %v", err)
		}

		if _, ok := sender.(*email.SMTPSender); !ok {
			t.Errorf("newEmailSender() returned %T, want *email.SMTPSender", sender)
		}
	})

	t.Run("Resend", func(t *testing.T) {
		t.Parallel()

		sender, err := newEmailSender(&config.Config{
			EmailProvider: config.EmailProviderResend,
		})
		if err != nil {
			t.Fatalf("newEmailSender() returned an unexpected error: %v", err)
		}

		if _, ok := sender.(*email.ResendSender); !ok {
			t.Errorf("newEmailSender() returned %T, want *email.ResendSender", sender)
		}
	})

	t.Run("unset defaults to Resend", func(t *testing.T) {
		t.Parallel()

		sender, err := newEmailSender(&config.Config{})
		if err != nil {
			t.Fatalf("newEmailSender() returned an unexpected error: %v", err)
		}

		if _, ok := sender.(*email.ResendSender); !ok {
			t.Errorf("newEmailSender() returned %T, want *email.ResendSender", sender)
		}
	})
}
