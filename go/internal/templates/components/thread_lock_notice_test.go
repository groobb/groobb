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
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// renderThreadLockNotice renders the notice for reasons with the page drawn in
// locale, and returns the markup. The reasons are passed as the domain's own so
// that a case names the condition rather than the presentation form of it.
//
// [Ja] renderThreadLockNotice は、ページを locale で描いた状態で reasons の案内を描画し、
// そのマークアップを返します。理由はドメインのものとして渡すため、各ケースは表示用の形では
// なく条件そのものを名指します。
func renderThreadLockNotice(t *testing.T, locale model.Locale, reasons []model.ThreadLockReason, refusal bool) string {
	t.Helper()

	ctx := i18n.SetLocale(context.Background(), locale)
	data := components.ThreadLockNoticeData{
		Lock:      viewmodel.NewThreadLock(reasons),
		PostLimit: model.ThreadPostLimit,
		Refusal:   refusal,
	}

	var buf bytes.Buffer
	if err := components.ThreadLockNotice(data).Render(ctx, &buf); err != nil {
		t.Fatalf("failed to render: %v", err)
	}

	return buf.String()
}

// TestThreadLockNotice verifies that a thread holding every post it can hold
// says so in either UI language, with the cap the application enforces written
// into the sentence, and that a thread still taking posts draws nothing at all.
//
// [Ja] TestThreadLockNotice は、持てる投稿をすべて持っているスレッドがどちらの UI 言語
// でもその旨を述べること、そしてその文にアプリケーションが適用する上限が書き込まれること、
// また投稿をまだ受け付けるスレッドには何も描かれないことを検証します。
func TestThreadLockNotice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		locale  model.Locale
		reasons []model.ThreadLockReason
		want    string
	}{
		{
			name:    "post limit reached on a Japanese page",
			locale:  model.LocaleJa,
			reasons: []model.ThreadLockReason{model.ThreadLockReasonPostLimitReached},
			want:    "このスレッドは投稿数の上限 (1000 件) に達しました。",
		},
		{
			name:    "post limit reached on an English page",
			locale:  model.LocaleEn,
			reasons: []model.ThreadLockReason{model.ThreadLockReasonPostLimitReached},
			want:    "This thread has reached its limit of 1000 posts.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := renderThreadLockNotice(t, tt.locale, tt.reasons, false); !strings.Contains(got, tt.want) {
				t.Errorf("ロックの案内に %q が含まれていない: %s", tt.want, got)
			}
		})
	}

	t.Run("an unlocked thread draws nothing", func(t *testing.T) {
		t.Parallel()

		if got := renderThreadLockNotice(t, model.LocaleJa, nil, false); got != "" {
			t.Errorf("ロックされていないスレッドの案内 = %q, want 空文字列", got)
		}
	})
}

// TestThreadLockNotice_Refusal verifies that the same reason is announced as an
// alert when it explains a submission that was just refused, and stays a quiet
// notice when it is part of a thread being read. Both say the same thing, but
// only one of them is the answer to something the visitor did.
//
// [Ja] TestThreadLockNotice_Refusal は、同じ理由が、たった今拒否された送信を説明する
// ときはアラートとして読み上げられ、読まれているスレッドの一部であるときは控えめな注記の
// ままであることを検証します。どちらも同じことを述べますが、訪問者が行ったことへの答えで
// あるのは一方だけです。
func TestThreadLockNotice_Refusal(t *testing.T) {
	t.Parallel()

	reasons := []model.ThreadLockReason{model.ThreadLockReasonPostLimitReached}

	refused := renderThreadLockNotice(t, model.LocaleJa, reasons, true)
	if !strings.Contains(refused, `role="alert"`) || !strings.Contains(refused, `data-variant="destructive"`) {
		t.Errorf("拒否された送信のロックの案内がアラートとして描かれていない: %s", refused)
	}

	read := renderThreadLockNotice(t, model.LocaleJa, reasons, false)
	if strings.Contains(read, `role="alert"`) {
		t.Errorf("読まれているスレッドのロックの案内がアラートとして描かれている: %s", read)
	}
}

