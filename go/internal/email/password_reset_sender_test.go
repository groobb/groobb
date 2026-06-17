package email

import (
	"context"
	"strings"
	"testing"
)

// TestPasswordResetSender_Send checks that Send picks the right localized subject
// and body templates and forwards them to the base Sender, including the fallback
// to English for an unknown locale, and that the reset link is present in both
// the HTML and text bodies.
//
// [Ja] TestPasswordResetSender_Send は、Send が正しいローカライズ済みの件名と本文
// テンプレートを選び基盤 Sender に渡すこと (未知ロケールの英語フォールバックを含む)、
// そしてリセットリンクが HTML とテキスト両方の本文に含まれることを確認する。
func TestPasswordResetSender_Send(t *testing.T) {
	t.Parallel()

	const (
		to       = "user@example.dev"
		resetURL = "https://groobb.example.dev/password/edit?token=opaque-token"
	)

	tests := []struct {
		name            string
		locale          string
		wantSubject     string
		wantHTMLSnippet string
		wantTextSnippet string
		wantValidity    string
	}{
		{
			name:            "Japanese",
			locale:          "ja",
			wantSubject:     "[Groobb] パスワードの再設定",
			wantHTMLSnippet: "パスワード再設定",
			wantTextSnippet: "パスワード再設定",
			wantValidity:    "1 時間",
		},
		{
			name:            "English",
			locale:          "en",
			wantSubject:     "[Groobb] Reset your password",
			wantHTMLSnippet: "reset the password",
			wantTextSnippet: "reset the password",
			wantValidity:    "1 hour",
		},
		{
			name:            "unknown locale falls back to English",
			locale:          "fr",
			wantSubject:     "[Groobb] Reset your password",
			wantHTMLSnippet: "reset the password",
			wantTextSnippet: "reset the password",
			wantValidity:    "1 hour",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			noop := NewNoopSender()
			sender := NewPasswordResetSender(noop)

			if err := sender.Send(context.Background(), to, resetURL, tt.locale); err != nil {
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
			if !strings.Contains(html, resetURL) {
				t.Errorf("HTML body missing the reset URL %q", resetURL)
			}
			if !strings.Contains(html, tt.wantHTMLSnippet) {
				t.Errorf("HTML body missing %q", tt.wantHTMLSnippet)
			}
			// The validity window is rendered from the expiry constant (1 hour), not
			// hard-coded, so the localized duration must appear in the body.
			//
			// [Ja] 有効期間は有効期限定数 (1 時間) から描画され、ハードコードではないため、
			// ローカライズされた期間が本文に現れる必要がある。
			if !strings.Contains(html, tt.wantValidity) {
				t.Errorf("HTML body missing the validity window %q", tt.wantValidity)
			}

			text := render(t, sent.TextBody)
			if !strings.Contains(text, resetURL) {
				t.Errorf("text body missing the reset URL %q", resetURL)
			}
			if !strings.Contains(text, tt.wantTextSnippet) {
				t.Errorf("text body missing %q", tt.wantTextSnippet)
			}
			if !strings.Contains(text, tt.wantValidity) {
				t.Errorf("text body missing the validity window %q", tt.wantValidity)
			}
		})
	}
}
