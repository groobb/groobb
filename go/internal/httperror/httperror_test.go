package httperror_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/httperror"
	"github.com/groobb/groobb/go/internal/i18n"
)

// TestNotFound verifies that the not-found response carries HTTP 404 with an
// HTML body, and renders the localized heading, explanation, and the link on to
// the top page for each supported locale. The status is asserted alongside the
// body because a page that reads as "not found" while answering 200 is a soft
// 404: the status is what crawlers and clients act on.
//
// [Ja] TestNotFound は not-found のレスポンスが HTTP 404 と HTML ボディを返し、
// サポートする各ロケールについてローカライズされた見出し・説明文・トップページへの
// リンクを描画することを検証します。ステータスをボディと併せて検証するのは、「見つから
// ない」と読めるページが 200 で応答する状態がソフト 404 だからです。クローラーや
// クライアントが従うのはステータスのほうです。
func TestNotFound(t *testing.T) {
	t.Parallel()

	renderer := httperror.NewRenderer(&config.Config{Env: "dev"})

	tests := []struct {
		name        string
		locale      string
		wantHeading string
		wantMessage string
		wantLink    string
	}{
		{
			name:        "Japanese",
			locale:      i18n.LangJa,
			wantHeading: "ページが見つかりません",
			wantMessage: "お探しのページは見つかりませんでした。",
			wantLink:    "トップページへ",
		},
		{
			name:        "English",
			locale:      i18n.LangEn,
			wantHeading: "Page not found",
			wantMessage: "The page you were looking for was not found.",
			wantLink:    "Go to the home page",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/no-such-page", nil)
			req = req.WithContext(i18n.SetLocale(req.Context(), tt.locale))
			rec := httptest.NewRecorder()

			renderer.NotFound(rec, req)

			if rec.Code != http.StatusNotFound {
				t.Errorf("status code = %d, want %d", rec.Code, http.StatusNotFound)
			}

			if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
				t.Errorf("Content-Type = %q, want %q", got, "text/html; charset=utf-8")
			}
			if got := rec.Header().Get("Cache-Control"); got != "private, no-store" {
				t.Errorf("Cache-Control = %q, want %q", got, "private, no-store")
			}

			body := rec.Body.String()
			wants := []string{
				tt.wantHeading,
				tt.wantMessage,
				tt.wantLink,
				`href="/"`,
				`lang="` + tt.locale + `"`,
			}
			for _, want := range wants {
				if !strings.Contains(body, want) {
					t.Errorf("response body does not contain %q", want)
				}
			}
		})
	}
}

// TestNotFoundFallsBackToPlainText verifies that a render failure still
// returns a plain-text 404 response with the same cache policy. A canceled
// request context makes the templ renderer fail before it writes the page.
//
// [Ja] TestNotFoundFallsBackToPlainText は、描画に失敗しても同じキャッシュ方針を持つ
// 平文の 404 レスポンスを返すことを検証します。キャンセル済みのリクエスト context に
// よって、templ の Renderer はページを書き込む前に失敗します。
func TestNotFoundFallsBackToPlainText(t *testing.T) {
	t.Parallel()

	renderer := httperror.NewRenderer(&config.Config{Env: "dev"})

	req := httptest.NewRequest(http.MethodGet, "/no-such-page", nil)
	ctx, cancel := context.WithCancel(req.Context())
	cancel()
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	renderer.NotFound(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type = %q, want %q", got, "text/plain; charset=utf-8")
	}
	if got := rec.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Errorf("Cache-Control = %q, want %q", got, "private, no-store")
	}
	if got := rec.Body.String(); got != "Not Found\n" {
		t.Errorf("response body = %q, want %q", got, "Not Found\n")
	}
}
