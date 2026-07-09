package email

import (
	"context"
	"strings"
	"testing"
)

// TestEmailChangeNotificationSender_Send checks that Send picks the right
// localized subject and body templates and forwards them to the base Sender,
// including the fallback to English for an unknown locale, and that the new
// address is present in both the HTML and text bodies.
//
// [Ja] TestEmailChangeNotificationSender_Send は、Send が正しいローカライズ済みの件名と
// 本文テンプレートを選び基盤 Sender に渡すこと (未知ロケールの英語フォールバックを含む)、
// そして新しいアドレスが HTML とテキスト両方の本文に含まれることを確認する。
func TestEmailChangeNotificationSender_Send(t *testing.T) {
	t.Parallel()

	const (
		to       = "old@example.dev"
		newEmail = "new@example.dev"
	)

	tests := []struct {
		name            string
		locale          string
		wantSubject     string
		wantHTMLSnippet string
		wantTextSnippet string
	}{
		{
			name:            "Japanese",
			locale:          "ja",
			wantSubject:     "[Groobb] メールアドレスが変更されました",
			wantHTMLSnippet: "メールアドレスが",
			wantTextSnippet: "メールアドレスが",
		},
		{
			name:            "English",
			locale:          "en",
			wantSubject:     "[Groobb] Your email address was changed",
			wantHTMLSnippet: "has been changed",
			wantTextSnippet: "has been changed",
		},
		{
			name:            "unknown locale falls back to English",
			locale:          "fr",
			wantSubject:     "[Groobb] Your email address was changed",
			wantHTMLSnippet: "has been changed",
			wantTextSnippet: "has been changed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			noop := NewNoopSender()
			sender := NewEmailChangeNotificationSender(noop)

			if err := sender.Send(context.Background(), to, newEmail, tt.locale); err != nil {
				t.Fatalf("Send() error = %v", err)
			}

			if len(noop.SentEmails) != 1 {
				t.Fatalf("len(SentEmails) = %d, want 1", len(noop.SentEmails))
			}
			sent := noop.SentEmails[0]

			// The mail is delivered to the old address, not the new one.
			//
			// [Ja] メールは新しいアドレスではなく旧アドレスへ配信される。
			if sent.To != to {
				t.Errorf("To = %q, want %q", sent.To, to)
			}
			if sent.Subject != tt.wantSubject {
				t.Errorf("Subject = %q, want %q", sent.Subject, tt.wantSubject)
			}

			html := render(t, sent.HTMLBody)
			if !strings.Contains(html, newEmail) {
				t.Errorf("HTML body missing the new address %q", newEmail)
			}
			if !strings.Contains(html, tt.wantHTMLSnippet) {
				t.Errorf("HTML body missing %q", tt.wantHTMLSnippet)
			}

			text := render(t, sent.TextBody)
			if !strings.Contains(text, newEmail) {
				t.Errorf("text body missing the new address %q", newEmail)
			}
			if !strings.Contains(text, tt.wantTextSnippet) {
				t.Errorf("text body missing %q", tt.wantTextSnippet)
			}
		})
	}
}
