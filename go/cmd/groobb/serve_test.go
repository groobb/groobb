package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/groobb/groobb/go/static"
)

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
