package email

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/model"
)

// TestSenders_HTMLBodyLanguageFollowsTheLocale renders every sender in every
// display language and checks the HTML body declares that language. Each sender
// picks its bodies with a switch whose default branch is English, so a language
// added to model.Locales() without body templates of its own would be mailed an
// English body under a subject translated into that language. Nothing else
// catches it: the switch still compiles, and the per-sender tests name ja and en
// one at a time rather than walking the set.
//
// The assertion reads the lang attribute because the per-locale templates pass it
// to the shared email layout explicitly, so it names the template that was
// chosen rather than the locale that was asked for.
//
// [Ja] TestSenders_HTMLBodyLanguageFollowsTheLocale は各 Sender をすべての表示言語で
// 描画し、HTML 本文がその言語を宣言していることを確認する。各 Sender は default 節が英語
// である switch で本文を選ぶため、本文テンプレートを伴わずに model.Locales() へ追加された
// 言語は、その言語に翻訳された件名に英語の本文が付いたメールになる。これを捉えるものは
// 他に無い。switch はコンパイルが通り、Sender ごとのテストは集合を走査せず ja と en を
// 1 つずつ名指しているためである。
//
// 検証に lang 属性を読むのは、ロケール別テンプレートがこれを共有のメールレイアウトへ明示的に
// 渡しており、要求されたロケールではなく選ばれたテンプレートを名指すためである。
func TestSenders_HTMLBodyLanguageFollowsTheLocale(t *testing.T) {
	t.Parallel()

	senders := []struct {
		name string
		send func(ctx context.Context, base Sender, locale model.Locale) error
	}{
		{
			name: "confirmation",
			send: func(ctx context.Context, base Sender, locale model.Locale) error {
				return NewConfirmationSender(base).Send(ctx, "user@example.dev", "482915", locale)
			},
		},
		{
			name: "email change notification",
			send: func(ctx context.Context, base Sender, locale model.Locale) error {
				return NewEmailChangeNotificationSender(base).Send(ctx, "old@example.dev", "new@example.dev", locale)
			},
		},
		{
			name: "password reset",
			send: func(ctx context.Context, base Sender, locale model.Locale) error {
				return NewPasswordResetSender(base).Send(ctx, "user@example.dev", "https://groobb.example.dev/password/edit?token=opaque-token", locale)
			},
		},
	}

	for _, sender := range senders {
		for _, locale := range model.Locales() {
			t.Run(fmt.Sprintf("%s/%s", sender.name, locale), func(t *testing.T) {
				t.Parallel()

				noop := NewNoopSender()
				if err := sender.send(context.Background(), noop, locale); err != nil {
					t.Fatalf("Send() error = %v", err)
				}

				if len(noop.SentEmails) != 1 {
					t.Fatalf("len(SentEmails) = %d, want 1", len(noop.SentEmails))
				}

				html := render(t, noop.SentEmails[0].HTMLBody)
				if want := fmt.Sprintf("lang=%q", locale); !strings.Contains(html, want) {
					t.Errorf("HTML body missing %s", want)
				}
			})
		}
	}
}
