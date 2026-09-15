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

// renderThreadLockNoticeは、ページをlocaleで描いた状態でreasonsの案内を描画し、
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
		t.Fatalf("描画に失敗: %v", err)
	}

	return buf.String()
}

// TestThreadLockNoticeは、どのロック理由も、スレッドを閉じたものをどちらのUI言語でも
// 述べること、上限については文にアプリケーションが適用する件数が書き込まれること、また
// 投稿をまだ受け付けるスレッドには何も描かれないことを検証します。
func TestThreadLockNotice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		locale  model.Locale
		reasons []model.ThreadLockReason
		want    string
	}{
		{
			name:    "日本語のページで投稿数の上限に達した",
			locale:  model.LocaleJa,
			reasons: []model.ThreadLockReason{model.ThreadLockReasonPostLimitReached},
			want:    "このスレッドは投稿数の上限 (1000 件) に達しました。",
		},
		{
			name:    "英語のページで投稿数の上限に達した",
			locale:  model.LocaleEn,
			reasons: []model.ThreadLockReason{model.ThreadLockReasonPostLimitReached},
			want:    "This thread has reached its limit of 1000 posts.",
		},
		{
			name:    "日本語のページでモデレーターがロックした",
			locale:  model.LocaleJa,
			reasons: []model.ThreadLockReason{model.ThreadLockReasonLockedByModerator},
			want:    "このスレッドは管理者によりロックされました。これ以上は書き込めません。",
		},
		{
			name:    "英語のページでモデレーターがロックした",
			locale:  model.LocaleEn,
			reasons: []model.ThreadLockReason{model.ThreadLockReasonLockedByModerator},
			want:    "This thread has been locked by an administrator. Nothing more can be written here.",
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

	t.Run("ロックされていないスレッドは何も描かない", func(t *testing.T) {
		t.Parallel()

		if got := renderThreadLockNotice(t, model.LocaleJa, nil, false); got != "" {
			t.Errorf("ロックされていないスレッドの案内 = %q、期待値 = 空文字列", got)
		}
	})
}

// TestThreadLockNotice_Refusalは、同じ理由が、たった今拒否された送信を説明する
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

// TestThreadLockNotice_ModeratorLockStatedAloneは、管理者が閉じたスレッドが、
// 持てる投稿をすべて持っている場合でも、そのことだけを述べることを検証します。
//
// ここにこれ以上書けない理由への答えは管理者の判断です。その隣に上限の文を描けば、1つの
// 拒否に対して2つ目の独立した説明を差し出すことになり、訪問者がそこからまず読み取るのは、
// 会話が次のスレッドで続くということになります。
func TestThreadLockNotice_ModeratorLockStatedAlone(t *testing.T) {
	t.Parallel()

	got := renderThreadLockNotice(t, model.LocaleJa, []model.ThreadLockReason{
		model.ThreadLockReasonLockedByModerator,
		model.ThreadLockReasonPostLimitReached,
	}, false)

	if !strings.Contains(got, "このスレッドは管理者によりロックされました。") {
		t.Errorf("管理者のロックの文が含まれていない: %s", got)
	}
	if strings.Contains(got, "投稿数の上限") {
		t.Errorf("管理者のロックと並べて上限の文が描かれている: %s", got)
	}
}

// TestThreadLockNotice_NextThreadは、次のスレッドへ向かう道が、上限だけが成立して
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
	moderatorLocked := []model.ThreadLockReason{model.ThreadLockReasonLockedByModerator}
	moderatorLockedAtLimit := []model.ThreadLockReason{
		model.ThreadLockReasonLockedByModerator,
		model.ThreadLockReasonPostLimitReached,
	}

	tests := []struct {
		name     string
		locale   model.Locale
		reasons  []model.ThreadLockReason
		path     templates.Path
		wantLink bool
		wantText string
	}{
		{
			name:     "日本語のページで投稿数の上限だけ",
			locale:   model.LocaleJa,
			reasons:  limitReached,
			path:     templates.BoardThreadsNewPath("jazz"),
			wantLink: true,
			wantText: "この掲示板で新しいスレッドを立てる",
		},
		{
			name:     "英語のページで投稿数の上限だけ",
			locale:   model.LocaleEn,
			reasons:  limitReached,
			path:     templates.BoardThreadsNewPath("jazz"),
			wantLink: true,
			wantText: "Start a new thread in this board",
		},
		{name: "掲示板の指定が無い", locale: model.LocaleJa, reasons: limitReached, wantLink: false},
		{name: "モデレーターのロックだけ", locale: model.LocaleJa, reasons: moderatorLocked, path: templates.BoardThreadsNewPath("jazz"), wantLink: false},
		{name: "モデレーターのロックと投稿数の上限", locale: model.LocaleJa, reasons: moderatorLockedAtLimit, path: templates.BoardThreadsNewPath("jazz"), wantLink: false},
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
				t.Fatalf("描画に失敗: %v", err)
			}

			got := buf.String()
			hasLink := strings.Contains(got, `href="/b/jazz/threads/new"`)
			if hasLink != tt.wantLink {
				t.Errorf("次のスレッドへのリンクの有無 = %v、期待値 = %v: %s", hasLink, tt.wantLink, got)
			}
			if tt.wantLink && !strings.Contains(got, tt.wantText) {
				t.Errorf("リンクの文言 %q が含まれていない: %s", tt.wantText, got)
			}
		})
	}
}

// TestThreadLockNotice_SaysEveryReasonは、スレッドをロックしうるどの理由も、
// どちらのUI言語でも何かを描くことを検証します。ロック中のスレッドは返信フォームも
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
