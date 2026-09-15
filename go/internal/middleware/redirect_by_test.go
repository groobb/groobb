package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

// TestRedirectByは、クライアントを別の場所へ送るレスポンスが、送った層としてGroobbを
// 示すこと、そして送らないレスポンスがその主張を伴わないことを検証します。
//
// 301は末尾スラッシュの正規化が生み、303はRequireAuthが使う形であるため、現時点で
// アプリケーションが発行する2つのリダイレクトは、どちらも手書きのステータスではなく実際の
// 発行元によって覆われています。304はリダイレクトではない3xxであり、ステータスの範囲だけ
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
			name:           "末尾スラッシュの正規化による301に名前を付ける",
			handler:        chimiddleware.RedirectSlashes(http.NotFoundHandler()),
			target:         "/settings/email/edit/",
			wantStatus:     http.StatusMovedPermanently,
			wantRedirectBy: "groobb",
		},
		{
			name: "サインインページへの303に名前を付ける",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, "/sign_in", http.StatusSeeOther)
			}),
			target:         "/settings",
			wantStatus:     http.StatusSeeOther,
			wantRedirectBy: "groobb",
		},
		{
			name: "メソッドを保つ恒久リダイレクトにも名前を付ける",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, "/home", http.StatusPermanentRedirect)
			}),
			target:         "/old",
			wantStatus:     http.StatusPermanentRedirect,
			wantRedirectBy: "groobb",
		},
		{
			name: "ページはリダイレクトではない",
			handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}),
			target:     "/home",
			wantStatus: http.StatusOK,
		},
		{
			name: "ステータスを指定せずに書いたボディはリダイレクトではない",
			handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if _, err := w.Write([]byte("<!doctype html>")); err != nil {
					t.Errorf("レスポンスボディの書き込みに失敗: %v", err)
				}
			}),
			target:     "/home",
			wantStatus: http.StatusOK,
		},
		{
			name:       "エラーページはリダイレクトではない",
			handler:    http.NotFoundHandler(),
			target:     "/missing",
			wantStatus: http.StatusNotFound,
		},
		{
			name: "遷移先の無い3xxはリダイレクトではない",
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
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, tt.wantStatus)
			}
			if got := rec.Header().Get("Redirect-By"); got != tt.wantRedirectBy {
				t.Errorf("Redirect-By = %q、期待値 = %q", got, tt.wantRedirectBy)
			}
		})
	}
}

// TestRedirectByUnwrapsTheWriterUnderneathは、このミドルウェアが下へ渡す
// ResponseWriterがサーバー自身のResponseWriterを露出し続けることを検証します。それが
// http.ResponseControllerがFlushや他の追加インターフェースへ到達するのに必要なものです。
// このラッパーは全ルートを覆うため、Unwrapが無いとサイト全体でそれらが失われます。
func TestRedirectByUnwrapsTheWriterUnderneath(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()

	var flushed bool
	handler := RedirectBy(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if err := http.NewResponseController(w).Flush(); err != nil {
			t.Errorf("レスポンスのフラッシュに失敗: %v", err)
			return
		}
		flushed = true
	}))
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/home", nil))

	if !flushed {
		t.Error("ハンドラーが下層のFlusherに到達できなかった")
	}
}
