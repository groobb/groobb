package email

import (
	"context"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/model"
)

// TestEmailChangeNotificationSender_Sendは、Sendが正しいローカライズ済みの件名と
// 本文テンプレートを選び基盤Senderに渡すこと (日本語以外のロケールに対してdefault節が
// 選ぶ英語のものを含む)、そして新しいアドレスがHTMLとテキスト両方の本文に含まれることを
// 確認する。
func TestEmailChangeNotificationSender_Send(t *testing.T) {
	t.Parallel()

	const (
		to       = "old@example.dev"
		newEmail = "new@example.dev"
	)

	tests := []struct {
		name            string
		locale          model.Locale
		wantSubject     string
		wantHTMLSnippet string
		wantTextSnippet string
	}{
		{
			name:            "日本語",
			locale:          "ja",
			wantSubject:     "[Groobb] メールアドレスが変更されました",
			wantHTMLSnippet: "メールアドレスが",
			wantTextSnippet: "メールアドレスが",
		},
		{
			name:            "英語",
			locale:          "en",
			wantSubject:     "[Groobb] Your email address was changed",
			wantHTMLSnippet: "has been changed",
			wantTextSnippet: "has been changed",
		},
		// 表示言語の外のロケールは素の型変換でしかSendに届かず、それを防ぐために
		// model.ParseLocaleがある以上、呼び出し元がこの値を作ることはない。安全網として
		// 残しているケースで、件名と本文が別の言語に割れることなく英語で一貫する。
		{
			name:            "表示言語の外のロケールでも英語のメールになる",
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
				t.Fatalf("Send()のエラー = %v", err)
			}

			if len(noop.SentEmails) != 1 {
				t.Fatalf("len(SentEmails) = %d、期待値 = 1", len(noop.SentEmails))
			}
			sent := noop.SentEmails[0]

			// メールは新しいアドレスではなく旧アドレスへ配信される。
			if sent.To != to {
				t.Errorf("To = %q、期待値 = %q", sent.To, to)
			}
			if sent.Subject != tt.wantSubject {
				t.Errorf("Subject = %q、期待値 = %q", sent.Subject, tt.wantSubject)
			}

			html := render(t, sent.HTMLBody)
			if !strings.Contains(html, newEmail) {
				t.Errorf("HTML本文に新しいアドレス %q が含まれていない", newEmail)
			}
			if !strings.Contains(html, tt.wantHTMLSnippet) {
				t.Errorf("HTML本文に %q が含まれていない", tt.wantHTMLSnippet)
			}

			text := render(t, sent.TextBody)
			if !strings.Contains(text, newEmail) {
				t.Errorf("テキスト本文に新しいアドレス %q が含まれていない", newEmail)
			}
			if !strings.Contains(text, tt.wantTextSnippet) {
				t.Errorf("テキスト本文に %q が含まれていない", tt.wantTextSnippet)
			}
		})
	}
}
