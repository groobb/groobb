package thread_lock_test

import (
	"net/http"
	"testing"

	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/templates"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// TestDeleteは、管理者の解除がスレッドを開き直すこと、そのことが伝えられること、
// そしてフォームが送信されたスレッドで応答することを検証します。フォームは_methodの
// オーバーライドでこのルートへ到達します。ブラウザからそこへ至る手立てはそれだけです。
func TestDelete(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	lock(t, f)

	rec := submit(newRouter(f, f.admin), lockPath(f.thread.ID), unlockForm(), true)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusSeeOther)
	}
	want := templates.ThreadPath(viewmodel.ThreadID(f.thread.ID)).String()
	if got := rec.Header().Get("Location"); got != want {
		t.Errorf("Location = %q、期待値 = %q", got, want)
	}
	if flash := decodeFlash(t, rec); flash.Type != session.FlashSuccess || flash.Message != "スレッドのロックを解除しました" {
		t.Errorf("フラッシュ = %+v、期待値 = 成功の「スレッドのロックを解除しました」", flash)
	}
	if findThread(t, f, f.thread.ID).LockedAt != nil {
		t.Error("解除の後もLockedAtが残っている")
	}
}

// TestDelete_AlreadyUnlockedは、ロックを持たないスレッドからの解除が、ロックを外した
// ときと同じ形で応答されることを検証します。要求が求めたのは管理者のロックが成立していない
// ことであり、実際に成立していないためです。
func TestDelete_AlreadyUnlocked(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	rec := submit(newRouter(f, f.admin), lockPath(f.thread.ID), unlockForm(), true)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusSeeOther)
	}
	if flash := decodeFlash(t, rec); flash.Type != session.FlashSuccess {
		t.Errorf("フラッシュの種類 = %q、期待値 = %q", flash.Type, session.FlashSuccess)
	}
	if findThread(t, f, f.thread.ID).LockedAt != nil {
		t.Error("ロックされていなかったスレッドにLockedAtが付いている")
	}
}

// TestDelete_WithoutCSRFTokenは、トークンの突き合わせ相手であるCookieを伴わずに届いた
// 解除が、ハンドラーへ届く前に拒否されること、そしてロックが残ることを検証します。
func TestDelete_WithoutCSRFToken(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	lock(t, f)

	rec := submit(newRouter(f, f.admin), lockPath(f.thread.ID), unlockForm(), false)

	if rec.Code != http.StatusForbidden {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusForbidden)
	}
	if findThread(t, f, f.thread.ID).LockedAt == nil {
		t.Error("CSRF検証を通らない送信でロックが外れている")
	}
}

// TestDelete_WithoutPermissionは、スレッドを開き直してはならないアカウントが403
// ページで応答されること、そしてロックが残ることを検証します。
func TestDelete_WithoutPermission(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	lock(t, f)

	rec := submit(newRouter(f, f.plain), lockPath(f.thread.ID), unlockForm(), true)

	if rec.Code != http.StatusForbidden {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusForbidden)
	}
	if findThread(t, f, f.thread.ID).LockedAt == nil {
		t.Error("権限の無い操作者の送信でロックが外れている")
	}
}

// TestDelete_UnpublishedThreadは、コミュニティがもう示していないスレッドが、開き直され
// るのではなく404で応答されることを検証します。
func TestDelete_UnpublishedThread(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	lock(t, f)
	unpublish(t, f)

	rec := submit(newRouter(f, f.admin), lockPath(f.thread.ID), unlockForm(), true)

	if rec.Code != http.StatusNotFound {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusNotFound)
	}
	if findThread(t, f, f.thread.ID).LockedAt == nil {
		t.Error("非公開のスレッドのロックが外れている")
	}
}
