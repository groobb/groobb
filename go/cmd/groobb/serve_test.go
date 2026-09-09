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

// newAdminRouteFixture builds the production administration routes over one test
// database. The actual registration function is the subject of these tests, so
// changing the authentication wiring of any route changes the router they
// exercise.
//
// [Ja] newAdminRouteFixture は、1 つのテスト用データベースの上に本番の管理用ルートを
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

// getAdminRoute builds the request an anonymous visitor makes by following a
// link to an administration page.
//
// [Ja] getAdminRoute は、匿名の訪問者が管理画面へのリンクを辿って送るリクエストを
// 組み立てます。
func getAdminRoute(path string) *http.Request {
	return httptest.NewRequest(http.MethodGet, path, nil)
}

// submitAdminRoute builds the submission a listing's button sends: a POST
// carrying the urlencoded body, with a CSRF cookie and a token that matches it.
// The matching token keeps the CSRF check from answering first, so the route's
// authentication guard is what the tests observe. The method override in the
// chain turns the POST into the DELETE when the form says so, which is the only
// way the revoke route is reached.
//
// [Ja] submitAdminRoute は、一覧のボタンが送る送信を組み立てます。すなわち urlencoded の
// ボディを運ぶ POST で、CSRF Cookie とそれに一致するトークンを添えます。一致するトークンに
// より CSRF の検証が先に応答することを避け、ルートの認証ガードをテストから観測できるように
// します。チェーンのメソッドオーバーライドは、フォームがそう述べていればこの POST を DELETE
// へ変えます。剥奪のルートへ到達する手立てはそれだけです。
func submitAdminRoute(path string, form url.Values) *http.Request {
	form.Set("csrf_token", adminRouteCSRFToken)
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: middleware.CSRFCookieName, Value: adminRouteCSRFToken})
	return req
}

// TestRegisterAdminRoutes_RequireAuth verifies that every administration route
// the server registers turns an anonymous request away to sign-in instead of
// letting it reach the handler. A page carries the address it was refused at, so
// the visitor arrives there once their session is issued, while the two writes
// fall back to bare sign-in because a submission cannot be replayed as a landing
// page.
//
// [Ja] TestRegisterAdminRoutes_RequireAuth は、サーバーが登録する管理用のどのルートも、
// 匿名のリクエストをハンドラーへ到達させず、サインインへ追い返すことを検証します。ページは
// 拒否されたアドレスを載せて送られ、セッションが発行されればそこへ辿り着きます。2 つの書き込みは
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
			name:         "the hub",
			req:          getAdminRoute(templates.AdminPath().String()),
			wantLocation: templates.SignInPath().WithReturnTo(templates.AdminPath().String()).String(),
		},
		{
			name:         "the user list",
			req:          getAdminRoute(templates.AdminUsersPath().String()),
			wantLocation: templates.SignInPath().WithReturnTo(templates.AdminUsersPath().String()).String(),
		},
		{
			name: "the grant",
			req: submitAdminRoute(
				templates.AdminUserRolesPath(viewmodel.UserID(plainID)).String(),
				url.Values{"role_name": {string(model.RoleNameAdmin)}},
			),
			wantLocation: templates.SignInPath().String(),
		},
		{
			name: "the revoke",
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
				t.Errorf("status code = %d, want %d", rec.Code, http.StatusSeeOther)
			}
			if got := rec.Header().Get("Location"); got != tt.wantLocation {
				t.Errorf("Location = %q, want %q", got, tt.wantLocation)
			}
		})
	}

	// The assignments the two writes named are read back after the requests, so
	// that a guard which lets one through is caught by the role it moved as well
	// as by the response it produced.
	//
	// [Ja] 2 つの書き込みが名指した割当を、リクエストの後に読み戻します。番人が通して
	// しまった場合に、応答だけでなく動いたロールからも捉えられるようにするためです。
	assertHoldsAdmin(t, db, plainID, false)
	assertHoldsAdmin(t, db, holderID, true)
}

// newSlashRouter builds the parts of the server's router that trailing-slash
// normalization meets: the middleware itself, a route reached with GET, a route
// reached with POST, the top page (the one route whose canonical path ends in a
// slash), and the embedded assets, whose file server is the only handler that
// redirects on its own. The routes stand in for the real ones so that this test
// pins the normalization rather than any handler's behaviour.
//
// [Ja] newSlashRouter は、サーバーのルーターのうち末尾スラッシュの正規化が出会う部分を
// 組み立てます。すなわちミドルウェア本体、GET で到達するルート、POST で到達するルート、
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

