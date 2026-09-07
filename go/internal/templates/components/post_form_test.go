package components_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/templates"
	"github.com/groobb/groobb/go/internal/templates/components"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// postFormAction is where the form under test submits: the posts of one thread.
//
// [Ja] postFormAction はテスト対象のフォームの送信先であり、あるスレッドの投稿です。
var postFormAction = templates.ThreadPostsPath(viewmodel.ThreadID(12))

// renderPostForm renders the reply form with the page drawn in locale, and
// returns the markup.
//
// [Ja] renderPostForm は、ページを locale で描いた状態で返信フォームを描画し、その
// マークアップを返します。
func renderPostForm(t *testing.T, locale model.Locale, data components.PostFormData) string {
	t.Helper()

	ctx := i18n.SetLocale(context.Background(), locale)

	var buf bytes.Buffer
	if err := components.PostForm(data).Render(ctx, &buf); err != nil {
		t.Fatalf("failed to render: %v", err)
	}

	return buf.String()
}

// TestPostForm verifies the form's structure in both UI languages: the field is
// named by a visible label, described by the hints stating the limit and the
// notation for quoting, and submitted to the thread's own posts with a CSRF
// token. The two languages are checked together because the structure is what
// the form is; a page that only had it in one language would be a form only half
// the community can use.
//
// [Ja] TestPostForm はフォームの構造を両方の UI 言語で検証します。入力欄が可視ラベルで
// 名付けられ、上限と引用の記法を述べるヒントで説明され、CSRF トークンと共にそのスレッド
// 自身の投稿へ送信されることです。2 つの言語をまとめて確かめるのは、フォームとは構造その
// ものであるためです。片方の言語でしかそれを持たないページは、コミュニティの半分にしか
// 使えないフォームです。
func TestPostForm(t *testing.T) {
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
				`<label for="body"`,
				`<span>本文</span>`,
				`10,000文字以内で入力してください`,
				`&gt;&gt;1 のようにレス番号で引用できます`,
				`続けて投稿するときは10秒の間隔が必要です。`,
				`返信する`,
			},
		},
		{
			name:   "English page",
			locale: model.LocaleEn,
			want: []string{
				`<label for="body"`,
				`<span>Message</span>`,
				`Up to 10,000 characters`,
				`as in &gt;&gt;1`,
				`Posting again takes an interval of 10 seconds.`,
				`Post a reply`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			body := renderPostForm(t, tt.locale, components.PostFormData{
				CSRFToken:           "token",
				Action:              postFormAction,
				PostIntervalSeconds: int(model.PostInterval.Seconds()),
			})

			for _, want := range tt.want {
				if !strings.Contains(body, want) {
					t.Errorf("返信フォームに %q が含まれていない", want)
				}
			}

			for _, want := range []string{
				`action="/t/12/posts"`,
				`method="POST"`,
				`name="csrf_token"`,
				`<textarea`,
				`id="body"`,
				`name="body"`,
				`aria-describedby="body-hint body-reference-hint"`,
			} {
				if !strings.Contains(body, want) {
					t.Errorf("返信フォームに %q が含まれていない", want)
				}
			}

			// The button is submittable: a form that could not be sent would hold
			// every reply written in it.
			//
			// [Ja] ボタンは送信できる。送れないフォームは、そこに書かれた返信をすべて
			// 抱えたままになる。
			if strings.Contains(testutil.OpeningTag(t, body, `type="submit"`), "disabled") {
				t.Error("返信フォームの送信ボタンが無効になっている")
			}
		})
	}
}

// TestPostForm_KeepsTheBody verifies that a form drawn with a body holds it, and
// that a body opening on a blank line keeps it. The textarea's first newline is
// consumed by the HTML parser, so without one added ahead of the text a visitor
// who started their reply with an empty line would get it back one line shorter.
//
// [Ja] TestPostForm_KeepsTheBody は、本文を伴って描かれたフォームがそれを保つこと、
// そして空行から始まる本文がその空行を保つことを検証します。textarea の最初の改行は
// HTML パーサーが消費するため、テキストの前に 1 つ足さなければ、空行から返信を書き始めた
// 訪問者はそれが 1 行短くなって返ってくることになります。
func TestPostForm_KeepsTheBody(t *testing.T) {
	t.Parallel()

	body := renderPostForm(t, model.LocaleJa, components.PostFormData{
		Action: postFormAction,
		Body:   "\n枯葉の名演について",
	})

	if !strings.Contains(body, ">\n\n枯葉の名演について</textarea>") {
		t.Error("返信フォームが、先頭の空行を含む本文をそのまま保っていない")
	}
}

