package post_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/templates"
	"github.com/groobb/groobb/go/internal/templates/components"
	"github.com/groobb/groobb/go/internal/templates/pages/post"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// threadID is the thread the refused reply was written in, which the page names
// and links back to.
//
// [Ja] threadID は、拒否された返信が書かれたスレッドであり、ページがそれを名指し、
// そこへ戻るリンクを持ちます。
const threadID = viewmodel.ThreadID(12)

// renderCreate renders the page a refused reply comes back on, with the page
// drawn in locale, and returns the markup.
//
// [Ja] renderCreate は、拒否された返信が戻ってくるページを locale で描画し、その
// マークアップを返します。
func renderCreate(t *testing.T, locale model.Locale, data post.CreatePageData) string {
	t.Helper()

	ctx := i18n.SetLocale(context.Background(), locale)

	var buf bytes.Buffer
	if err := post.Create(data).Render(ctx, &buf); err != nil {
		t.Fatalf("failed to render: %v", err)
	}

	return buf.String()
}

// refusedReply is a reply that was refused over something the visitor can still
// act on, carrying what they wrote and the message about it.
//
// [Ja] refusedReply は、訪問者がまだ手を打てる何かによって拒否された返信で、書かれた
// ものとそれについてのメッセージを持ちます。
func refusedReply(message string) components.PostFormData {
	errors := model.NewValidationError()
	errors.AddGlobal(message)

	return components.PostFormData{
		CSRFToken:           "token",
		Action:              templates.ThreadPostsPath(threadID),
		Body:                "枯葉の名演について",
		PostIntervalSeconds: int(model.PostInterval.Seconds()),
		Errors:              errors,
	}
}

// TestCreate verifies, in both UI languages, that a refused reply comes back on
// a page naming the thread it was written in, linking to it, and holding the
// reply in a form it can be sent from again. The link is the way the visitor
// checks whether the reply is there after an answer that never arrived, so the
// page carries it whatever the refusal was.
//
// [Ja] TestCreate は、拒否された返信が、それが書かれたスレッドを名指し、そこへリンクし、
// もう一度送信できるフォームに返信を保ったページとして戻ってくることを、両方の UI 言語で
// 検証します。このリンクは、答えの届かなかった送信の後で返信がそこにあるかを訪問者が
// 確かめる手立てであるため、拒否の理由が何であってもページはそれを持ちます。
func TestCreate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		locale model.Locale
		want   []string
	}{
		{
			name:   "Japanese page",
			locale: model.LocaleJa,
			want: []string{
				"返信を投稿できませんでした",
				"この返信の投稿先は次のスレッドです。",
				"本文",
				"返信する",
			},
		},
		{
			name:   "English page",
			locale: model.LocaleEn,
			want: []string{
				"Your reply was not posted",
				"This reply was written in the thread below.",
				"Message",
				"Post a reply",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			body := renderCreate(t, tt.locale, post.CreatePageData{
				ThreadID:       threadID,
				ThreadTitle:    "枯葉の名演",
				ThreadLanguage: viewmodel.NewThreadLanguage(model.LocaleJa.ThreadLanguage()),
				PostLimit:      model.ThreadPostLimit,
				Reply:          refusedReply("投稿を保存できませんでした。時間をおいてもう一度お試しください。"),
			})

			for _, want := range tt.want {
				if !strings.Contains(body, want) {
					t.Errorf("再試行の画面に %q が含まれていない", want)
				}
			}

			for _, want := range []string{
				`href="/t/12"`,
				"枯葉の名演",
				`action="/t/12/posts"`,
				`<textarea`,
				">\n枯葉の名演について</textarea>",
				"投稿を保存できませんでした。時間をおいてもう一度お試しください。",
			} {
				if !strings.Contains(body, want) {
					t.Errorf("再試行の画面に %q が含まれていない", want)
				}
			}
		})
	}
}

