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
	"github.com/groobb/groobb/go/internal/templates"
)

// TestShow verifies that GET /home returns HTTP 200 with an HTML body that
// renders the localized heading, a greeting addressed to the signed-in user's
// atname, and the sign-out form (POST /user_session via the _method=DELETE
// override, with the CSRF hidden field and a confirmation prompt), plus the
// shared signed-in header carrying the navigation back to home and the noindex
// robots meta, for each supported locale. The header's home link is marked
// aria-current="page" here, because home is the page being rendered. The user
// and the current path are placed in the context directly (as RequireAuth and
// CurrentPathMiddleware would), so the handler runs without those middlewares or
// a database.
//
// [Ja] TestShow は GET /home が HTTP 200 と、サポートする各ロケールについて、
// ローカライズされた見出し・サインイン済みユーザーの atname 宛ての挨拶・サインアウト
// フォーム (_method=DELETE オーバーライド経由の POST /user_session、CSRF hidden
// フィールドと確認文言つき)、そしてホームへ戻る導線を持つサインイン済みページ共通の
// ヘッダーと noindex の robots メタを描画した HTML ボディを返すことを検証します。
// ここではホームが今描画しているページなので、ヘッダーのホームリンクに
// aria-current="page" が付きます。ユーザーと現在のパスは (RequireAuth と
// CurrentPathMiddleware がするように) context に直接載せ、これらのミドルウェアや DB
// なしでハンドラーを走らせます。
func TestShow(t *testing.T) {
	t.Parallel()

	handler := home.NewHandler(&config.Config{Env: "dev"})

	tests := []struct {
		name             string
		locale           string
		wantHeading      string
		wantSignOutBtn   string
		wantSettingsLink string
		wantHeaderNav    string
	}{
		{name: "Japanese", locale: i18n.LangJa, wantHeading: "ホーム", wantSignOutBtn: "ログアウト", wantSettingsLink: "設定", wantHeaderNav: "グローバルナビゲーション"},
		{name: "English", locale: i18n.LangEn, wantHeading: "Home", wantSignOutBtn: "Sign out", wantSettingsLink: "Settings", wantHeaderNav: "Global navigation"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/home", nil)
			ctx := i18n.SetLocale(req.Context(), tt.locale)
			ctx = middleware.SetUserToContext(ctx, &model.User{Atname: "alice"})
			ctx = templates.SetCurrentPath(ctx, templates.HomePath().String())
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
				`aria-label="` + tt.wantHeaderNav + `"`,
				`href="/home"`,
				`aria-current="page"`,
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
