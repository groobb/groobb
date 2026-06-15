package email

import (
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
)

// Compile-time assertions that both senders satisfy Sender.
//
// [Ja] 両 Sender が Sender を満たすことのコンパイル時表明。
var (
	_ Sender = (*ResendSender)(nil)
	_ Sender = (*NoopSender)(nil)
)

func TestNewResendSender(t *testing.T) {
	t.Parallel()

	sender := NewResendSender("test-api-key", "noreply@example.dev", "Groobb")

	if sender.client == nil {
		t.Error("client is nil")
	}
	if sender.fromEmail != "noreply@example.dev" {
		t.Errorf("fromEmail = %q, want %q", sender.fromEmail, "noreply@example.dev")
	}
	if sender.fromName != "Groobb" {
		t.Errorf("fromName = %q, want %q", sender.fromName, "Groobb")
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
			name:      "with name",
			fromEmail: "noreply@example.dev",
			fromName:  "Groobb",
			want:      "Groobb <noreply@example.dev>",
		},
		{
			name:      "without name",
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
				t.Errorf("from() = %q, want %q", got, tt.want)
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
		t.Fatalf("Send() error = %v", err)
	}

	if len(sender.SentEmails) != 1 {
		t.Fatalf("len(SentEmails) = %d, want 1", len(sender.SentEmails))
	}
	if sender.SentEmails[0].To != "user@example.dev" {
		t.Errorf("SentEmails[0].To = %q, want %q", sender.SentEmails[0].To, "user@example.dev")
	}
	if sender.SentEmails[0].Subject != "確認用コード" {
		t.Errorf("SentEmails[0].Subject = %q, want %q", sender.SentEmails[0].Subject, "確認用コード")
	}
}

func TestNoopSender_MultipleSends(t *testing.T) {
	t.Parallel()

	sender := NewNoopSender()
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		if err := sender.Send(ctx, SendInput{To: "user@example.dev", Subject: "test"}); err != nil {
			t.Fatalf("Send() error = %v", err)
		}
	}

	if len(sender.SentEmails) != 3 {
		t.Errorf("len(SentEmails) = %d, want 3", len(sender.SentEmails))
	}
}

func TestNoopSender_Reset(t *testing.T) {
	t.Parallel()

	sender := NewNoopSender()
	ctx := context.Background()

	if err := sender.Send(ctx, SendInput{To: "user@example.dev", Subject: "test"}); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	sender.Reset()

	if len(sender.SentEmails) != 0 {
		t.Errorf("len(SentEmails) after Reset() = %d, want 0", len(sender.SentEmails))
	}
}

// TestSendInput_RendersComponents confirms a SendInput holding real templ
// components can be rendered, mirroring how per-mail senders will build bodies.
//
// [Ja] TestSendInput_RendersComponents は実際の templ コンポーネントを持つ SendInput
// が描画できることを確認する。メール種別ごとの Sender が本文を組む流れを模す。
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
		t.Fatalf("HTMLBody.Render() error = %v", err)
	}
	if !strings.Contains(htmlBuf.String(), "HTML_MARKER") {
		t.Error("expected HTML_MARKER in rendered HTML body")
	}

	var textBuf strings.Builder
	if err := input.TextBody.Render(ctx, &textBuf); err != nil {
		t.Fatalf("TextBody.Render() error = %v", err)
	}
	if !strings.Contains(textBuf.String(), "TEXT_MARKER") {
		t.Error("expected TEXT_MARKER in rendered text body")
	}
}