// TestThreadLockNotice_NextThread verifies that the way on to the next thread
// stands under the notice exactly when the cap is the only thing holding and the
// caller named where a thread is started.
//
// A caller that named none draws no link, which is the state of a page that
// answers a submission without having read the board. A lock that holds for
// another reason as well draws none either: the thread would have been stopped
// even with room left in it, so starting the next one would be walking around
// the decision rather than carrying the conversation on.
//
// [Ja] TestThreadLockNotice_NextThread は、次のスレッドへ向かう道が、上限だけが成立して
// いて、かつ呼び出し側がスレッドを立てる場所を名指したときにちょうど、案内の下に立つことを
// 検証します。
//
// 名指さなかった呼び出し側はリンクを描きません。掲示板を読まずに送信へ応答するページが
// その状態です。別の理由も併せて成立しているロックも描きません。そのスレッドは空きがあって
// も止められていたはずであり、次のスレッドを立てることは、会話を続けることではなくその判断を
// 迂回することになるためです。
func TestThreadLockNotice_NextThread(t *testing.T) {
	t.Parallel()

	limitReached := []model.ThreadLockReason{model.ThreadLockReasonPostLimitReached}
	// A reason the application does not hold yet stands in for the one M3 adds:
	// what the notice does with a second reason is decided here rather than when
	// that reason arrives.
	//
	// [Ja] アプリケーションがまだ持たない理由を、M3 が追加する理由の代わりに置く。2 つ目の
	// 理由に対して案内が何をするかは、その理由が現れたときではなくここで決めておく。
	alsoStopped := append([]model.ThreadLockReason{model.ThreadLockReason("stopped_by_an_admin")}, limitReached...)

	tests := []struct {
		name     string
		locale   model.Locale
		reasons  []model.ThreadLockReason
		path     templates.Path
		wantLink bool
		wantText string
	}{
		{
			name:     "post limit alone on a Japanese page",
			locale:   model.LocaleJa,
			reasons:  limitReached,
			path:     templates.BoardThreadsNewPath("jazz"),
			wantLink: true,
			wantText: "この掲示板で新しいスレッドを立てる",
		},
		{
			name:     "post limit alone on an English page",
			locale:   model.LocaleEn,
			reasons:  limitReached,
			path:     templates.BoardThreadsNewPath("jazz"),
			wantLink: true,
			wantText: "Start a new thread in this board",
		},
		{name: "no board named", locale: model.LocaleJa, reasons: limitReached, wantLink: false},
		{name: "another reason as well", locale: model.LocaleJa, reasons: alsoStopped, path: templates.BoardThreadsNewPath("jazz"), wantLink: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := i18n.SetLocale(context.Background(), tt.locale)
			data := components.ThreadLockNoticeData{
				Lock:                viewmodel.NewThreadLock(tt.reasons),
				PostLimit:           model.ThreadPostLimit,
				BoardThreadsNewPath: tt.path,
			}

			var buf bytes.Buffer
			if err := components.ThreadLockNotice(data).Render(ctx, &buf); err != nil {
				t.Fatalf("failed to render: %v", err)
			}

			got := buf.String()
			hasLink := strings.Contains(got, `href="/b/jazz/threads/new"`)
			if hasLink != tt.wantLink {
				t.Errorf("次のスレッドへのリンクの有無 = %v, want %v: %s", hasLink, tt.wantLink, got)
			}
			if tt.wantLink && !strings.Contains(got, tt.wantText) {
				t.Errorf("リンクの文言 %q が含まれていない: %s", tt.wantText, got)
			}
		})
	}
}

// TestThreadLockNotice_SaysEveryReason verifies that every reason a thread can
// be locked for draws something, in either UI language. A locked thread carries
// neither the reply form nor the way into an account, so a reason with no
// sentence of its own would end the thread with nothing at all: the visitor
// would be unable to write and unable to see why.
//
// [Ja] TestThreadLockNotice_SaysEveryReason は、スレッドをロックしうるどの理由も、
// どちらの UI 言語でも何かを描くことを検証します。ロック中のスレッドは返信フォームも
// アカウントへの導線も持たないため、自身の文を持たない理由はスレッドを何も無い状態で
// 終わらせます。訪問者は書くこともできず、なぜかを知ることもできません。
func TestThreadLockNotice_SaysEveryReason(t *testing.T) {
	t.Parallel()

	for _, locale := range model.Locales() {
		for _, reason := range model.ThreadLockReasons() {
			got := renderThreadLockNotice(t, locale, []model.ThreadLockReason{reason}, false)
			if strings.TrimSpace(got) == "" {
				t.Errorf("%s の %s のロックの案内が空になっている", locale, reason)
			}
		}
	}
}