// TestCreate_LockedThread verifies that a reply refused by a thread that takes
// no post comes back with the reason, with what was written left where it can be
// read and copied, and with no way to send it again. Sending it again would be
// refused again: the visitor's next step is to keep the text, not to press a
// button, and a form offering one would say otherwise.
//
// [Ja] TestCreate_LockedThread は、投稿を受け付けないスレッドに拒否された返信が、理由と
// 共に戻ってくること、書かれたものが読んで写し取れる場所に残ること、そしてもう一度送る
// 手立てが無いことを検証します。もう一度送っても再び拒否されます。訪問者の次の一手は
// ボタンを押すことではなくテキストを控えることであり、ボタンを差し出すフォームはそれとは
// 別のことを述べてしまいます。
func TestCreate_LockedThread(t *testing.T) {
	t.Parallel()

	body := renderCreate(t, model.LocaleJa, post.CreatePageData{
		ThreadID:       threadID,
		ThreadTitle:    "枯葉の名演",
		ThreadLanguage: viewmodel.NewThreadLanguage(model.LocaleJa.ThreadLanguage()),
		Lock:           viewmodel.NewThreadLock([]model.ThreadLockReason{model.ThreadLockReasonPostLimitReached}),
		PostLimit:      model.ThreadPostLimit,
		Reply: components.PostFormData{
			CSRFToken: "token",
			Action:    templates.ThreadPostsPath(threadID),
			Body:      "枯葉の名演について",
		},
	})

	for _, want := range []string{
		"このスレッドは投稿数の上限 (1000 件) に達しました。",
		`id="post-create-lock" tabindex="-1" autofocus`,
		`<label for="body">入力した本文</label>`,
		`readonly`,
		">\n枯葉の名演について</textarea>",
		"この本文はコピーできます。",
		`href="/t/12"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("ロック中のスレッドの再試行の画面に %q が含まれていない", want)
		}
	}

	for _, unwanted := range []string{"<form", "<button", `name="csrf_token"`, "返信する"} {
		if strings.Contains(body, unwanted) {
			t.Errorf("ロック中のスレッドの再試行の画面に %q が含まれている", unwanted)
		}
	}
}

// TestCreate_BodyInAnotherLanguage verifies that the thread's title declares the
// thread's language where it is repeated here, and that the reply declares none.
// A reply in a language other than the thread's is accepted, so a body labelled
// with the thread's language would claim one it is not written in — and a screen
// reader would read it by that language's rules.
//
// [Ja] TestCreate_BodyInAnotherLanguage は、ここに繰り返されるスレッドのタイトルが
// スレッドの言語を宣言すること、そして返信は何も宣言しないことを検証します。スレッドと
// 異なる言語での返信も受け付けるため、スレッドの言語を付けた本文は、それが書かれていない
// 言語を名乗ることになります。そしてスクリーンリーダーはそれをその言語の規則で読み上げ
// ます。
func TestCreate_BodyInAnotherLanguage(t *testing.T) {
	t.Parallel()

	body := renderCreate(t, model.LocaleJa, post.CreatePageData{
		ThreadID:       threadID,
		ThreadTitle:    "Autumn Leaves",
		ThreadLanguage: viewmodel.NewThreadLanguage(model.LocaleEn.ThreadLanguage()),
		PostLimit:      model.ThreadPostLimit,
		Reply:          refusedReply("投稿を保存できませんでした。時間をおいてもう一度お試しください。"),
	})

	if link := testutil.Element(t, body, `href="/t/12"`, "</a>"); !strings.Contains(link, `lang="en"`) {
		t.Errorf("投稿先のスレッドのタイトルがその言語を宣言していない: %s", link)
	}

	if textarea := testutil.Element(t, body, "<textarea", ">"); strings.Contains(textarea, "lang=") {
		t.Errorf("返信の本文が言語を宣言している: %s", textarea)
	}
}
