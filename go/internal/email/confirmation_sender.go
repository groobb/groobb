package email

import (
	"context"

	"github.com/a-h/templ"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/templates/emails/email_confirmation"
)

// ConfirmationSenderはメール確認コードのメールを描画して送信します。メール種別
// 固有の関心 (件名の翻訳・テンプレート選択) を本型が持つため、呼び出し側 (送信UseCase) は
// プリミティブ値だけを渡し、templatesやi18nをimportせずに済みます。
type ConfirmationSender struct {
	sender Sender
}

// NewConfirmationSenderは与えられた基盤Sender経由で配信する
// ConfirmationSenderを構築します。
func NewConfirmationSender(sender Sender) *ConfirmationSender {
	return &ConfirmationSender{sender: sender}
}

// Sendは指定ロケールで確認メールを描画しtoへ送信します。ロケールはi18nの
// 件名と本文テンプレートの双方を切り替えます。model.Localeは表示言語しか持たないため、
// default節は未知の値へのフォールバックではなく、日本語以外の唯一のロケールである英語を
// 表します。
func (s *ConfirmationSender) Send(ctx context.Context, to, code string, locale model.Locale) error {
	ctx = i18n.SetLocale(ctx, locale)
	subject := i18n.T(ctx, "email_confirmation_subject")

	data := email_confirmation.Data{Email: to, Code: code}

	var htmlBody, textBody templ.Component
	switch locale {
	case model.LocaleJa:
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
