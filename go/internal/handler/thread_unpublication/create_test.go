package thread_unpublication_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/templates"
	"github.com/groobb/groobb/go/internal/validator"
)

// TestCreateは、管理者の送信がスレッドをコミュニティの視界から外すこと、そのことが
// 伝えられること、そしてスレッドが並んでいた掲示板で応答することを検証します。掲示板は今、
// それを欠いたまま立っています。
func TestCreate(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	rec := submit(newRouter(f, f.admin), unpublicationPath(f.thread.ID), "掲示板の趣旨から外れているため", true)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusSeeOther)
	}
	want := templates.BoardPath(f.board.Slug).String()
	if got := rec.Header().Get("Location"); got != want {
		t.Errorf("Location = %q、期待値 = %q", got, want)
	}
	if flash := decodeFlash(t, rec); flash.Type != session.FlashSuccess || flash.Message != "スレッドを非公開にしました" {
		t.Errorf("フラッシュ = %+v、期待値 = 成功の「スレッドを非公開にしました」", flash)
	}
	if findThread(t, f, f.thread.ID).UnpublishedAt == nil {
		t.Error("非公開の送信の後もUnpublishedAt = nilのままになっている")
	}
}

// TestCreate_ReasonTooLongは、履歴が保持する範囲を超える注記が422とともに確認ページへ
// 戻り、短くできるように書かれたものを保つこと、そしてスレッドが視界に残ることを検証します。
func TestCreate_ReasonTooLong(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	reason := strings.Repeat("あ", validator.ModerationReasonMaxLength+1)

	rec := submit(newRouter(f, f.admin), unpublicationPath(f.thread.ID), reason, true)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusUnprocessableEntity)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "理由は500文字以内で入力してください") {
		t.Errorf("再描画された確認ページに長さの拒否の文言が含まれていない")
	}
	if !strings.Contains(body, reason) {
		t.Errorf("再描画された確認ページに書かれた注記が保たれていない")
	}
	if !strings.Contains(body, f.thread.Title) {
		t.Errorf("再描画された確認ページが対象のスレッドを名指していない")
	}
	if findThread(t, f, f.thread.ID).UnpublishedAt != nil {
		t.Error("拒否された送信でスレッドが非公開になっている")
	}
}

// TestCreate_WithoutCSRFTokenは、トークンの突き合わせ相手であるCookieを伴わずに届いた
// 送信が、ハンドラーへ届く前に拒否されること、そしてスレッドが視界に残ることを検証します。
func TestCreate_WithoutCSRFToken(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	rec := submit(newRouter(f, f.admin), unpublicationPath(f.thread.ID), "", false)

	if rec.Code != http.StatusForbidden {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusForbidden)
	}
	if findThread(t, f, f.thread.ID).UnpublishedAt != nil {
		t.Error("CSRF検証を通らない送信でスレッドが非公開になっている")
	}
}

// TestCreate_WithoutPermissionは、スレッドを視界から外してはならないアカウントが403
// ページで応答されること、そしてスレッドが視界に残ることを検証します。
func TestCreate_WithoutPermission(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	rec := submit(newRouter(f, f.plain), unpublicationPath(f.thread.ID), "", true)

	if rec.Code != http.StatusForbidden {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusForbidden)
	}
	if findThread(t, f, f.thread.ID).UnpublishedAt != nil {
		t.Error("権限の無い操作者の送信でスレッドが非公開になっている")
	}
}

// TestCreate_UnknownThreadは、どのスレッドも名指していないアドレスが404ページで応答
// されることを検証します。
func TestCreate_UnknownThread(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	rec := submit(newRouter(f, f.admin), unpublicationPath(f.thread.ID+1000), "", true)

	if rec.Code != http.StatusNotFound {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusNotFound)
	}
}

// TestCreate_AlreadyUnpublishedは、ページが開かれてから送信が届くまでの間に視界の外へ
// 移されたスレッドが、届いた操作と同じ形で応答されることを検証します。要求が求めたのは
// コミュニティにこのスレッドが示されないことであり、実際に示されていないためです。
func TestCreate_AlreadyUnpublished(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	unpublish(t, f)

	rec := submit(newRouter(f, f.admin), unpublicationPath(f.thread.ID), "", true)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusSeeOther)
	}
	want := templates.BoardPath(f.board.Slug).String()
	if got := rec.Header().Get("Location"); got != want {
		t.Errorf("Location = %q、期待値 = %q", got, want)
	}
	if flash := decodeFlash(t, rec); flash.Type != session.FlashSuccess {
		t.Errorf("フラッシュの種類 = %q、期待値 = %q", flash.Type, session.FlashSuccess)
	}
}
