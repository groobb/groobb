package email

import (
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"

	"github.com/groobb/groobb/go/internal/model"
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
// and body templates and forwards them to the base Sender, including the English
// ones its default branch selects for a locale that is not Japanese.
//
// [Ja] TestConfirmationSender_Send は、Send が正しいローカライズ済みの件名と本文
// テンプレートを選び基盤 Sender に渡すこと (日本語以外のロケールに対して default 節が
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
		// A locale outside the display languages reaches Send only through a bare
		// conversion, which model.ParseLocale exists to prevent, so no caller produces
		// one. The case is kept as the safety net: the mail stays coherent English
		// rather than splitting its subject and bodies across languages.
		//
		// [Ja] 表示言語の外のロケールは素の型変換でしか Send に届かず、それを防ぐために
		// model.ParseLocale がある以上、呼び出し元がこの値を作ることはない。安全網として
		// 残しているケースで、件名と本文が別の言語に割れることなく英語で一貫する。
		{
			name:            "a locale outside the display languages still yields an English mail",
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
