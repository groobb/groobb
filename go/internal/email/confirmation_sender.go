package email

import (
	"context"

	"github.com/a-h/templ"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/templates/emails/email_confirmation"
)

// ConfirmationSender renders and sends the email confirmation code mail. It owns
// the per-mail concerns (subject translation and template selection) so its
// caller (the send UseCase) passes only primitive values and never imports
// templates or i18n.
//
// [Ja] ConfirmationSender はメール確認コードのメールを描画して送信します。メール種別
// 固有の関心 (件名の翻訳・テンプレート選択) を本型が持つため、呼び出し側 (送信 UseCase) は
// プリミティブ値だけを渡し、templates や i18n を import せずに済みます。
type ConfirmationSender struct {
	sender Sender
}

// NewConfirmationSender builds a ConfirmationSender that delivers through the
// given base Sender.
//
// [Ja] NewConfirmationSender は与えられた基盤 Sender 経由で配信する
// ConfirmationSender を構築します。
func NewConfirmationSender(sender Sender) *ConfirmationSender {
	return &ConfirmationSender{sender: sender}
}

// Send renders the confirmation mail for the given locale and sends it to. The
// locale drives both the i18n subject and the body templates; an unknown locale
// falls back to English, matching the i18n default.
//
// [Ja] Send は指定ロケールで確認メールを描画し to へ送信します。ロケールは i18n の
// 件名と本文テンプレートの双方を切り替えます。未知のロケールは i18n の既定に合わせて
// 英語へフォールバックします。
func (s *ConfirmationSender) Send(ctx context.Context, to, code, locale string) error {
	ctx = i18n.SetLocale(ctx, locale)
	subject := i18n.T(ctx, "email_confirmation_subject")

	data := email_confirmation.Data{Email: to, Code: code}

	var htmlBody, textBody templ.Component
	switch locale {
	case "ja":
		htmlBody = email_confirmation.JaHTML(data)
		textBody = email_confirmation.JaText(data)
	default:
		htmlBody = email_confirmation.EnHTML(data)
		textBody = email_confirmation.EnText(data)
	}

	return s.sender.Send(ctx, SendInput{
		To:       to,
		Subject:  subject,
		HTMLBody: htmlBody,
		TextBody: textBody,
	})
}
