package settings_withdrawal_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/handler/settings_withdrawal"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
)

// TestNewはGET /settings/withdrawal/newがHTTP 200と、サポートする各ロケールに
// ついて、ローカライズされた見出し・退会で何が起きるかの説明・現在のパスワードフィールド
// (autocomplete="current-password" 付き)・_methodオーバーライド経由で
// DELETE /settings/withdrawalを動かすCSRF hiddenフィールド付きフォーム・onsubmitの
// confirm() ガード・noindexのrobotsメタを描画したHTMLボディを返すことを検証します。
// Newは描画のみ (DBもセッションも使わない) のため、他の依存はnilを渡します。
func TestNew(t *testing.T) {
	t.Parallel()

	handler := settings_withdrawal.NewHandler(&config.Config{Env: "dev"}, nil, nil, nil)

	tests := []struct {
		name          string
		locale        model.Locale
		wantHeading   string
		wantSubmit    string
		wantHeaderNav string
	}{
		{name: "日本語", locale: model.LocaleJa, wantHeading: "退会", wantSubmit: "退会する", wantHeaderNav: "グローバルナビゲーション"},
		{name: "英語", locale: model.LocaleEn, wantHeading: "Delete account", wantSubmit: "Delete account", wantHeaderNav: "Global navigation"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/settings/withdrawal/new", nil)
			req = req.WithContext(i18n.SetLocale(req.Context(), tt.locale))
			rec := httptest.NewRecorder()

			handler.New(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
			}
			if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
				t.Errorf("Content-Type = %q、期待値の接頭辞 = %q", got, "text/html")
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
				`lang="` + string(tt.locale) + `"`,
			}
			for _, want := range wants {
				if !strings.Contains(body, want) {
					t.Errorf("レスポンスボディに %q が含まれていない", want)
				}
			}
		})
	}
}
