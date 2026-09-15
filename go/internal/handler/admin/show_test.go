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

// newAdminHandlerはテスト用データベースのリポジトリでadmin Handlerを組み立てます。
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

// getAdminは、(RequireAuthが置くように) contextに指定されたユーザーを、
// (CurrentPathMiddlewareが置くように) 現在のパスを載せたGET /adminリクエストを組み立て、
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

// TestShowは、管理者がGET /adminを開くとHTTP 200と、サポートする各ロケールに
// ついて、ローカライズされた見出し・利用者一覧へのリンク・サインイン済みページ共通の
// ヘッダー・noindexのrobotsメタを運ぶHTMLボディで応答されることを検証します。
func TestShow(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		locale        model.Locale
		wantHeading   string
		wantUsersLink string
		wantHeaderNav string
	}{
		{name: "日本語", locale: model.LocaleJa, wantHeading: "管理", wantUsersLink: "利用者の一覧", wantHeaderNav: "グローバルナビゲーション"},
		{name: "英語", locale: model.LocaleEn, wantHeading: "Admin", wantUsersLink: "User list", wantHeaderNav: "Global navigation"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			db := testutil.SetupDB(t)
			userID := testutil.NewUserBuilder(t, db).Build()
			testutil.NewUserRoleBuilder(t, db).WithUserID(userID).Build()

			rec := getAdmin(t, db, userID, tt.locale)

			if rec.Code != http.StatusOK {
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
			}
			if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
				t.Errorf("Content-Type = %q、期待値の接頭辞 = %q", got, "text/html")
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
					t.Errorf("レスポンスボディに %q が含まれていない", want)
				}
			}
		})
	}
}

// TestShow_Forbiddenは、ロールを1つも持たないサインイン済みの訪問者が、ハブでは
// なく403ページで応答されることを検証します。サインインしていること自体は、管理画面を
// 開いてよいという意味ではありません。
//
// 拒否がnoindexであること、そしてハブの中身を一切運ばないことを確かめます。訪問者が
// 開いてはならないものを並べたページが、その人にも、クローラーにも渡らないようにするため
// です。
func TestShow_Forbidden(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	userID := testutil.NewUserBuilder(t, db).Build()

	rec := getAdmin(t, db, userID, model.LocaleJa)

	if rec.Code != http.StatusForbidden {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusForbidden)
	}
	if got := rec.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Errorf("Cache-Control = %q、期待値 = %q", got, "private, no-store")
	}

	body := rec.Body.String()
	if !strings.Contains(body, "権限がありません") {
		t.Error("403ページの見出しが描画されていない")
	}
	if !strings.Contains(body, `<meta name="robots" content="noindex"`) {
		t.Error("403ページにnoindexが付いていない")
	}
	if strings.Contains(body, `href="/admin/users"`) {
		t.Error("権限の無い訪問者へ管理画面へのリンクが描画されている")
	}
}

// TestShow_WithoutUserは、RequireAuthが保証するユーザー無しでハンドラーへ到達した
// 場合に、panicや単なる権限不足ではなくInternal Server Errorで応答することを検証します。
func TestShow_WithoutUser(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	req := httptest.NewRequest(http.MethodGet, templates.AdminPath().String(), nil)
	req = req.WithContext(i18n.SetLocale(req.Context(), model.LocaleJa))
	rec := httptest.NewRecorder()

	newAdminHandler(t, db).Show(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusInternalServerError)
	}
	if got := rec.Body.String(); got != "Internal Server Error\n" {
		t.Errorf("レスポンスボディ = %q、期待値 = %q", got, "Internal Server Error\n")
	}
}

// TestShow_PermissionLookupFailureは、操作者のロール取得失敗が403ページではなく
// Internal Server Errorで応答されることを検証します。データベース障害は、サインイン済みの
// 訪問者に権限が無いことを意味しません。
func TestShow_PermissionLookupFailure(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	userID := testutil.NewUserBuilder(t, db).Build()
	if err := db.Reader.Close(); err != nil {
		t.Fatalf("ReaderのClose()のエラー = %v", err)
	}

	rec := getAdmin(t, db, userID, model.LocaleJa)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusInternalServerError)
	}
	if got := rec.Body.String(); got != "Internal Server Error\n" {
		t.Errorf("レスポンスボディ = %q、期待値 = %q", got, "Internal Server Error\n")
	}
}
