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
// renders the localized heading, the email-change link (to /settings/email/edit),
// the two-factor authentication link (to /settings/two_factor_auth/new), and the
// withdrawal link (to /settings/withdrawal/new), plus the noindex robots meta, for
// each supported locale. The hub reads no per-user data, so no user is placed in the
// context; it is registered behind RequireAuth in production, but the handler itself
// runs without the auth middleware or a database.
//
// [Ja] TestShow は GET /settings が HTTP 200 と、サポートする各ロケールについて、
// ローカライズされた見出し・メールアドレス変更リンク (/settings/email/edit 宛て)・2 段階
// 認証リンク (/settings/two_factor_auth/new 宛て)・退会リンク (/settings/withdrawal/new 宛て)、
// そして noindex の robots メタを描画した HTML ボディを返すことを検証します。ハブはユーザー
// 固有のデータを読まないため context にユーザーを載せません。本番では RequireAuth の背後に
// 登録されますが、ハンドラー自体は認証ミドルウェアや DB なしで走ります。
func TestShow(t *testing.T) {
	t.Parallel()

	handler := settings.NewHandler(&config.Config{Env: "dev"})

	tests := []struct {
		name               string
		locale             string
		wantHeading        string
		wantEmailLink      string
		wantTwoFactorLink  string
		wantWithdrawalLink string
	}{
		{name: "Japanese", locale: i18n.LangJa, wantHeading: "設定", wantEmailLink: "メールアドレスの変更", wantTwoFactorLink: "2 段階認証", wantWithdrawalLink: "退会"},
		{name: "English", locale: i18n.LangEn, wantHeading: "Settings", wantEmailLink: "Change email address", wantTwoFactorLink: "Two-factor authentication", wantWithdrawalLink: "Delete account"},
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
				tt.wantTwoFactorLink,
				`href="/settings/two_factor_auth/new"`,
				tt.wantWithdrawalLink,
				`href="/settings/withdrawal/new"`,
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
