package settings_email_confirmation_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/handler/settings_email_confirmation"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
)

// TestNewはGET /settings/email/confirmation/newがHTTP 200と、サポートする各
// ロケールについて、ローカライズされた見出し・コードフィールド・CSRF hiddenフィールド付きで
// POST /settings/email/confirmationを動かすフォーム、そしてnoindexのrobotsメタを描画した
// HTMLボディを返すことを検証します。NewはユーザーもDBも読まないため、それらなしで
// ハンドラーを走らせます。フラッシュマネージャとUseCaseはNewでは使われないためnilを
// 渡します。
func TestNew(t *testing.T) {
	t.Parallel()

	handler := settings_email_confirmation.NewHandler(&config.Config{Env: "dev"}, nil, nil)

	tests := []struct {
		name          string
		locale        model.Locale
		wantHeading   string
		wantSubmit    string
		wantHeaderNav string
	}{
		{name: "日本語", locale: model.LocaleJa, wantHeading: "確認コードの入力", wantSubmit: "メールアドレスを変更", wantHeaderNav: "グローバルナビゲーション"},
		{name: "英語", locale: model.LocaleEn, wantHeading: "Enter confirmation code", wantSubmit: "Change email address", wantHeaderNav: "Global navigation"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/settings/email/confirmation/new", nil)
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
				`action="/settings/email/confirmation"`,
				`method="POST"`,
				`name="csrf_token"`,
				`name="code"`,
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
