// emailパッケージはメール送信機能を提供します。Senderインターフェース、
// 本番用の2つのSender (Resend APIを用いるものとSMTPを話すもの)、テスト用の
// no-op Sender、そしてGroobbが送る各メール種別ごとのSenderを含みます。
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

// Senderはtemplコンポーネントからレンダリングしたメールを送信する。
type Sender interface {
	// Sendはinputが表すメールを送信する。
	Send(ctx context.Context, input SendInput) error
}

// SendInputは1通のメール送信の入力。
type SendInput struct {
	// Toは送信先メールアドレス。
	To string

	// Subjectはメールの件名。
	Subject string

	// HTMLBodyはメール本文 (HTML形式)。
	HTMLBody templ.Component

	// TextBodyはメール本文 (テキスト形式)。nilの場合はHTML本文のみを
	// 送信する。
	TextBody templ.Component
}

// ResendSenderはResend API経由でメールを送信する。Senderの本番実装。
type ResendSender struct {
	client    *resend.Client
	fromEmail string
	fromName  string
}

// NewResendSenderはResendSenderを構築する。Fromアドレスと名前はconfigから
// 読まずに明示的に渡すことで、本パッケージをconfigから疎結合に保つ (Senderは
// workerクライアントがconfigの値から構築する)。
func NewResendSender(apiKey, fromEmail, fromName string) *ResendSender {
	// ResendのHTTPクライアントに明示的なタイムアウトを設定し、応答が
	// 返らないリクエストがworkerのgoroutineを無期限にブロックしないようにする。
	httpClient := &http.Client{Timeout: 30 * time.Second}
	return &ResendSender{
		client:    resend.NewCustomClient(httpClient, apiKey),
		fromEmail: fromEmail,
		fromName:  fromName,
	}
}

// fromはFromヘッダーを生成する。名前があれば "Name <email>" 形式、無ければ
// アドレスのみ。
func (s *ResendSender) from() string {
	if s.fromName != "" {
		return fmt.Sprintf("%s <%s>", s.fromName, s.fromEmail)
	}
	return s.fromEmail
}

// Sendは本文をレンダリングし、Resend経由でメールを送信する。
func (s *ResendSender) Send(ctx context.Context, input SendInput) error {
	var htmlBuf bytes.Buffer
	if err := input.HTMLBody.Render(ctx, &htmlBuf); err != nil {
		return fmt.Errorf("HTML本文のレンダリングに失敗: %w", err)
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

// NoopSenderはメールを送信せず記録する。Senderのテスト実装。
type NoopSender struct {
	// SentEmailsはSendに渡された全メールを順に保持し、検証に用いる。
	SentEmails []SendInput
}

// NewNoopSenderは記録が空のNoopSenderを構築する。
func NewNoopSender() *NoopSender {
	return &NoopSender{SentEmails: make([]SendInput, 0)}
}

// Sendはメールを送信せず記録する。
func (s *NoopSender) Send(_ context.Context, input SendInput) error {
	s.SentEmails = append(s.SentEmails, input)
	return nil
}

// Resetは記録済みメールをクリアし、1つのsenderを複数ケースで使い回せる
// ようにする。
func (s *NoopSender) Reset() {
	s.SentEmails = make([]SendInput, 0)
}
