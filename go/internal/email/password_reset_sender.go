package email

import (
	"context"

	"github.com/a-h/templ"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/templates/emails/password_reset"
)

// PasswordResetSender renders and sends the password reset mail. Like
// ConfirmationSender it owns the per-mail concerns (subject translation and
// template selection) so its caller (the send UseCase) passes only primitive
// values and never imports templates or i18n.
//
// [Ja] PasswordResetSender はパスワードリセットメールを描画して送信します。
// ConfirmationSender と同様にメール種別固有の関心 (件名の翻訳・テンプレート選択) を本型が
// 持つため、呼び出し側 (送信 UseCase) はプリミティブ値だけを渡し、templates や i18n を
// import せずに済みます。
type PasswordResetSender struct {
	sender Sender
}

// NewPasswordResetSender builds a PasswordResetSender that delivers through the
// given base Sender.
//
// [Ja] NewPasswordResetSender は与えられた基盤 Sender 経由で配信する
// PasswordResetSender を構築します。
func NewPasswordResetSender(sender Sender) *PasswordResetSender {
	return &PasswordResetSender{sender: sender}
}

// Send renders the password reset mail for the given locale and sends it to,
// presenting resetURL as the link to follow. The locale drives both the i18n
// subject and the body templates; an unknown locale falls back to English,
// matching the i18n default.
//
// [Ja] Send は指定ロケールでパスワードリセットメールを描画し、たどるべきリンクとして
// resetURL を提示して to へ送信します。ロケールは i18n の件名と本文テンプレートの双方を
// 切り替えます。未知のロケールは i18n の既定に合わせて英語へフォールバックします。
func (s *PasswordResetSender) Send(ctx context.Context, to, resetURL, locale string) error {
	ctx = i18n.SetLocale(ctx, locale)
	subject := i18n.T(ctx, "password_reset_email_subject")

	// The validity window comes from the domain constant (converted to whole
	// hours here) rather than being hard-coded in the templates, so the wording in
	// the mail always matches the real token expiry.
	//
	// [Ja] 有効期間はテンプレートにハードコードせず、ドメイン定数 (ここで時間数に変換) から
	// 取る。これによりメール本文の文言が実際のトークンの有効期限と常に一致する。
	data := password_reset.Data{
		Email:          to,
		ResetURL:       resetURL,
		ExpiresInHours: int(model.PasswordResetTokenExpirationDuration.Hours()),
	}

	var htmlBody, textBody templ.Component
	switch locale {
	case "ja":
		htmlBody = password_reset.JaHTML(data)
		textBody = password_reset.JaText(data)
	default:
		htmlBody = password_reset.EnHTML(data)
		textBody = password_reset.EnText(data)
	}

	return s.sender.Send(ctx, SendInput{
		To:       to,
		Subject:  subject,
		HTMLBody: htmlBody,
		TextBody: textBody,
	})
}
