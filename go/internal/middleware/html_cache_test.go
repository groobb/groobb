package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/httperror"
)

// TestHTMLCache verifies the private default for ordinary HTML, the more
// specific policies already selected for assets and 404 responses, and that a
// response which is not HTML is left without an HTML policy.
//
// [Ja] TestHTMLCache は通常 HTML の private な既定値と、アセット・404 に対して既に
// 選ばれている、より具体的な方針、そして HTML ではないレスポンスに HTML の方針が
// 付かないことを検証します。
func TestHTMLCache(t *testing.T) {
	t.Parallel()

	errorRenderer := httperror.NewRenderer(&config.Config{Env: "dev"})
	tests := []struct {
		name             string
		handler          http.Handler
		wantCacheControl string
	}{
		{
			name: "ordinary HTML gets the private default",
			handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.WriteHeader(http.StatusOK)
			}),
			wantCacheControl: "private, no-cache",
		},
		{
			name: "a static asset keeps its asset policy",
			handler: AssetCache(&config.Config{Env: "prod"})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/css; charset=utf-8")
				w.WriteHeader(http.StatusOK)
			})),
			wantCacheControl: "private, max-age=31536000, immutable",
		},
		{
			name:             "the 404 page keeps its no-store policy",
			handler:          http.HandlerFunc(errorRenderer.NotFound),
			wantCacheControl: "private, no-store",
		},
		{
			name: "non-HTML gets no HTML default",
			handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				w.WriteHeader(http.StatusOK)
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			HTMLCache(tt.handler).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

			if got := rec.Header().Get("Cache-Control"); got != tt.wantCacheControl {
				t.Errorf("Cache-Control = %q, want %q", got, tt.wantCacheControl)
			}
		})
	}
}

// TestHTMLCacheKeepsCSRFPersonalizedHTMLPrivate verifies that HTML containing a
// newly issued or reused visitor-specific CSRF token cannot enter a shared cache.
//
// [Ja] TestHTMLCacheKeepsCSRFPersonalizedHTMLPrivate は新しく発行した、または再利用した
// 訪問者固有の CSRF トークンを含む HTML が共有キャッシュに保存されないことを検証します。
func TestHTMLCacheKeepsCSRFPersonalizedHTMLPrivate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		requestToken string
	}{
		{name: "a new token is set and embedded"},
		{name: "the request token is reused and embedded", requestToken: "existing-token"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			csrf := NewCSRF(&config.Config{Env: "test"})
			handler := HTMLCache(csrf.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				if _, err := w.Write([]byte(CSRFTokenFromContext(r.Context()))); err != nil {
					t.Errorf("failed to write response: %v", err)
				}
			})))

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.requestToken != "" {
				req.AddCookie(&http.Cookie{Name: CSRFCookieName, Value: tt.requestToken})
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			resp := rec.Result()
			wantToken := tt.requestToken
			if wantToken == "" {
				for _, cookie := range resp.Cookies() {
					if cookie.Name == CSRFCookieName {
						wantToken = cookie.Value
						break
					}
				}
				if wantToken == "" {
					t.Fatal("CSRF cookie was not set")
				}
			}

			if got := rec.Body.String(); got != wantToken {
				t.Errorf("response body = %q, want embedded CSRF token %q", got, wantToken)
			}
			if got := resp.Header.Get("Cache-Control"); got != "private, no-cache" {
				t.Errorf("Cache-Control = %q, want %q", got, "private, no-cache")
			}
		})
	}
}

