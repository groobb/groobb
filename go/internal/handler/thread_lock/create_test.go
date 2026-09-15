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

// TestCreateは、管理者の送信がスレッドを閉じること、そのことが伝えられること、そして
// ボタンが押されたスレッドで応答することを検証します。
func TestCreate(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	rec := submit(newRouter(f, f.admin), lockPath(f.thread.ID), lockForm("規約に反する書き込みが続いたため"), true)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusSeeOther)
	}
	want := templates.ThreadPath(viewmodel.ThreadID(f.thread.ID)).String()
	if got := rec.Header().Get("Location"); got != want {
		t.Errorf("Location = %q、期待値 = %q", got, want)
	}
	if flash := decodeFlash(t, rec); flash.Type != session.FlashSuccess || flash.Message != "スレッドをロックしました" {
		t.Errorf("フラッシュ = %+v、期待値 = 成功の「スレッドをロックしました」", flash)
	}
	if findThread(t, f, f.thread.ID).LockedAt == nil {
		t.Error("ロックの送信の後もLockedAt = nilのままになっている")
	}
}

// TestCreate_ReasonTooLongは、履歴が保持する範囲を超える注記が422とともに確認ページへ
// 戻り、短くできるように書かれたものを保つこと、そしてスレッドが開いたまま残ることを検証します。
func TestCreate_ReasonTooLong(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	reason := strings.Repeat("あ", validator.ModerationReasonMaxLength+1)

	rec := submit(newRouter(f, f.admin), lockPath(f.thread.ID), lockForm(reason), true)

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
	if findThread(t, f, f.thread.ID).LockedAt != nil {
		t.Error("拒否された送信でスレッドがロックされている")
	}
}

// TestCreate_WithoutCSRFTokenは、トークンの突き合わせ相手であるCookieを伴わずに届いた
// 送信が、ハンドラーへ届く前に拒否されること、そしてスレッドが開いたまま残ることを検証します。
func TestCreate_WithoutCSRFToken(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	rec := submit(newRouter(f, f.admin), lockPath(f.thread.ID), lockForm(""), false)

	if rec.Code != http.StatusForbidden {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusForbidden)
	}
	if findThread(t, f, f.thread.ID).LockedAt != nil {
		t.Error("CSRF検証を通らない送信でスレッドがロックされている")
	}
}

// TestCreate_WithoutPermissionは、スレッドを閉じてはならないアカウントが403ページで
// 応答されること、そしてスレッドが開いたまま残ることを検証します。
func TestCreate_WithoutPermission(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	rec := submit(newRouter(f, f.plain), lockPath(f.thread.ID), lockForm(""), true)

	if rec.Code != http.StatusForbidden {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusForbidden)
	}
	if findThread(t, f, f.thread.ID).LockedAt != nil {
		t.Error("権限の無い操作者の送信でスレッドがロックされている")
	}
}

// TestCreate_UnpublishedThreadは、ページが開かれてから送信が届くまでの間に視界の外へ
// 移されたスレッドが、閉じられるのではなく404で応答されることを検証します。
func TestCreate_UnpublishedThread(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	unpublish(t, f)

	rec := submit(newRouter(f, f.admin), lockPath(f.thread.ID), lockForm(""), true)

	if rec.Code != http.StatusNotFound {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusNotFound)
	}
	if findThread(t, f, f.thread.ID).LockedAt != nil {
		t.Error("非公開のスレッドがロックされている")
	}
}
