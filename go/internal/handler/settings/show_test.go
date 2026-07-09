package settings_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/handler/settings"
	"github.com/groobb/groobb/go/internal/i18n"
)

// TestShow verifies that GET /settings returns HTTP 200 with an HTML body that
// renders the localized heading and the email-change link (to /settings/email/edit),
// plus the noindex robots meta, for each supported locale. The hub reads no per-user
// data, so no user is placed in the context; it is registered behind RequireAuth in
// production, but the handler itself runs without the auth middleware or a database.
//
// [Ja] TestShow は GET /settings が HTTP 200 と、サポートする各ロケールについて、
// ローカライズされた見出しとメールアドレス変更リンク (/settings/email/edit 宛て)、
// そして noindex の robots メタを描画した HTML ボディを返すことを検証します。ハブは
// ユーザー固有のデータを読まないため context にユーザーを載せません。本番では
// RequireAuth の背後に登録されますが、ハンドラー自体は認証ミドルウェアや DB なしで
// 走ります。
func TestShow(t *testing.T) {
	t.Parallel()

	handler := settings.NewHandler(&config.Config{Env: "dev"})

	tests := []struct {
		name          string
		locale        string
		wantHeading   string
		wantEmailLink string
	}{
		{name: "Japanese", locale: i18n.LangJa, wantHeading: "設定", wantEmailLink: "メールアドレスの変更"},
		{name: "English", locale: i18n.LangEn, wantHeading: "Settings", wantEmailLink: "Change email address"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/settings", nil)
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
				tt.wantEmailLink,
				`href="/settings/email/edit"`,
				`<meta name="robots" content="noindex"`,
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
