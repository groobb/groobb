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

// TestToCanonicalKeepsACSRFCookieOutOfSharedCachesは、本番と同じミドルウェアの
// 組み合わせを検証します。CSRF Cookieを持たない安全なリクエストは正規URLへの
// リダイレクトが書かれる前に新しいトークンを受け取り、その応答はprivateのままなので、
// 共有キャッシュが別の訪問者へトークンを渡せません。一方、ブラウザには恒久
// リダイレクトの1時間の有効期間が伝わります。
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
		t.Errorf("ステータスコード = %d、期待値 = %d", got, want)
	}
	if got, want := rec.Header().Get("Location"), templates.BoardPath("chat").String(); got != want {
		t.Errorf("Location = %q、期待値 = %q", got, want)
	}
	if got, want := rec.Header().Get("Cache-Control"), "private, max-age=3600"; got != want {
		t.Errorf("Cache-Control = %q、期待値 = %q", got, want)
	}

	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == middleware.CSRFCookieName && cookie.Value != "" {
			return
		}
	}
	t.Error("CSRFミドルウェアがリダイレクトに新しく発行したトークンを付けていない")
}
