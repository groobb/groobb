package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/handler/admin"
	"github.com/groobb/groobb/go/internal/handler/admin_user"
	"github.com/groobb/groobb/go/internal/handler/admin_user_role"
	"github.com/groobb/groobb/go/internal/httperror"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/templates"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/viewmodel"
	"github.com/groobb/groobb/go/static"
)

const adminRouteCSRFToken = "test-csrf-token"

// newAdminRouteFixtureは、1つのテスト用データベースの上に本番の管理用ルートを
// 組み立てます。実際の登録関数を検証対象にするため、どのルートの認証配線を変更しても
// テストが通るルーターが変わります。
func newAdminRouteFixture(t *testing.T) (*database.DB, http.Handler) {
	t.Helper()

	db := testutil.SetupDB(t)
	cfg := &config.Config{Env: "test"}
	errorRenderer := httperror.NewRenderer(cfg)
	roleRepo := repository.NewRoleRepository(db)
	userRepo := repository.NewUserRepository(db)
	userRoleRepo := repository.NewUserRoleRepository(db)

	hub := admin.NewHandler(cfg, errorRenderer, usecase.NewGetAdminHomeUsecase(roleRepo))
	users := admin_user.NewHandler(cfg, errorRenderer, usecase.NewGetAdminUsersUsecase(roleRepo, userRepo))
	userRoles := admin_user_role.NewHandler(
		errorRenderer,
		session.NewFlashManager(cfg),
		usecase.NewGrantUserRoleUsecase(db.Writer, roleRepo, userRepo, userRoleRepo),
		usecase.NewRevokeUserRoleUsecase(db.Writer, roleRepo, userRepo, userRoleRepo),
	)
	auth := middleware.NewAuth(session.NewManager(userRepo, cfg))

	router := chi.NewRouter()
	router.Use(middleware.PostFormLimit)
	router.Use(middleware.NewCSRF(cfg).Middleware)
	router.Use(middleware.MethodOverride)
	registerAdminRoutes(router, auth, hub, users, userRoles)

	return db, router
}

// getAdminRouteは、匿名の訪問者が管理画面へのリンクを辿って送るリクエストを
// 組み立てます。
func getAdminRoute(path string) *http.Request {
	return httptest.NewRequest(http.MethodGet, path, nil)
}

// submitAdminRouteは、一覧のボタンが送る送信を組み立てます。すなわちurlencodedの
// ボディを運ぶPOSTで、CSRF Cookieとそれに一致するトークンを添えます。一致するトークンに
// よりCSRFの検証が先に応答することを避け、ルートの認証ガードをテストから観測できるように
// します。チェーンのメソッドオーバーライドは、フォームがそう述べていればこのPOSTをDELETE
// へ変えます。剥奪のルートへ到達する手立てはそれだけです。
func submitAdminRoute(path string, form url.Values) *http.Request {
	form.Set("csrf_token", adminRouteCSRFToken)
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: middleware.CSRFCookieName, Value: adminRouteCSRFToken})
	return req
}

// TestRegisterAdminRoutes_RequireAuthは、サーバーが登録する管理用のどのルートも、
// 匿名のリクエストをハンドラーへ到達させず、サインインへ追い返すことを検証します。ページは
// 拒否されたアドレスを載せて送られ、セッションが発行されればそこへ辿り着きます。2つの書き込みは
// 送信を着地ページとして再現できないため、素のサインインへ送られます。
func TestRegisterAdminRoutes_RequireAuth(t *testing.T) {
	t.Parallel()

	db, router := newAdminRouteFixture(t)
	plainID := testutil.NewUserBuilder(t, db).WithAtname("plainuser").Build()
	holderID := testutil.NewUserBuilder(t, db).WithAtname("roleholder").Build()
	testutil.NewUserRoleBuilder(t, db).WithUserID(holderID).Build()

	tests := []struct {
		name         string
		req          *http.Request
		wantLocation string
	}{
		{
			name:         "管理ハブ",
			req:          getAdminRoute(templates.AdminPath().String()),
			wantLocation: templates.SignInPath().WithReturnTo(templates.AdminPath().String()).String(),
		},
		{
			name:         "ユーザー一覧",
			req:          getAdminRoute(templates.AdminUsersPath().String()),
			wantLocation: templates.SignInPath().WithReturnTo(templates.AdminUsersPath().String()).String(),
		},
		{
			name: "ロールの付与",
			req: submitAdminRoute(
				templates.AdminUserRolesPath(viewmodel.UserID(plainID)).String(),
				url.Values{"role_name": {string(model.RoleNameAdmin)}},
			),
			wantLocation: templates.SignInPath().String(),
		},
		{
			name: "ロールの剥奪",
			req: submitAdminRoute(
				templates.AdminUserRolePath(viewmodel.UserID(holderID), string(model.RoleNameAdmin)).String(),
				url.Values{"_method": {http.MethodDelete}},
			),
			wantLocation: templates.SignInPath().String(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, tt.req)

			if rec.Code != http.StatusSeeOther {
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusSeeOther)
			}
			if got := rec.Header().Get("Location"); got != tt.wantLocation {
				t.Errorf("Location = %q、期待値 = %q", got, tt.wantLocation)
			}
		})
	}

	// 2つの書き込みが名指した割当を、リクエストの後に読み戻します。番人が通して
	// しまった場合に、応答だけでなく動いたロールからも捉えられるようにするためです。
	assertHoldsAdmin(t, db, plainID, false)
	assertHoldsAdmin(t, db, holderID, true)
}

