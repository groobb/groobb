package email

import (
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
)

// render renders a templ component to a string for body assertions.
//
// [Ja] render は本文の検証のため templ コンポーネントを文字列へ描画する。
func render(t *testing.T, c templ.Component) string {
	t.Helper()
	var sb strings.Builder
	if err := c.Render(context.Background(), &sb); err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	return sb.String()
}

// TestConfirmationSender_Send checks that Send picks the right localized subject
// and body templates and forwards them to the base Sender, including the
// fallback to English for an unknown locale.
//
// [Ja] TestConfirmationSender_Send は、Send が正しいローカライズ済みの件名と本文
// テンプレートを選び基盤 Sender に渡すこと (未知ロケールの英語フォールバックを含む) を
// 確認する。
func TestConfirmationSender_Send(t *testing.T) {
	t.Parallel()

	const (
		to   = "user@example.dev"
		code = "482915"
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
			wantSubject:     "[Groobb] 確認用コード",
			wantHTMLSnippet: "確認用コードをお送りします",
			wantTextSnippet: "確認用コードをお送りします",
		},
		{
			name:            "English",
			locale:          "en",
			wantSubject:     "[Groobb] Confirmation code",
			wantHTMLSnippet: "confirmation code",
			wantTextSnippet: "confirmation code",
		},
		{
			name:            "unknown locale falls back to English",
			locale:          "fr",
			wantSubject:     "[Groobb] Confirmation code",
			wantHTMLSnippet: "confirmation code",
			wantTextSnippet: "confirmation code",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			noop := NewNoopSender()
			sender := NewConfirmationSender(noop)

			if err := sender.Send(context.Background(), to, code, tt.locale); err != nil {
				t.Fatalf("Send() error = %v", err)
			}

			if len(noop.SentEmails) != 1 {
				t.Fatalf("len(SentEmails) = %d, want 1", len(noop.SentEmails))
			}
			sent := noop.SentEmails[0]

			if sent.To != to {
				t.Errorf("To = %q, want %q", sent.To, to)
			}
			if sent.Subject != tt.wantSubject {
				t.Errorf("Subject = %q, want %q", sent.Subject, tt.wantSubject)
			}

			html := render(t, sent.HTMLBody)
			if !strings.Contains(html, code) {
				t.Errorf("HTML body missing the code %q", code)
			}
			if !strings.Contains(html, tt.wantHTMLSnippet) {
				t.Errorf("HTML body missing %q", tt.wantHTMLSnippet)
			}

			text := render(t, sent.TextBody)
			if !strings.Contains(text, code) {
				t.Errorf("text body missing the code %q", code)
			}
			if !strings.Contains(text, tt.wantTextSnippet) {
				t.Errorf("text body missing %q", tt.wantTextSnippet)
			}
		})
	}
}
