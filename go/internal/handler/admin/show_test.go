package admin_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/handler/admin"
	"github.com/groobb/groobb/go/internal/httperror"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/templates"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
)

// newAdminHandler wires an admin Handler over the test database's repositories,
// so a handler test drives the permission check against roles that are really
// stored.
//
// [Ja] newAdminHandler はテスト用データベースのリポジトリで admin Handler を組み立てます。
// ハンドラーテストが、実際に保存されたロールに対して権限の判定を動かせるようにするためです。
func newAdminHandler(t *testing.T, db *database.DB) *admin.Handler {
	t.Helper()

	cfg := &config.Config{Env: "test"}
	return admin.NewHandler(
		cfg,
		httperror.NewRenderer(cfg),
		usecase.NewGetAdminHomeUsecase(repository.NewRoleRepository(db)),
	)
}

// getAdmin builds a GET /admin request carrying the given user in the context (as
// RequireAuth would place it) and the current path (as CurrentPathMiddleware
// would), then serves it with the handler.
//
// [Ja] getAdmin は、(RequireAuth が置くように) context に指定されたユーザーを、
// (CurrentPathMiddleware が置くように) 現在のパスを載せた GET /admin リクエストを組み立て、
// ハンドラーで応答します。
func getAdmin(t *testing.T, db *database.DB, userID model.UserID, locale model.Locale) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, templates.AdminPath().String(), nil)
	ctx := i18n.SetLocale(req.Context(), locale)
	ctx = templates.SetCurrentPath(ctx, templates.AdminPath().String())
	ctx = middleware.SetUserToContext(ctx, &model.User{ID: userID, Atname: "alice"})
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	newAdminHandler(t, db).Show(rec, req)
	return rec
}

// TestShow verifies that an administrator opening GET /admin is answered with
// HTTP 200 and an HTML body carrying the localized heading, the link on to the
// user list, the shared signed-in header, and the noindex robots meta, for each
// supported locale.
//
// [Ja] TestShow は、管理者が GET /admin を開くと HTTP 200 と、サポートする各ロケールに
// ついて、ローカライズされた見出し・利用者一覧へのリンク・サインイン済みページ共通の
// ヘッダー・noindex の robots メタを運ぶ HTML ボディで応答されることを検証します。
func TestShow(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		locale        model.Locale
		wantHeading   string
		wantUsersLink string
		wantHeaderNav string
	}{
		{name: "Japanese", locale: model.LocaleJa, wantHeading: "管理", wantUsersLink: "利用者の一覧", wantHeaderNav: "グローバルナビゲーション"},
		{name: "English", locale: model.LocaleEn, wantHeading: "Admin", wantUsersLink: "User list", wantHeaderNav: "Global navigation"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			db := testutil.SetupDB(t)
			userID := testutil.NewUserBuilder(t, db).Build()
			testutil.NewUserRoleBuilder(t, db).WithUserID(userID).Build()

			rec := getAdmin(t, db, userID, tt.locale)

			if rec.Code != http.StatusOK {
				t.Errorf("status code = %d, want %d", rec.Code, http.StatusOK)
			}
			if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
				t.Errorf("Content-Type = %q, want prefix %q", got, "text/html")
			}

			body := rec.Body.String()
			wants := []string{
				tt.wantHeading,
				tt.wantUsersLink,
				`href="/admin/users"`,
				`aria-label="` + tt.wantHeaderNav + `"`,
				`id="admin-show-heading"`,
				`aria-labelledby="admin-show-heading"`,
				`<meta name="robots" content="noindex"`,
				`lang="` + string(tt.locale) + `"`,
			}
			for _, want := range wants {
				if !strings.Contains(body, want) {
					t.Errorf("response body does not contain %q", want)
				}
			}
		})
	}
}

// TestShow_Forbidden verifies that a signed-in visitor holding no role is
// answered with the 403 page rather than the hub: being signed in is not by
// itself permission to open the administration screens.
//
// The refusal is checked to be noindex and to carry none of the hub, so that a
// page listing what the visitor may not open is neither shown to them nor
// recorded by a crawler.
//
// [Ja] TestShow_Forbidden は、ロールを 1 つも持たないサインイン済みの訪問者が、ハブでは
// なく 403 ページで応答されることを検証します。サインインしていること自体は、管理画面を
// 開いてよいという意味ではありません。
//
// 拒否が noindex であること、そしてハブの中身を一切運ばないことを確かめます。訪問者が
// 開いてはならないものを並べたページが、その人にも、クローラーにも渡らないようにするため
// です。
func TestShow_Forbidden(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	userID := testutil.NewUserBuilder(t, db).Build()

	rec := getAdmin(t, db, userID, model.LocaleJa)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if got := rec.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Errorf("Cache-Control = %q, want %q", got, "private, no-store")
	}

	body := rec.Body.String()
	if !strings.Contains(body, "権限がありません") {
		t.Error("403 ページの見出しが描画されていない")
	}
	if !strings.Contains(body, `<meta name="robots" content="noindex"`) {
		t.Error("403 ページに noindex が付いていない")
	}
	if strings.Contains(body, `href="/admin/users"`) {
		t.Error("権限の無い訪問者へ管理画面へのリンクが描画されている")
	}
}

// TestShow_WithoutUser verifies that reaching the handler without the user
// RequireAuth promises is answered with an internal server error instead of
// panicking or treating the visitor as merely unauthorized.
//
// [Ja] TestShow_WithoutUser は、RequireAuth が保証するユーザー無しでハンドラーへ到達した
// 場合に、panic や単なる権限不足ではなく Internal Server Error で応答することを検証します。
func TestShow_WithoutUser(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	req := httptest.NewRequest(http.MethodGet, templates.AdminPath().String(), nil)
	req = req.WithContext(i18n.SetLocale(req.Context(), model.LocaleJa))
	rec := httptest.NewRecorder()

	newAdminHandler(t, db).Show(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if got := rec.Body.String(); got != "Internal Server Error\n" {
		t.Errorf("response body = %q, want %q", got, "Internal Server Error\n")
	}
}

// TestShow_PermissionLookupFailure verifies that failure to resolve the actor's
// roles is answered with an internal server error rather than the 403 page. A
// database outage does not mean the signed-in visitor lacks permission.
//
// [Ja] TestShow_PermissionLookupFailure は、操作者のロール取得失敗が 403 ページではなく
// Internal Server Error で応答されることを検証します。データベース障害は、サインイン済みの
// 訪問者に権限が無いことを意味しません。
func TestShow_PermissionLookupFailure(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	userID := testutil.NewUserBuilder(t, db).Build()
	if err := db.Reader.Close(); err != nil {
		t.Fatalf("Reader の Close() error = %v", err)
	}

	rec := getAdmin(t, db, userID, model.LocaleJa)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if got := rec.Body.String(); got != "Internal Server Error\n" {
		t.Errorf("response body = %q, want %q", got, "Internal Server Error\n")
	}
}
