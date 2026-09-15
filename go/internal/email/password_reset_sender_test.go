package email

import (
	"context"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/model"
)

// TestPasswordResetSender_Sendは、Sendが正しいローカライズ済みの件名と本文
// テンプレートを選び基盤Senderに渡すこと (日本語以外のロケールに対してdefault節が
// 選ぶ英語のものを含む)、そしてリセットリンクがHTMLとテキスト両方の本文に含まれることを
// 確認する。
func TestPasswordResetSender_Send(t *testing.T) {
	t.Parallel()

	const (
		to       = "user@example.dev"
		resetURL = "https://groobb.example.dev/password/edit?token=opaque-token"
	)

	tests := []struct {
		name            string
		locale          model.Locale
		wantSubject     string
		wantHTMLSnippet string
		wantTextSnippet string
		wantValidity    string
	}{
		{
			name:            "日本語",
			locale:          "ja",
			wantSubject:     "[Groobb] パスワードの再設定",
			wantHTMLSnippet: "パスワード再設定",
			wantTextSnippet: "パスワード再設定",
			wantValidity:    "1 時間",
		},
		{
			name:            "英語",
			locale:          "en",
			wantSubject:     "[Groobb] Reset your password",
			wantHTMLSnippet: "reset the password",
			wantTextSnippet: "reset the password",
			wantValidity:    "1 hour",
		},
		// 表示言語の外のロケールは素の型変換でしかSendに届かず、それを防ぐために
		// model.ParseLocaleがある以上、呼び出し元がこの値を作ることはない。安全網として
		// 残しているケースで、件名と本文が別の言語に割れることなく英語で一貫する。
		{
			name:            "表示言語の外のロケールでも英語のメールになる",
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
				t.Fatalf("Send()のエラー = %v", err)
			}

			if len(noop.SentEmails) != 1 {
				t.Fatalf("len(SentEmails) = %d、期待値 = 1", len(noop.SentEmails))
			}
			sent := noop.SentEmails[0]

			if sent.To != to {
				t.Errorf("To = %q、期待値 = %q", sent.To, to)
			}
			if sent.Subject != tt.wantSubject {
				t.Errorf("Subject = %q、期待値 = %q", sent.Subject, tt.wantSubject)
			}

			html := render(t, sent.HTMLBody)
			if !strings.Contains(html, resetURL) {
				t.Errorf("HTML本文にリセットURL %q が含まれていない", resetURL)
			}
			if !strings.Contains(html, tt.wantHTMLSnippet) {
				t.Errorf("HTML本文に %q が含まれていない", tt.wantHTMLSnippet)
			}
			// 有効期間は有効期限定数 (1時間) から描画され、ハードコードではないため、
			// ローカライズされた期間が本文に現れる必要がある。
			if !strings.Contains(html, tt.wantValidity) {
				t.Errorf("HTML本文に有効期間 %q が含まれていない", tt.wantValidity)
			}

			text := render(t, sent.TextBody)
			if !strings.Contains(text, resetURL) {
				t.Errorf("テキスト本文にリセットURL %q が含まれていない", resetURL)
			}
			if !strings.Contains(text, tt.wantTextSnippet) {
				t.Errorf("テキスト本文に %q が含まれていない", tt.wantTextSnippet)
			}
			if !strings.Contains(text, tt.wantValidity) {
				t.Errorf("テキスト本文に有効期間 %q が含まれていない", tt.wantValidity)
			}
		})
	}
}