// TestHTMLCacheDetectsImplicitHTML verifies that a handler relying on Go's
// content sniffing still receives the HTML default before its implicit 200.
//
// [Ja] TestHTMLCacheDetectsImplicitHTML は Go の内容判定に委ねるハンドラーでも、暗黙の
// 200 を送る前に HTML の既定値が付くことを検証します。
func TestHTMLCacheDetectsImplicitHTML(t *testing.T) {
	t.Parallel()

	handler := HTMLCache(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write([]byte("<!doctype html><title>Groobb</title>")); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if got := rec.Header().Get("Cache-Control"); got != "private, no-cache" {
		t.Errorf("Cache-Control = %q, want %q", got, "private, no-cache")
	}
}

// TestHTMLCacheAppliesDefaultToTrailingSlashRedirect verifies that the site-wide
// cache middleware wraps the trailing-slash normalization and marks its HTML 301
// before the redirect is sent.
//
// [Ja] TestHTMLCacheAppliesDefaultToTrailingSlashRedirect は、サイト全体のキャッシュ
// ミドルウェアが末尾スラッシュの正規化を包み、その HTML 301 の送出前に既定値を
// 付けることを検証します。
func TestHTMLCacheAppliesDefaultToTrailingSlashRedirect(t *testing.T) {
	t.Parallel()

	handler := HTMLCache(chimiddleware.RedirectSlashes(http.NotFoundHandler()))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/settings/", nil))

	if rec.Code != http.StatusMovedPermanently {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusMovedPermanently)
	}
	if got := rec.Header().Get("Location"); got != "/settings" {
		t.Errorf("Location = %q, want %q", got, "/settings")
	}
	if got := rec.Header().Get("Cache-Control"); got != "private, no-cache" {
		t.Errorf("Cache-Control = %q, want %q", got, "private, no-cache")
	}
}

// TestHTMLCacheFlushesWithTheDefault verifies that the site-wide wrapper applies
// the HTML default before Flush sends its implicit 200, while retaining access
// to the server writer's optional interface.
//
// [Ja] TestHTMLCacheFlushesWithTheDefault はサイト全体のラッパーが、Flush による暗黙の
// 200 の送出前に HTML の既定値を適用しつつ、サーバーの writer の追加インターフェースへ
// 到達できることを検証します。
func TestHTMLCacheFlushesWithTheDefault(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	handler := HTMLCache(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := http.NewResponseController(w).Flush(); err != nil {
			t.Errorf("failed to flush the response: %v", err)
		}
	}))
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if !rec.Flushed {
		t.Error("the handler did not flush the writer underneath")
	}
	if got := rec.Result().Header.Get("Cache-Control"); got != "private, no-cache" {
		t.Errorf("Cache-Control = %q, want %q", got, "private, no-cache")
	}
}

// TestHTMLCacheLeavesContentTypeToTheServer verifies that the wrapper reads the
// body only to recognize HTML, without adding a Content-Type net/http would not
// send: it does not detect the type of an encoded or an empty body. A real
// server answers the request because httptest.ResponseRecorder runs detection
// rules of its own, which would hide what the wrapper contributes.
//
// [Ja] TestHTMLCacheLeavesContentTypeToTheServer は、ラッパーが HTML を認識するためだけに
// 本文を読み、net/http が送らない Content-Type を足さないことを検証します。net/http は
// エンコード済みの本文と空の本文について型を判定しません。実際のサーバーに応答させるのは、
// httptest.ResponseRecorder が独自の判定規則を持ち、ラッパーの寄与を覆い隠すためです。
func TestHTMLCacheLeavesContentTypeToTheServer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		contentEncoding string
		body            []byte
	}{
		{
			name:            "an encoded body is left undetected",
			contentEncoding: "gzip",
			body:            []byte("\x1f\x8b\x08 compressed HTML"),
		},
		{
			name: "an empty body is left undetected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(HTMLCache(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if tt.contentEncoding != "" {
					w.Header().Set("Content-Encoding", tt.contentEncoding)
				}
				if _, err := w.Write(tt.body); err != nil {
					t.Errorf("failed to write response: %v", err)
				}
			})))
			defer server.Close()

			resp, err := server.Client().Get(server.URL)
			if err != nil {
				t.Fatalf("failed to request the test server: %v", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if got := resp.Header.Get("Content-Type"); got != "" {
				t.Errorf("Content-Type = %q, want it left for net/http to decide", got)
			}
			if got := resp.Header.Get("Cache-Control"); got != "" {
				t.Errorf("Cache-Control = %q, want no HTML default", got)
			}
		})
	}
}
