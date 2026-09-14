package thread_lock_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/templates"
	"github.com/groobb/groobb/go/internal/validator"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// TestCreate verifies that an administrator's submission closes the thread, that
// it says so, and that it answers with the thread the button was pressed on.
//
// [Ja] TestCreateは、管理者の送信がスレッドを閉じること、そのことが伝えられること、そして
// ボタンが押されたスレッドで応答することを検証します。
func TestCreate(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	rec := submit(newRouter(f, f.admin), lockPath(f.thread.ID), lockForm("規約に反する書き込みが続いたため"), true)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	want := templates.ThreadPath(viewmodel.ThreadID(f.thread.ID)).String()
	if got := rec.Header().Get("Location"); got != want {
		t.Errorf("Location = %q, want %q", got, want)
	}
	if flash := decodeFlash(t, rec); flash.Type != session.FlashSuccess || flash.Message != "スレッドをロックしました" {
		t.Errorf("フラッシュ = %+v, want 成功の「スレッドをロックしました」", flash)
	}
	if findThread(t, f, f.thread.ID).LockedAt == nil {
		t.Error("ロックの送信の後も LockedAt = nil のままになっている")
	}
}

// TestCreate_ReasonTooLong verifies that a note beyond what the history holds
// comes back on the confirmation page with 422, keeping what was written so it
// can be shortened, and that the thread is left open.
//
// [Ja] TestCreate_ReasonTooLongは、履歴が保持する範囲を超える注記が422とともに確認ページへ
// 戻り、短くできるように書かれたものを保つこと、そしてスレッドが開いたまま残ることを検証します。
func TestCreate_ReasonTooLong(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	reason := strings.Repeat("あ", validator.ModerationReasonMaxLength+1)

	rec := submit(newRouter(f, f.admin), lockPath(f.thread.ID), lockForm(reason), true)

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
	if findThread(t, f, f.thread.ID).LockedAt != nil {
		t.Error("拒否された送信でスレッドがロックされている")
	}
}

// TestCreate_WithoutCSRFToken verifies that a submission arriving without the
// cookie the token is compared against is refused before it reaches the handler,
// and that the thread is left open.
//
// [Ja] TestCreate_WithoutCSRFTokenは、トークンの突き合わせ相手であるCookieを伴わずに届いた
// 送信が、ハンドラーへ届く前に拒否されること、そしてスレッドが開いたまま残ることを検証します。
func TestCreate_WithoutCSRFToken(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	rec := submit(newRouter(f, f.admin), lockPath(f.thread.ID), lockForm(""), false)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if findThread(t, f, f.thread.ID).LockedAt != nil {
		t.Error("CSRF検証を通らない送信でスレッドがロックされている")
	}
}

// TestCreate_WithoutPermission verifies that an account that may not close a
// thread is answered with the 403 page and that the thread stays open.
//
// [Ja] TestCreate_WithoutPermissionは、スレッドを閉じてはならないアカウントが403ページで
// 応答されること、そしてスレッドが開いたまま残ることを検証します。
func TestCreate_WithoutPermission(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	rec := submit(newRouter(f, f.plain), lockPath(f.thread.ID), lockForm(""), true)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if findThread(t, f, f.thread.ID).LockedAt != nil {
		t.Error("権限の無い操作者の送信でスレッドがロックされている")
	}
}

// TestCreate_UnpublishedThread verifies that a thread taken out of view between
// the page being opened and the submission arriving is answered with 404 rather
// than closed.
//
// [Ja] TestCreate_UnpublishedThreadは、ページが開かれてから送信が届くまでの間に視界の外へ
// 移されたスレッドが、閉じられるのではなく404で応答されることを検証します。
func TestCreate_UnpublishedThread(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	unpublish(t, f)

	rec := submit(newRouter(f, f.admin), lockPath(f.thread.ID), lockForm(""), true)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if findThread(t, f, f.thread.ID).LockedAt != nil {
		t.Error("非公開のスレッドがロックされている")
	}
}
