package sign_up_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/handler/sign_up"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
)

// TestNewはGET /sign_upがHTTP 200と、emailフィールド・CSRF hiddenフィールドを
// 持つHTMLフォームを、サポートする各ロケールのローカライズ済み見出しとともに返すことを
// 検証します。NewはセッションマネージャやUseCaseに触れないため、ここではnilにします。
func TestNew(t *testing.T) {
	t.Parallel()

	handler := sign_up.NewHandler(&config.Config{Env: "test"}, nil, nil, nil)

	tests := []struct {
		name        string
		locale      model.Locale
		wantHeading string
	}{
		{name: "日本語", locale: model.LocaleJa, wantHeading: "Groobbに登録"},
		{name: "英語", locale: model.LocaleEn, wantHeading: "Sign up for Groobb"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/sign_up", nil)
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
				`action="/sign_up"`,
				`method="POST"`,
				`name="csrf_token"`,
				`name="email"`,
				`type="email"`,
				`autocomplete="email"`,
				`<label for="email"`,
			}
			for _, want := range wants {
				if !strings.Contains(body, want) {
					t.Errorf("レスポンスボディに %q が含まれていない", want)
				}
			}
		})
	}
}

// TestNew_RendersTurnstileWidgetは、Turnstileのサイトキーが設定されているとき
// GET /sign_upがウィジェット (サイトキーを持つcf-turnstile divとapi.jsスクリプト) を
// 描画することを検証し、renderNewがcfg.TurnstileSiteKeyをフォームへ渡していることを
// 確認します。Newはトークンを検証しないため、検証器はnilのままにします。サイトキーは
// Cloudflareのダミーテストキーで、フィクスチャに留めます。
func TestNew_RendersTurnstileWidget(t *testing.T) {
	t.Parallel()

	// Cloudflareの「常に成功」ダミーサイトキー (テスト専用)。
	const dummySiteKey = "1x00000000000000000000AA"

	handler := sign_up.NewHandler(&config.Config{Env: "test", TurnstileSiteKey: dummySiteKey}, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/sign_up", nil)
	req = req.WithContext(i18n.SetLocale(req.Context(), model.LocaleJa))
	rec := httptest.NewRecorder()

	handler.New(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	wants := []string{
		`class="cf-turnstile"`,
		`data-sitekey="1x00000000000000000000AA"`,
		"challenges.cloudflare.com/turnstile/v0/api.js",
	}
	for _, want := range wants {
		if !strings.Contains(body, want) {
			t.Errorf("レスポンスボディに %q が含まれていない", want)
		}
	}
}
