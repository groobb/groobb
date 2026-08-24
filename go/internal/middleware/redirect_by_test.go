package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

// TestRedirectBy verifies that a response which sends the client elsewhere names
// Groobb as the layer that sent it, and that a response which does not redirect
// carries no such claim.
//
// The 301 is produced by the trailing-slash normalization and the 303 by the
// shape RequireAuth uses, so the two redirects the application issues today are
// both covered by a real issuer rather than a hand-written status. The 304 is a
// 3xx that is not a redirect, and it is here because a status range alone would
// mark it.
//
// [Ja] TestRedirectBy は、クライアントを別の場所へ送るレスポンスが、送った層として Groobb を
// 示すこと、そして送らないレスポンスがその主張を伴わないことを検証します。
//
// 301 は末尾スラッシュの正規化が生み、303 は RequireAuth が使う形であるため、現時点で
// アプリケーションが発行する 2 つのリダイレクトは、どちらも手書きのステータスではなく実際の
// 発行元によって覆われています。304 はリダイレクトではない 3xx であり、ステータスの範囲だけ
// では印を付けてしまうため、ここに置いています。
func TestRedirectBy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		handler        http.Handler
		target         string
		wantStatus     int
		wantRedirectBy string
	}{
		{
			name:           "the trailing-slash normalization names its 301",
			handler:        chimiddleware.RedirectSlashes(http.NotFoundHandler()),
			target:         "/settings/email/edit/",
			wantStatus:     http.StatusMovedPermanently,
			wantRedirectBy: "groobb",
		},
		{
			name: "a redirect to the sign-in page names its 303",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, "/sign_in", http.StatusSeeOther)
			}),
			target:         "/settings",
			wantStatus:     http.StatusSeeOther,
			wantRedirectBy: "groobb",
		},
		{
			name: "a permanent redirect that keeps the method is named too",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, "/home", http.StatusPermanentRedirect)
			}),
			target:         "/old",
			wantStatus:     http.StatusPermanentRedirect,
			wantRedirectBy: "groobb",
		},
		{
			name: "a page is not a redirect",
			handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}),
			target:     "/home",
			wantStatus: http.StatusOK,
		},
		{
			name: "a body written without a status is not a redirect",
			handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if _, err := w.Write([]byte("<!doctype html>")); err != nil {
					t.Errorf("failed to write the response body: %v", err)
				}
			}),
			target:     "/home",
			wantStatus: http.StatusOK,
		},
		{
			name:       "the error page is not a redirect",
			handler:    http.NotFoundHandler(),
			target:     "/missing",
			wantStatus: http.StatusNotFound,
		},
		{
			name: "a 3xx without a destination is not a redirect",
			handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotModified)
			}),
			target:     "/static/css/style.css",
			wantStatus: http.StatusNotModified,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			RedirectBy(tt.handler).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.target, nil))

			if rec.Code != tt.wantStatus {
				t.Errorf("status code = %d, want %d", rec.Code, tt.wantStatus)
			}
			if got := rec.Header().Get("Redirect-By"); got != tt.wantRedirectBy {
				t.Errorf("Redirect-By = %q, want %q", got, tt.wantRedirectBy)
			}
		})
	}
}

// TestRedirectByUnwrapsTheWriterUnderneath verifies that the writer this
// middleware hands down still exposes the server's own writer, which is what
// http.ResponseController needs to reach Flush and the other optional interfaces.
// The wrapper covers every route, so a missing Unwrap would take those away site
// wide.
//
// [Ja] TestRedirectByUnwrapsTheWriterUnderneath は、このミドルウェアが下へ渡す
// ResponseWriter がサーバー自身の ResponseWriter を露出し続けることを検証します。それが
// http.ResponseController が Flush や他の追加インターフェースへ到達するのに必要なものです。
// このラッパーは全ルートを覆うため、Unwrap が無いとサイト全体でそれらが失われます。
func TestRedirectByUnwrapsTheWriterUnderneath(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()

	var flushed bool
	handler := RedirectBy(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if err := http.NewResponseController(w).Flush(); err != nil {
			t.Errorf("failed to flush the response: %v", err)
			return
		}
		flushed = true
	}))
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/home", nil))

	if !flushed {
		t.Error("the handler could not reach the flusher underneath")
	}
}
