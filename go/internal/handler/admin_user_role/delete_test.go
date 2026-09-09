package admin_user_role_test

import (
	"net/http"
	"testing"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/templates"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// rolePath is the address of one role an account holds, which the revoke
// submits to.
//
// [Ja] rolePath は、アカウントが持つ 1 つのロールのアドレスで、剥奪の送信先です。
func rolePath(id model.UserID, roleName model.RoleName) string {
	return templates.AdminUserRolePath(viewmodel.UserID(id), string(roleName)).String()
}

// TestDelete_RevokesTheRole verifies that one administrator taking the role from
// another removes it, says so, and returns to the page of the listing the button
// was pressed on.
//
// [Ja] TestDelete_RevokesTheRole は、ある管理者が別の管理者からロールを取り上げるとそれが
// 外れること、そのことが伝えられること、そしてボタンが押された一覧のページへ戻ることを
// 検証します。
func TestDelete_RevokesTheRole(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	adminID := buildAdministrator(t, f.db, "adminuser")
	targetID := buildAdministrator(t, f.db, "otheradmin")

	rec := submit(newRouter(f, adminID), rolePath(targetID, model.RoleNameAdmin), revokeForm("other", "3"), true)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	want := templates.AdminUsersPagePath("other", 3).String()
	if got := rec.Header().Get("Location"); got != want {
		t.Errorf("Location = %q, want %q", got, want)
	}
	if flash := decodeFlash(t, rec); flash.Type != session.FlashSuccess || flash.Message != "@otheradminから管理者ロールを剥奪しました" {
		t.Errorf("フラッシュ = %+v, want 成功の「@otheradminから管理者ロールを剥奪しました」", flash)
	}
	if holdsAdmin(t, f.db, targetID) {
		t.Error("剥奪した相手が admin ロールを持ったままになっている")
	}
}

// TestDelete_FromSelf verifies that an administrator stepping down is admitted
// while another administrator remains, and is sent to the home page instead of
// back to the listing: the listing is the first thing they can no longer read.
//
// [Ja] TestDelete_FromSelf は、他に管理者が残っている限り、管理者が自ら降りることが許され、
// 一覧ではなくホームへ送られることを検証します。一覧は真っ先に読めなくなるものであるためです。
func TestDelete_FromSelf(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	adminID := buildAdministrator(t, f.db, "adminuser")
	buildAdministrator(t, f.db, "otheradmin")

	rec := submit(newRouter(f, adminID), rolePath(adminID, model.RoleNameAdmin), revokeForm("", ""), true)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got, want := rec.Header().Get("Location"), templates.HomePath().String(); got != want {
		t.Errorf("Location = %q, want %q", got, want)
	}
	if holdsAdmin(t, f.db, adminID) {
		t.Error("自分から外した admin ロールが残っている")
	}
}

// TestDelete_LastAdministrator verifies that the community is not left without
// an administrator: the revoke is refused, the reason comes back on the listing
// as a flash, and the role is still held.
//
// The refusal is answered with the listing rather than with a page, because the
// rows around it are what the visitor does next — hand the role to somebody else
// first, or leave it where it is.
//
// [Ja] TestDelete_LastAdministrator は、コミュニティが管理者のいない状態にならないことを
// 検証します。剥奪は拒否され、その理由はフラッシュとして一覧に戻り、ロールは保たれたままです。
//
// 拒否にページではなく一覧で応答するのは、その周りの行こそが訪問者の次の手立てであるためです。
// 先に誰かへロールを渡すか、そのままにしておくかです。
func TestDelete_LastAdministrator(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	adminID := buildAdministrator(t, f.db, "adminuser")

	rec := submit(newRouter(f, adminID), rolePath(adminID, model.RoleNameAdmin), revokeForm("admin", "1"), true)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	want := templates.AdminUsersPagePath("admin", 1).String()
	if got := rec.Header().Get("Location"); got != want {
		t.Errorf("Location = %q, want %q", got, want)
	}
	flash := decodeFlash(t, rec)
	if flash.Type != session.FlashError {
		t.Errorf("フラッシュの種類 = %q, want %q", flash.Type, session.FlashError)
	}
	if flash.Message != "最後の管理者から管理者ロールを外すことはできません。先に別の利用者を管理者にしてください。" {
		t.Errorf("フラッシュの本文 = %q, want 最後の管理者の拒否の文言", flash.Message)
	}
	if !holdsAdmin(t, f.db, adminID) {
		t.Error("最後の管理者から admin ロールが外れている")
	}
}

// TestDelete_WithoutPermission verifies that an account that may not take roles
// away is answered with the 403 page, and that the role is still held.
//
// [Ja] TestDelete_WithoutPermission は、ロールを取り上げてはならないアカウントが 403
// ページで応答されること、そしてロールが保たれたままであることを検証します。
func TestDelete_WithoutPermission(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	actorID := testutil.NewUserBuilder(t, f.db).WithAtname("plainuser").Build()
	targetID := buildAdministrator(t, f.db, "adminuser")

	rec := submit(newRouter(f, actorID), rolePath(targetID, model.RoleNameAdmin), revokeForm("", ""), true)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if !holdsAdmin(t, f.db, targetID) {
		t.Error("権限の無い操作者の送信で admin ロールが外れている")
	}
}

// TestDelete_NamesNothing verifies that an address naming no account or no role
// is answered with the 404 page.
//
// [Ja] TestDelete_NamesNothing は、どのアカウントもどのロールも名指していないアドレスが
// 404 ページで応答されることを検証します。
func TestDelete_NamesNothing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path func(adminID model.UserID) string
	}{
		{
			name: "id が整数ではない",
			path: func(model.UserID) string { return "/admin/users/abc/roles/admin" },
		},
		{
			name: "アカウントが存在しない",
			path: func(adminID model.UserID) string { return rolePath(adminID+1, model.RoleNameAdmin) },
		},
		{
			name: "ロールが存在しない",
			path: func(adminID model.UserID) string { return rolePath(adminID, model.RoleName("moderator")) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := newFixture(t)
			adminID := buildAdministrator(t, f.db, "adminuser")

			rec := submit(newRouter(f, adminID), tt.path(adminID), revokeForm("", ""), true)

			if rec.Code != http.StatusNotFound {
				t.Errorf("status code = %d, want %d", rec.Code, http.StatusNotFound)
			}
		})
	}
}

// TestDelete_WithoutCSRFToken verifies that a revocation arriving without the
// token the form embeds is refused before the handler, and that the role is
// still held. A revoke a link on another site could trigger would let anyone
// take the community's administrators away.
//
// [Ja] TestDelete_WithoutCSRFToken は、フォームが埋め込むトークンを伴わずに届いた剥奪が
// ハンドラーの手前で拒否されること、そしてロールが保たれたままであることを検証します。
// 他サイトのリンクが起こせる剥奪は、誰もがコミュニティから管理者を奪えることを意味する
// ためです。
func TestDelete_WithoutCSRFToken(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	adminID := buildAdministrator(t, f.db, "adminuser")
	targetID := buildAdministrator(t, f.db, "otheradmin")

	form := revokeForm("", "")
	form.Del("csrf_token")
	rec := submit(newRouter(f, adminID), rolePath(targetID, model.RoleNameAdmin), form, false)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if !holdsAdmin(t, f.db, targetID) {
		t.Error("CSRF トークンの無い送信で admin ロールが外れている")
	}
}
