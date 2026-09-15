package email

import (
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"

	"github.com/groobb/groobb/go/internal/model"
)

// renderは本文の検証のためtemplコンポーネントを文字列へ描画する。
func render(t *testing.T, c templ.Component) string {
	t.Helper()
	var sb strings.Builder
	if err := c.Render(context.Background(), &sb); err != nil {
		t.Fatalf("Render()のエラー = %v", err)
	}
	return sb.String()
}

// TestConfirmationSender_Sendは、Sendが正しいローカライズ済みの件名と本文
// テンプレートを選び基盤Senderに渡すこと (日本語以外のロケールに対してdefault節が
// 選ぶ英語のものを含む) を確認する。
func TestConfirmationSender_Send(t *testing.T) {
	t.Parallel()

	const (
		to   = "user@example.dev"
		code = "482915"
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
			wantSubject:     "[Groobb] 確認用コード",
			wantHTMLSnippet: "確認用コードをお送りします",
			wantTextSnippet: "確認用コードをお送りします",
		},
		{
			name:            "英語",
			locale:          "en",
			wantSubject:     "[Groobb] Confirmation code",
			wantHTMLSnippet: "confirmation code",
			wantTextSnippet: "confirmation code",
		},
		// 表示言語の外のロケールは素の型変換でしかSendに届かず、それを防ぐために
		// model.ParseLocaleがある以上、呼び出し元がこの値を作ることはない。安全網として
		// 残しているケースで、件名と本文が別の言語に割れることなく英語で一貫する。
		{
			name:            "表示言語の外のロケールでも英語のメールになる",
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
			if !strings.Contains(html, code) {
				t.Errorf("HTML本文にコード %q が含まれていない", code)
			}
			if !strings.Contains(html, tt.wantHTMLSnippet) {
				t.Errorf("HTML本文に %q が含まれていない", tt.wantHTMLSnippet)
			}

			text := render(t, sent.TextBody)
			if !strings.Contains(text, code) {
				t.Errorf("テキスト本文にコード %q が含まれていない", code)
			}
			if !strings.Contains(text, tt.wantTextSnippet) {
				t.Errorf("テキスト本文に %q が含まれていない", tt.wantTextSnippet)
			}
		})
	}
}