// newSlashRouterは、サーバーのルーターのうち末尾スラッシュの正規化が出会う部分を
// 組み立てます。すなわちミドルウェア本体、GETで到達するルート、POSTで到達するルート、
// トップページ (正規のパスがスラッシュで終わる唯一のルート)、そして埋め込みアセット
// (自身でリダイレクトを発行する唯一のハンドラーであるファイルサーバー) です。ルートは
// 実物の代役であり、これによって本テストはハンドラーの挙動ではなく正規化を固定します。
func newSlashRouter() *chi.Mux {
	r := chi.NewRouter()
	r.Use(chimiddleware.RedirectSlashes)

	ok := func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }
	r.Get("/", ok)
	r.Get("/settings/email/edit", ok)
	r.Post("/sign_up", ok)
	r.Handle("/static/*", http.StripPrefix("/static", http.FileServer(http.FS(static.Assets()))))

	return r
}

// TestRedirectSlashesは、末尾スラッシュ付きのURLがスラッシュ無しの同じURLへの
// 恒久リダイレクトで応答されること、クエリ文字列がその1ホップを越えて残ること、そして
// 正規のパスがスラッシュそのものであるトップページが対象外であることを検証します。
//
// 2つのアドレスのどちらが正規かを検索エンジンに伝えるのは恒久リダイレクトであるため、
// ステータスを遷移先と併せて検証します。POSTのケースはメソッドが保たれないことを記録
// するものです。301ではクライアントがGETへ落とすことが許されており、フォームの
// 送信先がスラッシュで終わるパスになったときにそれが見えるようにしています。
func TestRedirectSlashes(t *testing.T) {
	t.Parallel()

	router := newSlashRouter()

	tests := []struct {
		name         string
		method       string
		target       string
		wantStatus   int
		wantLocation string
	}{
		{
			name:         "ページは末尾スラッシュを除いたパスへリダイレクトされる",
			method:       http.MethodGet,
			target:       "/settings/email/edit/",
			wantStatus:   http.StatusMovedPermanently,
			wantLocation: "/settings/email/edit",
		},
		{
			name:         "クエリ文字列がリダイレクトを越えて残る",
			method:       http.MethodGet,
			target:       "/settings/email/edit/?return_to=%2Fhome",
			wantStatus:   http.StatusMovedPermanently,
			wantLocation: "/settings/email/edit?return_to=%2Fhome",
		},
		{
			name:         "連続した末尾スラッシュが1回のリダイレクトで正規化される",
			method:       http.MethodGet,
			target:       "/settings/email/edit//",
			wantStatus:   http.StatusMovedPermanently,
			wantLocation: "/settings/email/edit",
		},
		{
			name:         "POSTもリダイレクトされ、クライアントでメソッドが失われる",
			method:       http.MethodPost,
			target:       "/sign_up/",
			wantStatus:   http.StatusMovedPermanently,
			wantLocation: "/sign_up",
		},
		{
			name:       "トップページはそのまま配信される",
			method:     http.MethodGet,
			target:     "/",
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(tt.method, tt.target, nil))

			if rec.Code != tt.wantStatus {
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, tt.wantStatus)
			}
			if got := rec.Header().Get("Location"); got != tt.wantLocation {
				t.Errorf("Location = %q、期待値 = %q", got, tt.wantLocation)
			}
		})
	}
}

// TestRedirectSlashesDoesNotLoopOnAssetDirectoryは、静的アセットを収めた
// ディレクトリが、URLの2つの形の間を往復するのではなく404に落ち着くことを検証します。
//
// ファイルサーバーはディレクトリのパスに対し、末尾スラッシュを足すリダイレクトを返します。
// それは本ミドルウェアが剥がすリダイレクトそのものであり、2つを組み合わせると訪問者が
// 無限ループを踏むという既知の非互換になります。Groobbがこれを免れているのは
// static.Assetsがディレクトリを存在しないものとして扱うためで、それが成り立たなくなった
// ことに気付くのが本テストです。
func TestRedirectSlashesDoesNotLoopOnAssetDirectory(t *testing.T) {
	t.Parallel()

	router := newSlashRouter()

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/static/css/", nil))

	if rec.Code != http.StatusMovedPermanently {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusMovedPermanently)
	}
	location := rec.Header().Get("Location")
	if location != "/static/css" {
		t.Fatalf("Location = %q、期待値 = %q", location, "/static/css")
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, location, nil))

	if rec.Code != http.StatusNotFound {
		t.Errorf("%s でのステータスコード = %d、期待値 = %d", location, rec.Code, http.StatusNotFound)
	}
}
