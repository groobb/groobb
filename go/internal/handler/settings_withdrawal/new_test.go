package settings_withdrawal_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/handler/settings_withdrawal"
	"github.com/groobb/groobb/go/internal/i18n"
)

// TestNew verifies that GET /settings/withdrawal/new returns HTTP 200 with an HTML
// body that renders the localized heading, the explanation of what withdrawal does,
// the current-password field (with autocomplete="current-password"), the form
// driving DELETE /settings/withdrawal via the _method override with the CSRF hidden
// field, the onsubmit confirm() guard, and the noindex robots meta, for each
// supported locale. New only renders (no database, no session), so the other
// dependencies are passed as nil.
//
// [Ja] TestNew は GET /settings/withdrawal/new が HTTP 200 と、サポートする各ロケールに
// ついて、ローカライズされた見出し・退会で何が起きるかの説明・現在のパスワードフィールド
// (autocomplete="current-password" 付き)・_method オーバーライド経由で
// DELETE /settings/withdrawal を動かす CSRF hidden フィールド付きフォーム・onsubmit の
// confirm() ガード・noindex の robots メタを描画した HTML ボディを返すことを検証します。
// New は描画のみ (DB もセッションも使わない) のため、他の依存は nil を渡します。
func TestNew(t *testing.T) {
	t.Parallel()

	handler := settings_withdrawal.NewHandler(&config.Config{Env: "dev"}, nil, nil, nil)

	tests := []struct {
		name          string
		locale        string
		wantHeading   string
		wantSubmit    string
		wantHeaderNav string
	}{
		{name: "Japanese", locale: i18n.LangJa, wantHeading: "退会", wantSubmit: "退会する", wantHeaderNav: "グローバルナビゲーション"},
		{name: "English", locale: i18n.LangEn, wantHeading: "Delete account", wantSubmit: "Delete account", wantHeaderNav: "Global navigation"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/settings/withdrawal/new", nil)
			req = req.WithContext(i18n.SetLocale(req.Context(), tt.locale))
			rec := httptest.NewRecorder()

			handler.New(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("status code = %d, want %d", rec.Code, http.StatusOK)
			}
			if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
				t.Errorf("Content-Type = %q, want prefix %q", got, "text/html")
			}

			body := rec.Body.String()
			wants := []string{
				tt.wantHeading,
				tt.wantSubmit,
				`action="/settings/withdrawal"`,
				`method="POST"`,
				`name="_method" value="DELETE"`,
				`name="csrf_token"`,
				`name="current_password"`,
				`autocomplete="current-password"`,
				`onsubmit=`,
				`aria-label="` + tt.wantHeaderNav + `"`,
				`href="/home"`,
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
