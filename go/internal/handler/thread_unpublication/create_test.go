package thread_unpublication_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/templates"
	"github.com/groobb/groobb/go/internal/validator"
)

// TestCreate verifies that an administrator's submission takes the thread out of
// the community's view, that it says so, and that it answers with the board the
// thread was listed in, which now stands without it.
//
// [Ja] TestCreateは、管理者の送信がスレッドをコミュニティの視界から外すこと、そのことが
// 伝えられること、そしてスレッドが並んでいた掲示板で応答することを検証します。掲示板は今、
// それを欠いたまま立っています。
func TestCreate(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	rec := submit(newRouter(f, f.admin), unpublicationPath(f.thread.ID), "掲示板の趣旨から外れているため", true)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	want := templates.BoardPath(f.board.Slug).String()
	if got := rec.Header().Get("Location"); got != want {
		t.Errorf("Location = %q, want %q", got, want)
	}
	if flash := decodeFlash(t, rec); flash.Type != session.FlashSuccess || flash.Message != "スレッドを非公開にしました" {
		t.Errorf("フラッシュ = %+v, want 成功の「スレッドを非公開にしました」", flash)
	}
	if findThread(t, f, f.thread.ID).UnpublishedAt == nil {
		t.Error("非公開の送信の後も UnpublishedAt = nil のままになっている")
	}
}

// TestCreate_ReasonTooLong verifies that a note beyond what the history holds
// comes back on the confirmation page with 422, keeping what was written so it
// can be shortened, and that the thread is left in view.
//
// [Ja] TestCreate_ReasonTooLongは、履歴が保持する範囲を超える注記が422とともに確認ページへ
// 戻り、短くできるように書かれたものを保つこと、そしてスレッドが視界に残ることを検証します。
func TestCreate_ReasonTooLong(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	reason := strings.Repeat("あ", validator.ModerationReasonMaxLength+1)

	rec := submit(newRouter(f, f.admin), unpublicationPath(f.thread.ID), reason, true)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
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

// TestCreate_WithoutCSRFToken verifies that a submission arriving without the
// cookie the token is compared against is refused before it reaches the handler,
// and that the thread is left in view.
//
// [Ja] TestCreate_WithoutCSRFTokenは、トークンの突き合わせ相手であるCookieを伴わずに届いた
// 送信が、ハンドラーへ届く前に拒否されること、そしてスレッドが視界に残ることを検証します。
func TestCreate_WithoutCSRFToken(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	rec := submit(newRouter(f, f.admin), unpublicationPath(f.thread.ID), "", false)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if findThread(t, f, f.thread.ID).UnpublishedAt != nil {
		t.Error("CSRF検証を通らない送信でスレッドが非公開になっている")
	}
}

// TestCreate_WithoutPermission verifies that an account that may not take a
// thread out of view is answered with the 403 page and that the thread stays in
// view.
//
// [Ja] TestCreate_WithoutPermissionは、スレッドを視界から外してはならないアカウントが403
// ページで応答されること、そしてスレッドが視界に残ることを検証します。
func TestCreate_WithoutPermission(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	rec := submit(newRouter(f, f.plain), unpublicationPath(f.thread.ID), "", true)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if findThread(t, f, f.thread.ID).UnpublishedAt != nil {
		t.Error("権限の無い操作者の送信でスレッドが非公開になっている")
	}
}

// TestCreate_UnknownThread verifies that an address naming no thread is answered
// with the 404 page.
//
// [Ja] TestCreate_UnknownThreadは、どのスレッドも名指していないアドレスが404ページで応答
// されることを検証します。
func TestCreate_UnknownThread(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	rec := submit(newRouter(f, f.admin), unpublicationPath(f.thread.ID+1000), "", true)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

// TestCreate_AlreadyUnpublished verifies that a thread taken out of view between
// the page being opened and the submission arriving is answered as the operation
// that landed is: what the request asked for is that the community not be shown
// this thread, and it is not shown it.
//
// [Ja] TestCreate_AlreadyUnpublishedは、ページが開かれてから送信が届くまでの間に視界の外へ
// 移されたスレッドが、届いた操作と同じ形で応答されることを検証します。要求が求めたのは
// コミュニティにこのスレッドが示されないことであり、実際に示されていないためです。
func TestCreate_AlreadyUnpublished(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	unpublish(t, f)

	rec := submit(newRouter(f, f.admin), unpublicationPath(f.thread.ID), "", true)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	want := templates.BoardPath(f.board.Slug).String()
	if got := rec.Header().Get("Location"); got != want {
		t.Errorf("Location = %q, want %q", got, want)
	}
	if flash := decodeFlash(t, rec); flash.Type != session.FlashSuccess {
		t.Errorf("フラッシュの種類 = %q, want %q", flash.Type, session.FlashSuccess)
	}
}
