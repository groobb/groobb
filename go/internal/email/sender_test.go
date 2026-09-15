package email

import (
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
)

// 両SenderがSenderを満たすことのコンパイル時表明。
var (
	_ Sender = (*ResendSender)(nil)
	_ Sender = (*NoopSender)(nil)
)

func TestNewResendSender(t *testing.T) {
	t.Parallel()

	sender := NewResendSender("test-api-key", "noreply@example.dev", "Groobb")

	if sender.client == nil {
		t.Error("clientがnilになっている")
	}
	if sender.fromEmail != "noreply@example.dev" {
		t.Errorf("fromEmail = %q、期待値 = %q", sender.fromEmail, "noreply@example.dev")
	}
	if sender.fromName != "Groobb" {
		t.Errorf("fromName = %q、期待値 = %q", sender.fromName, "Groobb")
	}
}

func TestResendSender_from(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		fromEmail string
		fromName  string
		want      string
	}{
		{
			name:      "名前あり",
			fromEmail: "noreply@example.dev",
			fromName:  "Groobb",
			want:      "Groobb <noreply@example.dev>",
		},
		{
			name:      "名前なし",
			fromEmail: "noreply@example.dev",
			fromName:  "",
			want:      "noreply@example.dev",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sender := NewResendSender("test-api-key", tt.fromEmail, tt.fromName)
			if got := sender.from(); got != tt.want {
				t.Errorf("from() = %q、期待値 = %q", got, tt.want)
			}
		})
	}
}

func TestNoopSender_Send(t *testing.T) {
	t.Parallel()

	sender := NewNoopSender()
	ctx := context.Background()

	input := SendInput{
		To:       "user@example.dev",
		Subject:  "確認用コード",
		HTMLBody: templ.Raw("<p>body</p>"),
		TextBody: templ.Raw("body"),
	}
	if err := sender.Send(ctx, input); err != nil {
		t.Fatalf("Send()のエラー = %v", err)
	}

	if len(sender.SentEmails) != 1 {
		t.Fatalf("len(SentEmails) = %d、期待値 = 1", len(sender.SentEmails))
	}
	if sender.SentEmails[0].To != "user@example.dev" {
		t.Errorf("SentEmails[0].To = %q、期待値 = %q", sender.SentEmails[0].To, "user@example.dev")
	}
	if sender.SentEmails[0].Subject != "確認用コード" {
		t.Errorf("SentEmails[0].Subject = %q、期待値 = %q", sender.SentEmails[0].Subject, "確認用コード")
	}
}

func TestNoopSender_MultipleSends(t *testing.T) {
	t.Parallel()

	sender := NewNoopSender()
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		if err := sender.Send(ctx, SendInput{To: "user@example.dev", Subject: "test"}); err != nil {
			t.Fatalf("Send()のエラー = %v", err)
		}
	}

	if len(sender.SentEmails) != 3 {
		t.Errorf("len(SentEmails) = %d、期待値 = 3", len(sender.SentEmails))
	}
}

func TestNoopSender_Reset(t *testing.T) {
	t.Parallel()

	sender := NewNoopSender()
	ctx := context.Background()

	if err := sender.Send(ctx, SendInput{To: "user@example.dev", Subject: "test"}); err != nil {
		t.Fatalf("Send()のエラー = %v", err)
	}
	sender.Reset()

	if len(sender.SentEmails) != 0 {
		t.Errorf("Reset()後のlen(SentEmails) = %d、期待値 = 0", len(sender.SentEmails))
	}
}

// TestSendInput_RendersComponentsは実際のtemplコンポーネントを持つSendInput
// が描画できることを確認する。メール種別ごとのSenderが本文を組む流れを模す。
func TestSendInput_RendersComponents(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	input := SendInput{
		To:       "user@example.dev",
		Subject:  "Subject",
		HTMLBody: templ.Raw("<p>HTML_MARKER</p>"),
		TextBody: templ.Raw("TEXT_MARKER"),
	}

	var htmlBuf strings.Builder
	if err := input.HTMLBody.Render(ctx, &htmlBuf); err != nil {
		t.Fatalf("HTMLBody.Render()のエラー = %v", err)
	}
	if !strings.Contains(htmlBuf.String(), "HTML_MARKER") {
		t.Error("描画したHTML本文にHTML_MARKERが含まれていない")
	}

	var textBuf strings.Builder
	if err := input.TextBody.Render(ctx, &textBuf); err != nil {
		t.Fatalf("TextBody.Render()のエラー = %v", err)
	}
	if !strings.Contains(textBuf.String(), "TEXT_MARKER") {
		t.Error("描画したテキスト本文にTEXT_MARKERが含まれていない")
	}
}
