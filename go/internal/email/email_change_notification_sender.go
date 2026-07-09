package email

import (
	"context"

	"github.com/a-h/templ"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/templates/emails/email_change_notification"
)

// EmailChangeNotificationSender renders and sends the mail that notifies a user's
// previous address that their account email was changed. Like the other per-mail
// senders it owns the per-mail concerns (subject translation and template
// selection) so its caller (the send UseCase) passes only primitive values and
// never imports templates or i18n.
//
// [Ja] EmailChangeNotificationSender は、ユーザーの以前のアドレスにアカウントの
// メールアドレスが変更されたことを通知するメールを描画して送信します。他のメール種別
// ごとの Sender と同様にメール種別固有の関心 (件名の翻訳・テンプレート選択) を本型が
// 持つため、呼び出し側 (送信 UseCase) はプリミティブ値だけを渡し、templates や i18n を
// import せずに済みます。
type EmailChangeNotificationSender struct {
	sender Sender
}

// NewEmailChangeNotificationSender builds an EmailChangeNotificationSender that
// delivers through the given base Sender.
//
// [Ja] NewEmailChangeNotificationSender は与えられた基盤 Sender 経由で配信する
// EmailChangeNotificationSender を構築します。
func NewEmailChangeNotificationSender(sender Sender) *EmailChangeNotificationSender {
	return &EmailChangeNotificationSender{sender: sender}
}

// Send renders the notification mail for the given locale and sends it to (the
// user's old address), telling them the account email was changed to newEmail.
// The locale drives both the i18n subject and the body templates; an unknown
// locale falls back to English, matching the i18n default.
//
// [Ja] Send は指定ロケールで通知メールを描画し、アカウントのメールが newEmail に
// 変更されたことを伝えて to (ユーザーの旧アドレス) へ送信します。ロケールは i18n の
// 件名と本文テンプレートの双方を切り替えます。未知のロケールは i18n の既定に合わせて
// 英語へフォールバックします。
func (s *EmailChangeNotificationSender) Send(ctx context.Context, to, newEmail, locale string) error {
	ctx = i18n.SetLocale(ctx, locale)
	subject := i18n.T(ctx, "email_change_notification_subject")

	data := email_change_notification.Data{NewEmail: newEmail}

	var htmlBody, textBody templ.Component
	switch locale {
	case "ja":
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
