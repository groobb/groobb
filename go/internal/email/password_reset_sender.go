package email

import (
	"context"

	"github.com/a-h/templ"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/templates/emails/password_reset"
)

// PasswordResetSenderはパスワードリセットメールを描画して送信します。
// ConfirmationSenderと同様にメール種別固有の関心 (件名の翻訳・テンプレート選択) を本型が
// 持つため、呼び出し側 (送信UseCase) はプリミティブ値だけを渡し、templatesやi18nを
// importせずに済みます。
type PasswordResetSender struct {
	sender Sender
}

// NewPasswordResetSenderは与えられた基盤Sender経由で配信する
// PasswordResetSenderを構築します。
func NewPasswordResetSender(sender Sender) *PasswordResetSender {
	return &PasswordResetSender{sender: sender}
}

// Sendは指定ロケールでパスワードリセットメールを描画し、たどるべきリンクとして
// resetURLを提示してtoへ送信します。ロケールはi18nの件名と本文テンプレートの双方を
// 切り替えます。model.Localeは表示言語しか持たないため、default節は未知の値への
// フォールバックではなく、日本語以外の唯一のロケールである英語を表します。
func (s *PasswordResetSender) Send(ctx context.Context, to, resetURL string, locale model.Locale) error {
	ctx = i18n.SetLocale(ctx, locale)
	subject := i18n.T(ctx, "password_reset_email_subject")

	// 有効期間はテンプレートにハードコードせず、ドメイン定数 (ここで時間数に変換) から
	// 取る。これによりメール本文の文言が実際のトークンの有効期限と常に一致する。
	data := password_reset.Data{
		Email:          to,
		ResetURL:       resetURL,
		ExpiresInHours: int(model.PasswordResetTokenExpirationDuration.Hours()),
	}

	var htmlBody, textBody templ.Component
	switch locale {
	case model.LocaleJa:
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
