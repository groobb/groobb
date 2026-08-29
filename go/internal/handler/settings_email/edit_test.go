package settings_email_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/handler/settings_email"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/model"
)

// TestEdit verifies that GET /settings/email/edit returns HTTP 200 with an HTML
// body that renders the localized heading, the signed-in user's current email
// (read-only), the new-email and current-password fields, and the form driving
// PATCH /settings/email via the _method override with the CSRF hidden field, plus
// the noindex robots meta, for each supported locale. The user is placed in the
// context directly (as RequireAuth would), so the handler runs without the auth
// middleware or a database; the UseCase is unused by Edit, so nil is passed.
//
// [Ja] TestEdit は GET /settings/email/edit が HTTP 200 と、サポートする各ロケールに
// ついて、ローカライズされた見出し・サインイン済みユーザーの現在の email (読み取り専用)・
// 新しい email と現在のパスワードのフィールド・_method オーバーライド経由で
// PATCH /settings/email を動かす CSRF hidden フィールド付きフォーム、そして noindex の
// robots メタを描画した HTML ボディを返すことを検証します。ユーザーは (RequireAuth が
// するように) context に直接載せ、認証ミドルウェアや DB なしでハンドラーを走らせます。
// UseCase は Edit では使われないため nil を渡します。
func TestEdit(t *testing.T) {
	t.Parallel()

	handler := settings_email.NewHandler(&config.Config{Env: "dev"}, nil)

	tests := []struct {
		name          string
		locale        model.Locale
		wantHeading   string
		wantSubmit    string
		wantHeaderNav string
	}{
		{name: "Japanese", locale: model.LocaleJa, wantHeading: "メールアドレスの変更", wantSubmit: "変更する", wantHeaderNav: "グローバルナビゲーション"},
		{name: "English", locale: model.LocaleEn, wantHeading: "Change email address", wantSubmit: "Change email address", wantHeaderNav: "Global navigation"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/settings/email/edit", nil)
			ctx := i18n.SetLocale(req.Context(), tt.locale)
			ctx = middleware.SetUserToContext(ctx, &model.User{Email: "member@example.com", Atname: "alice"})
			req = req.WithContext(ctx)
			rec := httptest.NewRecorder()

			handler.Edit(rec, req)

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
				"member@example.com", // the current email, shown read-only
				`action="/settings/email"`,
				`method="POST"`,
				`name="_method" value="PATCH"`,
				`name="csrf_token"`,
				`name="email"`,
				`name="current_password"`,
				`aria-label="` + tt.wantHeaderNav + `"`,
				`href="/home"`,
				`<meta name="robots" content="noindex"`,
				`lang="` + string(tt.locale) + `"`,
			}
			for _, want := range wants {
				if !strings.Contains(body, want) {
					t.Errorf("response body does not contain %q", want)
				}
			}
		})
	}
}
