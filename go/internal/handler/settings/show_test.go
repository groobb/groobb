package settings_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/handler/settings"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/templates"
)

// TestShowはGET /settingsがHTTP 200と、サポートする各ロケールについて、
// ローカライズされた見出し・メールアドレス変更リンク (/settings/email/edit宛て)・2段階
// 認証リンク (/settings/two_factor_auth/new宛て)・退会リンク (/settings/withdrawal/new宛て)、
// そしてホームへ戻る導線を持つサインイン済みページ共通のヘッダーとnoindexのrobotsメタを
// 描画したHTMLボディを返すことを検証します。ハブはヘッダーのホームリンクが指すページでは
// ないため、ここではリンクに印が付きません。現在のパスは (CurrentPathMiddlewareがするように)
// contextに載せるので、aria-currentが無いことはパスの欠落ではなくコンポーネント自身の判断に
// なります。ハブはユーザー固有のデータを読まないためcontextにユーザーを載せません。本番では
// RequireAuthの背後に登録されますが、ハンドラー自体は認証ミドルウェアやDBなしで走ります。
func TestShow(t *testing.T) {
	t.Parallel()

	handler := settings.NewHandler(&config.Config{Env: "dev"})

	tests := []struct {
		name               string
		locale             model.Locale
		wantHeading        string
		wantEmailLink      string
		wantTwoFactorLink  string
		wantWithdrawalLink string
		wantHeaderNav      string
	}{
		{name: "日本語", locale: model.LocaleJa, wantHeading: "設定", wantEmailLink: "メールアドレスの変更", wantTwoFactorLink: "2段階認証", wantWithdrawalLink: "退会", wantHeaderNav: "グローバルナビゲーション"},
		{name: "英語", locale: model.LocaleEn, wantHeading: "Settings", wantEmailLink: "Change email address", wantTwoFactorLink: "Two-factor authentication", wantWithdrawalLink: "Delete account", wantHeaderNav: "Global navigation"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/settings", nil)
			ctx := i18n.SetLocale(req.Context(), tt.locale)
			ctx = templates.SetCurrentPath(ctx, templates.SettingsPath().String())
			req = req.WithContext(ctx)
			rec := httptest.NewRecorder()

			handler.Show(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
			}
			if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
				t.Errorf("Content-Type = %q、期待値の接頭辞 = %q", got, "text/html")
			}

			body := rec.Body.String()
			wants := []string{
				tt.wantHeading,
				tt.wantEmailLink,
				`href="/settings/email/edit"`,
				tt.wantTwoFactorLink,
				`href="/settings/two_factor_auth/new"`,
				tt.wantWithdrawalLink,
				`href="/settings/withdrawal/new"`,
				`aria-label="` + tt.wantHeaderNav + `"`,
				`href="/home"`,
				`id="settings-show-heading"`,
				`aria-labelledby="settings-show-heading"`,
				`<meta name="robots" content="noindex"`,
				`lang="` + string(tt.locale) + `"`,
			}
			for _, want := range wants {
				if !strings.Contains(body, want) {
					t.Errorf("レスポンスボディに %q が含まれていない", want)
				}
			}

			if strings.Contains(body, `aria-current="page"`) {
				t.Error("設定ハブでヘッダーのホームリンクが現在ページとして印を付けられている")
			}
		})
	}
}
