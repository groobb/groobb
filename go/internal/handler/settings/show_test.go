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

// TestShow verifies that GET /settings returns HTTP 200 with an HTML body that
// renders the localized heading, the email-change link (to /settings/email/edit),
// the two-factor authentication link (to /settings/two_factor_auth/new), and the
// withdrawal link (to /settings/withdrawal/new), plus the shared signed-in header
// carrying the navigation back to home and the noindex robots meta, for each
// supported locale. The header's home link is left unmarked here, because the hub
// is not the page it points at; the current path is placed in the context (as
// CurrentPathMiddleware would), so the absence of aria-current is the component's
// own decision rather than a missing path. The hub reads no per-user data, so no
// user is placed in the context; it is registered behind RequireAuth in production,
// but the handler itself runs without the auth middleware or a database.
//
// [Ja] TestShow は GET /settings が HTTP 200 と、サポートする各ロケールについて、
// ローカライズされた見出し・メールアドレス変更リンク (/settings/email/edit 宛て)・2 段階
// 認証リンク (/settings/two_factor_auth/new 宛て)・退会リンク (/settings/withdrawal/new 宛て)、
// そしてホームへ戻る導線を持つサインイン済みページ共通のヘッダーと noindex の robots メタを
// 描画した HTML ボディを返すことを検証します。ハブはヘッダーのホームリンクが指すページでは
// ないため、ここではリンクに印が付きません。現在のパスは (CurrentPathMiddleware がするように)
// context に載せるので、aria-current が無いことはパスの欠落ではなくコンポーネント自身の判断に
// なります。ハブはユーザー固有のデータを読まないため context にユーザーを載せません。本番では
// RequireAuth の背後に登録されますが、ハンドラー自体は認証ミドルウェアや DB なしで走ります。
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
		{name: "Japanese", locale: model.LocaleJa, wantHeading: "設定", wantEmailLink: "メールアドレスの変更", wantTwoFactorLink: "2 段階認証", wantWithdrawalLink: "退会", wantHeaderNav: "グローバルナビゲーション"},
		{name: "English", locale: model.LocaleEn, wantHeading: "Settings", wantEmailLink: "Change email address", wantTwoFactorLink: "Two-factor authentication", wantWithdrawalLink: "Delete account", wantHeaderNav: "Global navigation"},
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
					t.Errorf("response body does not contain %q", want)
				}
			}

			if strings.Contains(body, `aria-current="page"`) {
				t.Error("設定ハブでヘッダーのホームリンクが現在ページとして印を付けられている")
			}
		})
	}
}