// TestRedirectSlashes verifies that a URL carrying a trailing slash is answered
// with a permanent redirect to the same URL without one, that the query string
// survives the hop, and that the top page — whose canonical path is the slash
// itself — is left alone.
//
// A permanent redirect is what tells a search engine which of the two addresses
// is the canonical one, so the status is asserted alongside the location.
// The POST case records that the method does not survive: 301 lets a client fall
// back to GET, and it is here to make that visible if a form is ever pointed at
// a path ending in a slash.
//
// [Ja] TestRedirectSlashes は、末尾スラッシュ付きの URL がスラッシュ無しの同じ URL への
// 恒久リダイレクトで応答されること、クエリ文字列がその 1 ホップを越えて残ること、そして
// 正規のパスがスラッシュそのものであるトップページが対象外であることを検証します。
//
// 2 つのアドレスのどちらが正規かを検索エンジンに伝えるのは恒久リダイレクトであるため、
// ステータスを遷移先と併せて検証します。POST のケースはメソッドが保たれないことを記録
// するものです。301 ではクライアントが GET へ落とすことが許されており、フォームの
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
			name:         "a page redirects to the path without the trailing slash",
			method:       http.MethodGet,
			target:       "/settings/email/edit/",
			wantStatus:   http.StatusMovedPermanently,
			wantLocation: "/settings/email/edit",
		},
		{
			name:         "the query string survives the redirect",
			method:       http.MethodGet,
			target:       "/settings/email/edit/?return_to=%2Fhome",
			wantStatus:   http.StatusMovedPermanently,
			wantLocation: "/settings/email/edit?return_to=%2Fhome",
		},
		{
			name:         "repeated trailing slashes are normalized in a single hop",
			method:       http.MethodGet,
			target:       "/settings/email/edit//",
			wantStatus:   http.StatusMovedPermanently,
			wantLocation: "/settings/email/edit",
		},
		{
			name:         "a POST is redirected too, which drops the method at the client",
			method:       http.MethodPost,
			target:       "/sign_up/",
			wantStatus:   http.StatusMovedPermanently,
			wantLocation: "/sign_up",
		},
		{
			name:       "the top page is served as it is",
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
				t.Errorf("status code = %d, want %d", rec.Code, tt.wantStatus)
			}
			if got := rec.Header().Get("Location"); got != tt.wantLocation {
				t.Errorf("Location = %q, want %q", got, tt.wantLocation)
			}
		})
	}
}

// TestRedirectSlashesDoesNotLoopOnAssetDirectory verifies that the directory a
// static asset sits in comes to rest at a 404 rather than bouncing between the
// two forms of its URL.
//
// A file server hands a directory path back with a redirect that appends a
// trailing slash, which is the redirect this middleware strips; the two together
// are a documented incompatibility that costs a visitor an endless loop. Groobb
// escapes it because static.Assets reports its directories as missing, and this
// test is what notices if that ever stops being true.
//
// [Ja] TestRedirectSlashesDoesNotLoopOnAssetDirectory は、静的アセットを収めた
// ディレクトリが、URL の 2 つの形の間を往復するのではなく 404 に落ち着くことを検証します。
//
// ファイルサーバーはディレクトリのパスに対し、末尾スラッシュを足すリダイレクトを返します。
// それは本ミドルウェアが剥がすリダイレクトそのものであり、2 つを組み合わせると訪問者が
// 無限ループを踏むという既知の非互換になります。Groobb がこれを免れているのは
// static.Assets がディレクトリを存在しないものとして扱うためで、それが成り立たなくなった
// ことに気付くのが本テストです。
func TestRedirectSlashesDoesNotLoopOnAssetDirectory(t *testing.T) {
	t.Parallel()

	router := newSlashRouter()

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/static/css/", nil))

	if rec.Code != http.StatusMovedPermanently {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusMovedPermanently)
	}
	location := rec.Header().Get("Location")
	if location != "/static/css" {
		t.Fatalf("Location = %q, want %q", location, "/static/css")
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, location, nil))

	if rec.Code != http.StatusNotFound {
		t.Errorf("status code at %s = %d, want %d", location, rec.Code, http.StatusNotFound)
	}
}
