package post_unpublication_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/templates"
	"github.com/groobb/groobb/go/internal/validator"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// TestCreate verifies that an administrator's submission takes the post out of
// view, that it says so, and that it answers with the thread the post was
// written in, at the thread's own address rather than at the post's anchor.
//
// [Ja] TestCreateは、管理者の送信が投稿を視界から外すこと、そのことが伝えられること、そして
// 投稿が書かれたスレッドで、投稿のアンカーではなくスレッド自身のアドレスで応答することを
// 検証します。
func TestCreate(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	rec := submit(newRouter(f, f.admin), unpublicationPath(f.thread.ID, f.post.Number), "規約に反する書き込みのため", true)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	want := templates.ThreadPath(viewmodel.ThreadID(f.thread.ID)).String()
	if got := rec.Header().Get("Location"); got != want {
		t.Errorf("Location = %q, want %q", got, want)
	}
	if flash := decodeFlash(t, rec); flash.Type != session.FlashSuccess || flash.Message != "投稿を非公開にしました" {
		t.Errorf("フラッシュ = %+v, want 成功の「投稿を非公開にしました」", flash)
	}
	post := findPost(t, f)
	if post.UnpublishedAt == nil {
		t.Error("非公開の送信の後も UnpublishedAt = nil のままになっている")
	}
	if post.Body != postBody {
		t.Errorf("Body = %q, want %q", post.Body, postBody)
	}
}

// TestCreate_ReasonTooLong verifies that a note beyond what the history holds
// comes back on the confirmation page with 422, keeping what was written so it
// can be shortened, taking the focus to the field it is written in, and that the
// post is left in view.
//
// [Ja] TestCreate_ReasonTooLongは、履歴が保持する範囲を超える注記が422とともに確認ページへ
// 戻り、短くできるように書かれたものを保つこと、それが書かれる入力欄へ焦点を移すこと、そして
// 投稿が視界に残ることを検証します。
func TestCreate_ReasonTooLong(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	reason := strings.Repeat("あ", validator.ModerationReasonMaxLength+1)

	rec := submit(newRouter(f, f.admin), unpublicationPath(f.thread.ID, f.post.Number), reason, true)

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
	if !strings.Contains(body, postBody) {
		t.Errorf("再描画された確認ページが対象の投稿を示していない")
	}
	if !strings.Contains(body, "autofocus") {
		t.Error("再描画された確認ページの理由欄にautofocusが付いていない")
	}
	if findPost(t, f).UnpublishedAt != nil {
		t.Error("拒否された送信で投稿が非公開になっている")
	}
}

// TestCreate_WithoutCSRFToken verifies that a submission arriving without the
// cookie the token is compared against is refused before it reaches the handler,
// and that the post is left in view.
//
// [Ja] TestCreate_WithoutCSRFTokenは、トークンの突き合わせ相手であるCookieを伴わずに届いた
// 送信が、ハンドラーへ届く前に拒否されること、そして投稿が視界に残ることを検証します。
func TestCreate_WithoutCSRFToken(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	rec := submit(newRouter(f, f.admin), unpublicationPath(f.thread.ID, f.post.Number), "", false)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if findPost(t, f).UnpublishedAt != nil {
		t.Error("CSRF検証を通らない送信で投稿が非公開になっている")
	}
}

// TestCreate_WithoutPermission verifies that an account that may not take a post
// out of view is answered with the 403 page and that the post stays in view.
//
// [Ja] TestCreate_WithoutPermissionは、投稿を視界から外してはならないアカウントが403ページ
// で応答されること、そして投稿が視界に残ることを検証します。
func TestCreate_WithoutPermission(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	rec := submit(newRouter(f, f.plain), unpublicationPath(f.thread.ID, f.post.Number), "", true)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if findPost(t, f).UnpublishedAt != nil {
		t.Error("権限の無い操作者の送信で投稿が非公開になっている")
	}
}

// TestCreate_MissingPost verifies that an address naming no post the thread
// issued is answered with the 404 page.
//
// [Ja] TestCreate_MissingPostは、スレッドが発行していない投稿を名指すアドレスが404ページで
// 応答されることを検証します。
func TestCreate_MissingPost(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	rec := submit(newRouter(f, f.admin), unpublicationPath(f.thread.ID, f.post.Number+1000), "", true)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

// TestCreate_UnpublishedThread verifies that a post whose thread was taken out of
// view between the page being opened and the submission arriving is answered
// with 404 rather than unpublished: the thread's own mark already hides it.
//
// [Ja] TestCreate_UnpublishedThreadは、ページが開かれてから送信が届くまでの間にスレッドが
// 視界の外へ移された投稿が、非公開にされるのではなく404で応答されることを検証します。
// スレッド自身の印が既にそれを隠しているためです。
func TestCreate_UnpublishedThread(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	unpublishThread(t, f)

	rec := submit(newRouter(f, f.admin), unpublicationPath(f.thread.ID, f.post.Number), "", true)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if findPost(t, f).UnpublishedAt != nil {
		t.Error("非公開のスレッドの投稿が非公開になっている")
	}
}
