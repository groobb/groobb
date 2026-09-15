package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/httperror"
)

// TestHTMLCacheは通常HTMLのprivateな既定値と、アセット・404に対して既に
// 選ばれている、より具体的な方針、そしてHTMLではないレスポンスにHTMLの方針が
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
			name: "通常のHTMLにはprivateの既定値が付く",
			handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.WriteHeader(http.StatusOK)
			}),
			wantCacheControl: "private, no-cache",
		},
		{
			name: "静的アセットはアセットの方針を保つ",
			handler: AssetCache(&config.Config{Env: "prod"})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/css; charset=utf-8")
				w.WriteHeader(http.StatusOK)
			})),
			wantCacheControl: "private, max-age=31536000, immutable",
		},
		{
			name:             "404ページはno-storeの方針を保つ",
			handler:          http.HandlerFunc(errorRenderer.NotFound),
			wantCacheControl: "private, no-store",
		},
		{
			name: "HTML以外にはHTMLの既定値が付かない",
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
				t.Errorf("Cache-Control = %q、期待値 = %q", got, tt.wantCacheControl)
			}
		})
	}
}

// TestHTMLCacheKeepsCSRFPersonalizedHTMLPrivateは新しく発行した、または再利用した
// 訪問者固有のCSRFトークンを含むHTMLが共有キャッシュに保存されないことを検証します。
func TestHTMLCacheKeepsCSRFPersonalizedHTMLPrivate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		requestToken string
	}{
		{name: "新しいトークンを発行して埋め込む"},
		{name: "リクエストのトークンを再利用して埋め込む", requestToken: "existing-token"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			csrf := NewCSRF(&config.Config{Env: "test"})
			handler := HTMLCache(csrf.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				if _, err := w.Write([]byte(CSRFTokenFromContext(r.Context()))); err != nil {
					t.Errorf("レスポンスボディの書き込みに失敗: %v", err)
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
					t.Fatal("CSRF Cookieが設定されていない")
				}
			}

			if got := rec.Body.String(); got != wantToken {
				t.Errorf("レスポンスボディ = %q、期待値は埋め込まれたCSRFトークン %q", got, wantToken)
			}
			if got := resp.Header.Get("Cache-Control"); got != "private, no-cache" {
				t.Errorf("Cache-Control = %q、期待値 = %q", got, "private, no-cache")
			}
		})
	}
}

// TestHTMLCacheDetectsImplicitHTMLはGoの内容判定に委ねるハンドラーでも、暗黙の
// 200を送る前にHTMLの既定値が付くことを検証します。
func TestHTMLCacheDetectsImplicitHTML(t *testing.T) {
	t.Parallel()

	handler := HTMLCache(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write([]byte("<!doctype html><title>Groobb</title>")); err != nil {
			t.Errorf("レスポンスボディの書き込みに失敗: %v", err)
		}
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if got := rec.Header().Get("Cache-Control"); got != "private, no-cache" {
		t.Errorf("Cache-Control = %q、期待値 = %q", got, "private, no-cache")
	}
}

// TestHTMLCacheAppliesDefaultToTrailingSlashRedirectは、サイト全体のキャッシュ
// ミドルウェアが末尾スラッシュの正規化を包み、そのHTML 301の送出前に既定値を
// 付けることを検証します。
func TestHTMLCacheAppliesDefaultToTrailingSlashRedirect(t *testing.T) {
	t.Parallel()

	handler := HTMLCache(chimiddleware.RedirectSlashes(http.NotFoundHandler()))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/settings/", nil))

	if rec.Code != http.StatusMovedPermanently {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusMovedPermanently)
	}
	if got := rec.Header().Get("Location"); got != "/settings" {
		t.Errorf("Location = %q、期待値 = %q", got, "/settings")
	}
	if got := rec.Header().Get("Cache-Control"); got != "private, no-cache" {
		t.Errorf("Cache-Control = %q、期待値 = %q", got, "private, no-cache")
	}
}

// TestHTMLCacheFlushesWithTheDefaultはサイト全体のラッパーが、Flushによる暗黙の
// 200の送出前にHTMLの既定値を適用しつつ、サーバーのwriterの追加インターフェースへ
// 到達できることを検証します。
func TestHTMLCacheFlushesWithTheDefault(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	handler := HTMLCache(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := http.NewResponseController(w).Flush(); err != nil {
			t.Errorf("レスポンスのフラッシュに失敗: %v", err)
		}
	}))
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if !rec.Flushed {
		t.Error("ハンドラーが下層のwriterをフラッシュしていない")
	}
	if got := rec.Result().Header.Get("Cache-Control"); got != "private, no-cache" {
		t.Errorf("Cache-Control = %q、期待値 = %q", got, "private, no-cache")
	}
}

// TestHTMLCacheLeavesContentTypeToTheServerは、ラッパーがHTMLを認識するためだけに
// 本文を読み、net/httpが送らないContent-Typeを足さないことを検証します。net/httpは
// エンコード済みの本文と空の本文について型を判定しません。実際のサーバーに応答させるのは、
// httptest.ResponseRecorderが独自の判定規則を持ち、ラッパーの寄与を覆い隠すためです。
func TestHTMLCacheLeavesContentTypeToTheServer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		contentEncoding string
		body            []byte
	}{
		{
			name:            "エンコード済みのボディは型を判定しない",
			contentEncoding: "gzip",
			body:            []byte("\x1f\x8b\x08 compressed HTML"),
		},
		{
			name: "空のボディは型を判定しない",
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
					t.Errorf("レスポンスボディの書き込みに失敗: %v", err)
				}
			})))
			defer server.Close()

			resp, err := server.Client().Get(server.URL)
			if err != nil {
				t.Fatalf("テストサーバーへのリクエストに失敗: %v", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if got := resp.Header.Get("Content-Type"); got != "" {
				t.Errorf("Content-Type = %q、期待値は未設定 (net/httpの判定に任せる)", got)
			}
			if got := resp.Header.Get("Cache-Control"); got != "" {
				t.Errorf("Cache-Control = %q、期待値はHTMLの既定値なし", got)
			}
		})
	}
}
