package sign_in_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/handler/sign_in"
	"github.com/groobb/groobb/go/internal/i18n"
)

// TestNew verifies that GET /sign_in returns HTTP 200 with an HTML form carrying
// the email and password fields and the CSRF hidden field, with the localized
// heading for each supported locale. New does not touch the session manager,
// UseCases, or the Turnstile verifier, so they are left nil here.
//
// [Ja] TestNew は GET /sign_in が HTTP 200 と、email・password フィールド・CSRF hidden
// フィールドを持つ HTML フォームを、サポートする各ロケールのローカライズ済み見出しと
// ともに返すことを検証します。New はセッションマネージャ・UseCase・Turnstile 検証器に
// 触れないため、ここでは nil にします。
func TestNew(t *testing.T) {
	t.Parallel()

	handler := sign_in.NewHandler(&config.Config{Env: "test"}, nil, nil, nil, nil)

	tests := []struct {
		name        string
		locale      string
		wantHeading string
	}{
		{name: "Japanese", locale: i18n.LangJa, wantHeading: "Groobb にログイン"},
		{name: "English", locale: i18n.LangEn, wantHeading: "Sign in to Groobb"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/sign_in", nil)
			req = req.WithContext(i18n.SetLocale(req.Context(), tt.locale))
			rec := httptest.NewRecorder()

			handler.New(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("status code = %d, want %d", rec.Code, http.StatusOK)
			}
			if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
				t.Errorf("Content-Type = %q, want prefix %q", got, "text/html")
			}

			body := rec.Body.String()
			for _, want := range []string{
				tt.wantHeading,
				`name="email"`,
				`name="password"`,
				`name="csrf_token"`,
				`action="/sign_in"`,
			} {
				if !strings.Contains(body, want) {
					t.Errorf("body does not contain %q", want)
				}
			}
		})
	}
}

// TestNew_RendersTurnstileWidget verifies that when a Turnstile site key is
// configured, GET /sign_in renders the widget — the cf-turnstile div carrying the
// site key and the api.js script — confirming renderNew forwards
// cfg.TurnstileSiteKey into the form. New does not verify tokens, so the verifier
// is left nil. The site key is Cloudflare's dummy testing key, kept to fixtures.
//
// [Ja] TestNew_RendersTurnstileWidget は、Turnstile のサイトキーが設定されているとき
// GET /sign_in がウィジェット (サイトキーを持つ cf-turnstile div と api.js スクリプト) を
// 描画することを検証し、renderNew が cfg.TurnstileSiteKey をフォームへ渡していることを
// 確認します。New はトークンを検証しないため、検証器は nil のままにします。サイトキーは
// Cloudflare のダミーテストキーで、フィクスチャに留めます。
func TestNew_RendersTurnstileWidget(t *testing.T) {
	t.Parallel()

	// Cloudflare's always-passing dummy site key, kept to test fixtures.
	//
	// [Ja] Cloudflare の「常に成功」ダミーサイトキー (テスト専用)。
	const dummySiteKey = "1x00000000000000000000AA"

	handler := sign_in.NewHandler(&config.Config{Env: "test", TurnstileSiteKey: dummySiteKey}, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/sign_in", nil)
	req = req.WithContext(i18n.SetLocale(req.Context(), i18n.LangJa))
	rec := httptest.NewRecorder()

	handler.New(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	wants := []string{
		`class="cf-turnstile"`,
		`data-sitekey="1x00000000000000000000AA"`,
		"challenges.cloudflare.com/turnstile/v0/api.js",
	}
	for _, want := range wants {
		if !strings.Contains(body, want) {
			t.Errorf("response body does not contain %q", want)
		}
	}
}
