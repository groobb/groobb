// Package email provides email delivery: a Sender interface, a Resend-backed
// production sender, and a no-op sender for tests. Per-mail-type senders
// (confirmation, password reset) and their templates are added in later tasks.
//
// [Ja] email パッケージはメール送信機能を提供します。Sender インターフェース、
// Resend を用いた本番用 Sender、テスト用の no-op Sender を含みます。メール種別
// ごとの Sender (確認コード・パスワードリセット) とそのテンプレートは後続タスクで
// 追加します。
package email

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/a-h/templ"
	"github.com/resend/resend-go/v2"
)

// Sender sends an email rendered from templ components.
// [Ja] Sender は templ コンポーネントからレンダリングしたメールを送信する。
type Sender interface {
	// Send sends the email described by input.
	// [Ja] Send は input が表すメールを送信する。
	Send(ctx context.Context, input SendInput) error
}

// SendInput is the input for sending one email.
// [Ja] SendInput は 1 通のメール送信の入力。
type SendInput struct {
	// To is the recipient email address.
	// [Ja] To は送信先メールアドレス。
	To string

	// Subject is the email subject line.
	// [Ja] Subject はメールの件名。
	Subject string

	// HTMLBody is the HTML body of the email.
	// [Ja] HTMLBody はメール本文 (HTML 形式)。
	HTMLBody templ.Component

	// TextBody is the plain-text body of the email. When nil, only the HTML
	// body is sent.
	//
	// [Ja] TextBody はメール本文 (テキスト形式)。nil の場合は HTML 本文のみを
	// 送信する。
	TextBody templ.Component
}

// ResendSender sends email through the Resend API. It is the production
// implementation of Sender.
//
// [Ja] ResendSender は Resend API 経由でメールを送信する。Sender の本番実装。
type ResendSender struct {
	client    *resend.Client
	fromEmail string
	fromName  string
}

// NewResendSender builds a ResendSender. The from address and name are passed
// explicitly (rather than read from config) so this package stays decoupled
// from config; the worker client constructs the sender from config values.
//
// [Ja] NewResendSender は ResendSender を構築する。From アドレスと名前は config から
// 読まずに明示的に渡すことで、本パッケージを config から疎結合に保つ (Sender は
// worker クライアントが config の値から構築する)。
func NewResendSender(apiKey, fromEmail, fromName string) *ResendSender {
	// Give the Resend HTTP client an explicit timeout so a hung request cannot
	// block a worker goroutine indefinitely.
	//
	// [Ja] Resend の HTTP クライアントに明示的なタイムアウトを設定し、応答が
	// 返らないリクエストが worker の goroutine を無期限にブロックしないようにする。
	httpClient := &http.Client{Timeout: 30 * time.Second}
	return &ResendSender{
		client:    resend.NewCustomClient(httpClient, apiKey),
		fromEmail: fromEmail,
		fromName:  fromName,
	}
}

// from builds the From header: "Name <email>" when a name is set, otherwise the
// bare address.
//
// [Ja] from は From ヘッダーを生成する。名前があれば "Name <email>" 形式、無ければ
// アドレスのみ。
func (s *ResendSender) from() string {
	if s.fromName != "" {
		return fmt.Sprintf("%s <%s>", s.fromName, s.fromEmail)
	}
	return s.fromEmail
}

// Send renders the bodies and sends the email through Resend.
// [Ja] Send は本文をレンダリングし、Resend 経由でメールを送信する。
func (s *ResendSender) Send(ctx context.Context, input SendInput) error {
	var htmlBuf bytes.Buffer
	if err := input.HTMLBody.Render(ctx, &htmlBuf); err != nil {
		return fmt.Errorf("HTML 本文のレンダリングに失敗: %w", err)
	}

	params := &resend.SendEmailRequest{
		From:    s.from(),
		To:      []string{input.To},
		Subject: input.Subject,
		Html:    htmlBuf.String(),
	}

	if input.TextBody != nil {
		var textBuf bytes.Buffer
		if err := input.TextBody.Render(ctx, &textBuf); err != nil {
			return fmt.Errorf("テキスト本文のレンダリングに失敗: %w", err)
		}
		params.Text = textBuf.String()
	}

	if _, err := s.client.Emails.SendWithContext(ctx, params); err != nil {
		return fmt.Errorf("メール送信に失敗: %w", err)
	}
	return nil
}

// NoopSender records emails instead of sending them. It is the test
// implementation of Sender.
//
// [Ja] NoopSender はメールを送信せず記録する。Sender のテスト実装。
type NoopSender struct {
	// SentEmails holds every email passed to Send, in order, for assertions.
	// [Ja] SentEmails は Send に渡された全メールを順に保持し、検証に用いる。
	SentEmails []SendInput
}

// NewNoopSender builds a NoopSender with an empty record.
// [Ja] NewNoopSender は記録が空の NoopSender を構築する。
func NewNoopSender() *NoopSender {
	return &NoopSender{SentEmails: make([]SendInput, 0)}
}

// Send records the email without sending it.
// [Ja] Send はメールを送信せず記録する。
func (s *NoopSender) Send(_ context.Context, input SendInput) error {
	s.SentEmails = append(s.SentEmails, input)
	return nil
}

// Reset clears the recorded emails so one sender can be reused across cases.
// [Ja] Reset は記録済みメールをクリアし、1 つの sender を複数ケースで使い回せる
// ようにする。
func (s *NoopSender) Reset() {
	s.SentEmails = make([]SendInput, 0)
}
