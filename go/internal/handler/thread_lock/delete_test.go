package thread_lock_test

import (
	"net/http"
	"testing"

	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/templates"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// TestDelete verifies that an administrator's lift reopens the thread, that it
// says so, and that it answers with the thread the form was submitted from. The
// form reaches the route through the _method override, which is the only way it
// is reached from a browser.
//
// [Ja] TestDeleteは、管理者の解除がスレッドを開き直すこと、そのことが伝えられること、
// そしてフォームが送信されたスレッドで応答することを検証します。フォームは_methodの
// オーバーライドでこのルートへ到達します。ブラウザからそこへ至る手立てはそれだけです。
func TestDelete(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	lock(t, f)

	rec := submit(newRouter(f, f.admin), lockPath(f.thread.ID), unlockForm(), true)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	want := templates.ThreadPath(viewmodel.ThreadID(f.thread.ID)).String()
	if got := rec.Header().Get("Location"); got != want {
		t.Errorf("Location = %q, want %q", got, want)
	}
	if flash := decodeFlash(t, rec); flash.Type != session.FlashSuccess || flash.Message != "スレッドのロックを解除しました" {
		t.Errorf("フラッシュ = %+v, want 成功の「スレッドのロックを解除しました」", flash)
	}
	if findThread(t, f, f.thread.ID).LockedAt != nil {
		t.Error("解除の後も LockedAt が残っている")
	}
}

// TestDelete_AlreadyUnlocked verifies that lifting a lock from a thread that
// carries none is answered the way lifting one is: what the request asked for is
// that the administrators' lock not hold, and it does not.
//
// [Ja] TestDelete_AlreadyUnlockedは、ロックを持たないスレッドからの解除が、ロックを外した
// ときと同じ形で応答されることを検証します。要求が求めたのは管理者のロックが成立していない
// ことであり、実際に成立していないためです。
func TestDelete_AlreadyUnlocked(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	rec := submit(newRouter(f, f.admin), lockPath(f.thread.ID), unlockForm(), true)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if flash := decodeFlash(t, rec); flash.Type != session.FlashSuccess {
		t.Errorf("フラッシュの種類 = %q, want %q", flash.Type, session.FlashSuccess)
	}
	if findThread(t, f, f.thread.ID).LockedAt != nil {
		t.Error("ロックされていなかったスレッドに LockedAt が付いている")
	}
}

// TestDelete_WithoutCSRFToken verifies that a lift arriving without the cookie
// the token is compared against is refused before it reaches the handler, and
// that the lock stays on.
//
// [Ja] TestDelete_WithoutCSRFTokenは、トークンの突き合わせ相手であるCookieを伴わずに届いた
// 解除が、ハンドラーへ届く前に拒否されること、そしてロックが残ることを検証します。
func TestDelete_WithoutCSRFToken(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	lock(t, f)

	rec := submit(newRouter(f, f.admin), lockPath(f.thread.ID), unlockForm(), false)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if findThread(t, f, f.thread.ID).LockedAt == nil {
		t.Error("CSRF検証を通らない送信でロックが外れている")
	}
}

// TestDelete_WithoutPermission verifies that an account that may not reopen a
// thread is answered with the 403 page and that the lock stays on.
//
// [Ja] TestDelete_WithoutPermissionは、スレッドを開き直してはならないアカウントが403
// ページで応答されること、そしてロックが残ることを検証します。
func TestDelete_WithoutPermission(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	lock(t, f)

	rec := submit(newRouter(f, f.plain), lockPath(f.thread.ID), unlockForm(), true)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if findThread(t, f, f.thread.ID).LockedAt == nil {
		t.Error("権限の無い操作者の送信でロックが外れている")
	}
}

// TestDelete_UnpublishedThread verifies that a thread the community no longer
// shows is answered with 404 rather than reopened.
//
// [Ja] TestDelete_UnpublishedThreadは、コミュニティがもう示していないスレッドが、開き直され
// るのではなく404で応答されることを検証します。
func TestDelete_UnpublishedThread(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	lock(t, f)
	unpublish(t, f)

	rec := submit(newRouter(f, f.admin), lockPath(f.thread.ID), unlockForm(), true)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if findThread(t, f, f.thread.ID).LockedAt == nil {
		t.Error("非公開のスレッドのロックが外れている")
	}
}
