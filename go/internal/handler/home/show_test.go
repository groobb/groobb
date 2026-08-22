package home_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/handler/home"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/model"
)

// TestShow verifies that GET /home returns HTTP 200 with an HTML body that
// renders the localized heading, a greeting addressed to the signed-in user's
// atname, and the sign-out form (POST /user_session via the _method=DELETE
// override, with the CSRF hidden field and a confirmation prompt), plus the
// noindex robots meta, for each supported locale. The user is placed in the
// context directly (as RequireAuth would), so the handler runs without the auth
// middleware or a database.
//
// [Ja] TestShow は GET /home が HTTP 200 と、サポートする各ロケールについて、
// ローカライズされた見出し・サインイン済みユーザーの atname 宛ての挨拶・サインアウト
// フォーム (_method=DELETE オーバーライド経由の POST /user_session、CSRF hidden
// フィールドと確認文言つき)、そして noindex の robots メタを描画した HTML ボディを
// 返すことを検証します。ユーザーは (RequireAuth がするように) context に直接載せ、
// 認証ミドルウェアや DB なしでハンドラーを走らせます。
func TestShow(t *testing.T) {
	t.Parallel()

	handler := home.NewHandler(&config.Config{Env: "dev"})

	tests := []struct {
		name             string
		locale           string
		wantHeading      string
		wantSignOutBtn   string
		wantSettingsLink string
	}{
		{name: "Japanese", locale: i18n.LangJa, wantHeading: "ホーム", wantSignOutBtn: "ログアウト", wantSettingsLink: "設定"},
		{name: "English", locale: i18n.LangEn, wantHeading: "Home", wantSignOutBtn: "Sign out", wantSettingsLink: "Settings"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/home", nil)
			ctx := i18n.SetLocale(req.Context(), tt.locale)
			ctx = middleware.SetUserToContext(ctx, &model.User{Atname: "alice"})
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
				tt.wantSignOutBtn,
				tt.wantSettingsLink,
				`href="/settings"`,
				"@alice",
				`action="/user_session"`,
				`method="POST"`,
				`name="_method" value="DELETE"`,
				`name="csrf_token"`,
				"data-confirm",
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
