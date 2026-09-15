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

// TestEditはGET /settings/email/editがHTTP 200と、サポートする各ロケールに
// ついて、ローカライズされた見出し・サインイン済みユーザーの現在のemail (読み取り専用)・
// 新しいemailと現在のパスワードのフィールド・_methodオーバーライド経由で
// PATCH /settings/emailを動かすCSRF hiddenフィールド付きフォーム、そしてnoindexの
// robotsメタを描画したHTMLボディを返すことを検証します。ユーザーは (RequireAuthが
// するように) contextに直接載せ、認証ミドルウェアやDBなしでハンドラーを走らせます。
// UseCaseはEditでは使われないためnilを渡します。
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
		{name: "日本語", locale: model.LocaleJa, wantHeading: "メールアドレスの変更", wantSubmit: "変更する", wantHeaderNav: "グローバルナビゲーション"},
		{name: "英語", locale: model.LocaleEn, wantHeading: "Change email address", wantSubmit: "Change email address", wantHeaderNav: "Global navigation"},
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
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
			}
			if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
				t.Errorf("Content-Type = %q、期待値の接頭辞 = %q", got, "text/html")
			}

			body := rec.Body.String()
			wants := []string{
				tt.wantHeading,
				tt.wantSubmit,
				"member@example.com", // 読み取り専用で表示する現在のメールアドレス
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
					t.Errorf("レスポンスボディに %q が含まれていない", want)
				}
			}
		})
	}
}
