package email

import (
	"context"

	"github.com/a-h/templ"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/templates/emails/email_change_notification"
)

// EmailChangeNotificationSenderは、ユーザーの以前のアドレスにアカウントの
// メールアドレスが変更されたことを通知するメールを描画して送信します。他のメール種別
// ごとのSenderと同様にメール種別固有の関心 (件名の翻訳・テンプレート選択) を本型が
// 持つため、呼び出し側 (送信UseCase) はプリミティブ値だけを渡し、templatesやi18nを
// importせずに済みます。
type EmailChangeNotificationSender struct {
	sender Sender
}

// NewEmailChangeNotificationSenderは与えられた基盤Sender経由で配信する
// EmailChangeNotificationSenderを構築します。
func NewEmailChangeNotificationSender(sender Sender) *EmailChangeNotificationSender {
	return &EmailChangeNotificationSender{sender: sender}
}

// Sendは指定ロケールで通知メールを描画し、アカウントのメールがnewEmailに
// 変更されたことを伝えてto (ユーザーの旧アドレス) へ送信します。ロケールはi18nの
// 件名と本文テンプレートの双方を切り替えます。model.Localeは表示言語しか持たないため、
// default節は未知の値へのフォールバックではなく、日本語以外の唯一のロケールである英語を
// 表します。
func (s *EmailChangeNotificationSender) Send(ctx context.Context, to, newEmail string, locale model.Locale) error {
	ctx = i18n.SetLocale(ctx, locale)
	subject := i18n.T(ctx, "email_change_notification_subject")

	data := email_change_notification.Data{NewEmail: newEmail}

	var htmlBody, textBody templ.Component
	switch locale {
	case model.LocaleJa:
		htmlBody = email_change_notification.JaHTML(data)
		textBody = email_change_notification.JaText(data)
	default:
		htmlBody = email_change_notification.EnHTML(data)
		textBody = email_change_notification.EnText(data)
	}

	return s.sender.Send(ctx, SendInput{
		To:       to,
		Subject:  subject,
		HTMLBody: htmlBody,
		TextBody: textBody,
	})
}