// TestPostForm_FieldError verifies the field error summary in both UI languages,
// including its link to the invalid body. Focus stays on the body so the
// visitor can correct it immediately, and the nearby error remains associated
// with the control.
//
// [Ja] TestPostForm_FieldError は、本文の入力エラー要約と不正な本文へのリンクを両方の
// UI 言語で検証します。すぐに修正できるようフォーカスは本文に置き、入力欄付近のエラーも
// 引き続きその入力欄に関連付けます。
func TestPostForm_FieldError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		locale  model.Locale
		heading string
		label   string
		message string
	}{
		{
			name:    "Japanese page",
			locale:  model.LocaleJa,
			heading: "入力内容を確認してください",
			label:   "本文",
			message: "本文は10,000文字以内で入力してください",
		},
		{
			name:    "English page",
			locale:  model.LocaleEn,
			heading: "Check your entries",
			label:   "Message",
			message: "Please enter a message of no more than 10,000 characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			errors := model.NewValidationError()
			errors.AddField("body", tt.message)

			body := renderPostForm(t, tt.locale, components.PostFormData{
				Action: postFormAction,
				Body:   "長すぎる本文",
				Errors: errors,
			})

			summary := testutil.Element(t, body, `id="post-form-errors"`, "<form")
			for _, want := range []string{
				"<h2>" + tt.heading + "</h2>",
				`href="#body"`,
				tt.label + ": " + tt.message,
			} {
				if !strings.Contains(summary, want) {
					t.Errorf("フォーム冒頭のエラー要約に %q が含まれていない", want)
				}
			}

			textarea := testutil.OpeningTag(t, body, `id="body"`)
			for _, want := range []string{
				`aria-invalid="true"`,
				`aria-describedby="body-hint body-reference-hint body-error-0"`,
				"autofocus",
			} {
				if !strings.Contains(textarea, want) {
					t.Errorf("不正な本文の入力欄に %q が含まれていない", want)
				}
			}
			if strings.Count(body, "autofocus") != 1 {
				t.Error("本文以外にもフォーカス先が設定されている")
			}
			if nearby := testutil.Element(t, body, `id="body-error-0"`, "</p>"); !strings.Contains(nearby, tt.message) {
				t.Error("入力欄付近のエラーに本文のメッセージが含まれていない")
			}
		})
	}
}

// TestPostForm_FormWideErrorLeavesTheFieldAlone verifies that a submission
// refused as a whole is summarized above the form and leaves the body unmarked.
// Waiting out an interval, an account that can no longer post and a save that
// failed are not faults of the text: marking the field would tell the visitor to
// correct something that is already correct, and the caret belongs on the reason
// rather than in the field.
//
// [Ja] TestPostForm_FormWideErrorLeavesTheFieldAlone は、全体として拒否された送信が
// フォームの上に要約され、本文には印が付かないことを検証します。間隔を待つこと、投稿
// できなくなったアカウント、失敗した保存は、いずれもテキストの落ち度ではありません。
// フィールドに印を付ければ、既に正しいものを直すよう訪問者に告げることになり、キャレットが
// 属するのはフィールドの中ではなく理由の側です。
func TestPostForm_FormWideErrorLeavesTheFieldAlone(t *testing.T) {
	t.Parallel()

	errors := model.NewValidationError()
	errors.AddGlobal("投稿の間隔が短すぎます。7秒後にもう一度お試しください。")

	body := renderPostForm(t, model.LocaleJa, components.PostFormData{
		Action: postFormAction,
		Body:   "枯葉の名演について",
		Errors: errors,
	})

	for _, want := range []string{
		`id="post-form-errors" tabindex="-1" autofocus`,
		`role="alert"`,
		`投稿の間隔が短すぎます。7秒後にもう一度お試しください。`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("フォーム全体のエラーを持つ返信フォームに %q が含まれていない", want)
		}
	}

	for _, unwanted := range []string{`data-invalid="true"`, `aria-invalid="true"`, `href="#body"`} {
		if strings.Contains(body, unwanted) {
			t.Errorf("フォーム全体のエラーを持つ返信フォームに %q が含まれている", unwanted)
		}
	}

	if strings.Contains(testutil.Element(t, body, "<textarea", ">"), "autofocus") {
		t.Error("フォーム全体のエラーを持つ返信フォームで、本文にキャレットが置かれている")
	}
}

// TestPostForm_UnsubmittedFormTakesNoFocus verifies that a form that has not
// been submitted opens with the caret nowhere. The form stands at the end of a
// thread somebody came to read, and a page that jumped to its own form would
// take the visitor past the conversation they opened it for.
//
// [Ja] TestPostForm_UnsubmittedFormTakesNoFocus は、まだ送信されていないフォームが
// どこにもキャレットを置かずに開くことを検証します。このフォームは誰かが読みに来た
// スレッドの末尾に立つものであり、自身のフォームへ飛ぶページは、訪問者をそれを開いた
// 目的である会話の先へ連れて行ってしまいます。
func TestPostForm_UnsubmittedFormTakesNoFocus(t *testing.T) {
	t.Parallel()

	body := renderPostForm(t, model.LocaleJa, components.PostFormData{Action: postFormAction})

	if strings.Contains(body, "autofocus") {
		t.Error("送信されていない返信フォームがフォーカスを取っている")
	}
	if strings.Contains(body, `id="post-form-errors"`) {
		t.Error("送信されていない返信フォームにエラー要約が表示されている")
	}
}
