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

// rolePathは、アカウントが持つ1つのロールのアドレスで、剥奪の送信先です。
func rolePath(id model.UserID, roleName model.RoleName) string {
	return templates.AdminUserRolePath(viewmodel.UserID(id), string(roleName)).String()
}

// TestDelete_RevokesTheRoleは、ある管理者が別の管理者からロールを取り上げるとそれが
// 外れること、そのことが伝えられること、そしてボタンが押された一覧のページへ戻ることを
// 検証します。
func TestDelete_RevokesTheRole(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	adminID := buildAdministrator(t, f.db, "adminuser")
	targetID := buildAdministrator(t, f.db, "otheradmin")

	rec := submit(newRouter(f, adminID), rolePath(targetID, model.RoleNameAdmin), revokeForm("other", "3"), true)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusSeeOther)
	}
	want := templates.AdminUsersPagePath("other", 3).String()
	if got := rec.Header().Get("Location"); got != want {
		t.Errorf("Location = %q、期待値 = %q", got, want)
	}
	if flash := decodeFlash(t, rec); flash.Type != session.FlashSuccess || flash.Message != "@otheradminから管理者ロールを剥奪しました" {
		t.Errorf("フラッシュ = %+v、期待値は成功の「@otheradminから管理者ロールを剥奪しました」", flash)
	}
	if holdsAdmin(t, f.db, targetID) {
		t.Error("剥奪した相手がadminロールを持ったままになっている")
	}
}

// TestDelete_FromSelfは、他に管理者が残っている限り、管理者が自ら降りることが許され、
// 一覧ではなくホームへ送られることを検証します。一覧は真っ先に読めなくなるものであるためです。
func TestDelete_FromSelf(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	adminID := buildAdministrator(t, f.db, "adminuser")
	buildAdministrator(t, f.db, "otheradmin")

	rec := submit(newRouter(f, adminID), rolePath(adminID, model.RoleNameAdmin), revokeForm("", ""), true)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusSeeOther)
	}
	if got, want := rec.Header().Get("Location"), templates.HomePath().String(); got != want {
		t.Errorf("Location = %q、期待値 = %q", got, want)
	}
	if holdsAdmin(t, f.db, adminID) {
		t.Error("自分から外したadminロールが残っている")
	}
}

// TestDelete_LastAdministratorは、コミュニティが管理者のいない状態にならないことを
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
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusSeeOther)
	}
	want := templates.AdminUsersPagePath("admin", 1).String()
	if got := rec.Header().Get("Location"); got != want {
		t.Errorf("Location = %q、期待値 = %q", got, want)
	}
	flash := decodeFlash(t, rec)
	if flash.Type != session.FlashError {
		t.Errorf("フラッシュの種類 = %q、期待値 = %q", flash.Type, session.FlashError)
	}
	if flash.Message != "最後の管理者から管理者ロールを外すことはできません。先に別の利用者を管理者にしてください。" {
		t.Errorf("フラッシュのメッセージ = %q、期待値は最後の管理者の拒否の文言", flash.Message)
	}
	if !holdsAdmin(t, f.db, adminID) {
		t.Error("最後の管理者からadminロールが外れている")
	}
}

// TestDelete_WithoutPermissionは、ロールを取り上げてはならないアカウントが403
// ページで応答されること、そしてロールが保たれたままであることを検証します。
func TestDelete_WithoutPermission(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	actorID := testutil.NewUserBuilder(t, f.db).WithAtname("plainuser").Build()
	targetID := buildAdministrator(t, f.db, "adminuser")

	rec := submit(newRouter(f, actorID), rolePath(targetID, model.RoleNameAdmin), revokeForm("", ""), true)

	if rec.Code != http.StatusForbidden {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusForbidden)
	}
	if !holdsAdmin(t, f.db, targetID) {
		t.Error("権限の無い操作者の送信でadminロールが外れている")
	}
}

// TestDelete_NamesNothingは、どのアカウントもどのロールも名指していないアドレスが
// 404ページで応答されることを検証します。
func TestDelete_NamesNothing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path func(adminID model.UserID) string
	}{
		{
			name: "idが整数ではない",
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
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusNotFound)
			}
		})
	}
}

// TestDelete_WithoutCSRFTokenは、フォームが埋め込むトークンを伴わずに届いた剥奪が
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
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusForbidden)
	}
	if !holdsAdmin(t, f.db, targetID) {
		t.Error("CSRFトークンの無い送信でadminロールが外れている")
	}
}
