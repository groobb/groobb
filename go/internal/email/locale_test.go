package email

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/model"
)

// TestSenders_HTMLBodyLanguageFollowsTheLocaleは各Senderをすべての表示言語で
// 描画し、HTML本文がその言語を宣言していることを確認する。各Senderはdefault節が英語
// であるswitchで本文を選ぶため、本文テンプレートを伴わずにmodel.Locales() へ追加された
// 言語は、その言語に翻訳された件名に英語の本文が付いたメールになる。これを捉えるものは
// 他に無い。switchはコンパイルが通り、Senderごとのテストは集合を走査せずjaとenを
// 1つずつ名指しているためである。
//
// 検証にlang属性を読むのは、ロケール別テンプレートがこれを共有のメールレイアウトへ明示的に
// 渡しており、要求されたロケールではなく選ばれたテンプレートを名指すためである。
func TestSenders_HTMLBodyLanguageFollowsTheLocale(t *testing.T) {
	t.Parallel()

	senders := []struct {
		name string
		send func(ctx context.Context, base Sender, locale model.Locale) error
	}{
		{
			name: "確認コード",
			send: func(ctx context.Context, base Sender, locale model.Locale) error {
				return NewConfirmationSender(base).Send(ctx, "user@example.dev", "482915", locale)
			},
		},
		{
			name: "メールアドレス変更の通知",
			send: func(ctx context.Context, base Sender, locale model.Locale) error {
				return NewEmailChangeNotificationSender(base).Send(ctx, "old@example.dev", "new@example.dev", locale)
			},
		},
		{
			name: "パスワードリセット",
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
					t.Fatalf("Send()のエラー = %v", err)
				}

				if len(noop.SentEmails) != 1 {
					t.Fatalf("len(SentEmails) = %d、期待値 = 1", len(noop.SentEmails))
				}

				html := render(t, noop.SentEmails[0].HTMLBody)
				if want := fmt.Sprintf("lang=%q", locale); !strings.Contains(html, want) {
					t.Errorf("HTML本文に %s が含まれていない", want)
				}
			})
		}
	}
}
