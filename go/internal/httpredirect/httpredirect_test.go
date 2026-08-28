package httpredirect_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/httpredirect"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/templates"
)

// TestToCanonicalKeepsACSRFCookieOutOfSharedCaches verifies the middleware
// combination used in production: a safe request without a CSRF cookie receives
// a newly minted token before the canonical redirect is written, and that
// response remains private so a shared cache cannot hand the token to another
// visitor. The browser still receives a one-hour lifetime for the permanent
// redirect.
//
// [Ja] TestToCanonicalKeepsACSRFCookieOutOfSharedCaches は、本番と同じミドルウェアの
// 組み合わせを検証します。CSRF Cookie を持たない安全なリクエストは正規 URL への
// リダイレクトが書かれる前に新しいトークンを受け取り、その応答は private のままなので、
// 共有キャッシュが別の訪問者へトークンを渡せません。一方、ブラウザには恒久
// リダイレクトの 1 時間の有効期間が伝わります。
func TestToCanonicalKeepsACSRFCookieOutOfSharedCaches(t *testing.T) {
	t.Parallel()

	csrf := middleware.NewCSRF(&config.Config{Env: "test"})
	handler := middleware.HTMLCache(csrf.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpredirect.ToCanonical(w, r, templates.BoardPath("chat"))
	})))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/b/CHAT", nil)

	handler.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusPermanentRedirect; got != want {
		t.Errorf("status code = %d, want %d", got, want)
	}
	if got, want := rec.Header().Get("Location"), templates.BoardPath("chat").String(); got != want {
		t.Errorf("Location = %q, want %q", got, want)
	}
	if got, want := rec.Header().Get("Cache-Control"), "private, max-age=3600"; got != want {
		t.Errorf("Cache-Control = %q, want %q", got, want)
	}

	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == middleware.CSRFCookieName && cookie.Value != "" {
			return
		}
	}
	t.Error("CSRF middleware did not attach a newly minted token to the redirect")
}
