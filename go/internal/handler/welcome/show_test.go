package welcome_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/handler/welcome"
	"github.com/groobb/groobb/go/internal/i18n"
)

// TestShow verifies that the top page returns HTTP 200 with an HTML body that
// renders the localized hero text, the request-locale lang attribute, the
// footer, and the versioned asset references for each supported locale.
//
// [Ja] TestShow はトップページが HTTP 200 と、サポートする各ロケールについて、
// ローカライズされたヒーロー文言・リクエストロケールの lang 属性・フッター・
// バージョン付きのアセット参照を描画した HTML ボディを返すことを検証します。
func TestShow(t *testing.T) {
	t.Parallel()

	handler := welcome.NewHandler(&config.Config{Env: "dev"})

	tests := []struct {
		name        string
		locale      string
		wantHeading string
	}{
		{name: "Japanese", locale: i18n.LangJa, wantHeading: "あなたの掲示板を、つくろう。"},
		{name: "English", locale: i18n.LangEn, wantHeading: "Create your own bulletin board."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req = req.WithContext(i18n.SetLocale(req.Context(), tt.locale))
			rec := httptest.NewRecorder()

			handler.Show(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("status code = %d, want %d", rec.Code, http.StatusOK)
			}

			if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
				t.Errorf("Content-Type = %q, want prefix %q", got, "text/html")
			}

			body := rec.Body.String()
			wants := []string{
				tt.wantHeading,
				`lang="` + tt.locale + `"`,
				"<footer",
				"Groobb",
				"/static/css/style.css?v=",
				"/static/js/main.js?v=",
			}
			for _, want := range wants {
				if !strings.Contains(body, want) {
					t.Errorf("response body does not contain %q", want)
				}
			}
		})
	}
}
