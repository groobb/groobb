package sign_in_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/handler/sign_in"
	"github.com/groobb/groobb/go/internal/i18n"
)

// TestNew verifies that GET /sign_in returns HTTP 200 with an HTML form carrying
// the email and password fields and the CSRF hidden field, with the localized
// heading and required marker for each supported locale, while keeping the bare
// sign-in URL indexable and free of the shared signed-in header. New does not
// touch the session manager, UseCases, or the Turnstile verifier, so they are
// left nil here. This page stands in for the forms sharing
// components.RequiredFieldLabel: it confirms the shared marker reaches a page
// through the layout and the locale of the request.
//
// [Ja] TestNew は GET /sign_in が HTTP 200 と、email・password フィールド・CSRF hidden
// フィールドを持つ HTML フォームを、サポートする各ロケールのローカライズ済み見出しと必須
// マーカーとともに返し、素のサインイン URL をインデックス対象に保ち、サインイン済み
// ページ共通のヘッダーを持たないことを検証します。New はセッションマネージャ・
// UseCase・Turnstile 検証器に触れないため、ここでは nil にします。本ページは
// components.RequiredFieldLabel を共有するフォームの代表であり、共通マーカーが
// レイアウトとリクエストのロケールを通してページに届くことを確認します。
func TestNew(t *testing.T) {
	t.Parallel()

	handler := sign_in.NewHandler(&config.Config{Env: "test"}, nil, nil, nil, nil)

	tests := []struct {
		name         string
		locale       string
		wantHeading  string
		wantRequired string
		noHeaderNav  string
	}{
		{name: "Japanese", locale: i18n.LangJa, wantHeading: "Groobb にログイン", wantRequired: "必須", noHeaderNav: "グローバルナビゲーション"},
		{name: "English", locale: i18n.LangEn, wantHeading: "Sign in to Groobb", wantRequired: "Required", noHeaderNav: "Global navigation"},
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
				tt.wantRequired,
				`<label for="email"`,
				`<label for="password"`,
				`name="email"`,
				`name="password"`,
				`name="csrf_token"`,
				`action="/sign_in"`,
			} {
				if !strings.Contains(body, want) {
					t.Errorf("body does not contain %q", want)
				}
			}
			if strings.Contains(body, `name="robots" content="noindex"`) {
				t.Error("素の /sign_in に noindex が含まれている")
			}
			if strings.Contains(body, `aria-label="`+tt.noHeaderNav+`"`) {
				t.Error("未サインインのサインインページにサインイン済みページ共通のヘッダーが含まれている")
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

// TestNew_ReturnTo verifies that every GET /sign_in URL carrying return_to is
// noindex, while only a destination accepted for redirect is echoed into the form
// as a hidden field.
//
// [Ja] TestNew_ReturnTo は、return_to を持つすべての GET /sign_in URL が noindex になり、
// リダイレクト先として受理された遷移先だけが hidden フィールドとしてフォームに
// エコーバックされることを検証します。
func TestNew_ReturnTo(t *testing.T) {
	t.Parallel()

	handler := sign_in.NewHandler(&config.Config{Env: "test"}, nil, nil, nil, nil)

	tests := []struct {
		name          string
		returnTo      string
		wantHidden    bool
		wantAttribute string
	}{
		{
			name:          "同一オリジンの相対パスは hidden フィールドで引き継ぐ",
			returnTo:      "/settings",
			wantHidden:    true,
			wantAttribute: `value="/settings"`,
		},
		{
			name:       "別オリジンを指す値は引き継がない",
			returnTo:   "https://evil.example.com/settings",
			wantHidden: false,
		},
		{
			name:       "空値は引き継がない",
			returnTo:   "",
			wantHidden: false,
		},
	}

	// A URL carrying return_to is a duplicate of the same form even when its value
	// is rejected, so every parameterized variant is noindex; the bare /sign_in
	// checked by TestNew stays indexable.
	//
	// [Ja] return_to を持つ URL は、値が拒否される場合も同じフォームの重複であるため、
	// パラメータ付きの全バリアントを noindex とする。TestNew が確認する素の /sign_in は
	// インデックス対象のままである。
	const noIndexTag = `name="robots" content="noindex"`

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			target := "/sign_in?return_to=" + url.QueryEscape(tt.returnTo)
			req := httptest.NewRequest(http.MethodGet, target, nil)
			req = req.WithContext(i18n.SetLocale(req.Context(), i18n.LangJa))
			rec := httptest.NewRecorder()

			handler.New(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
			}
			body := rec.Body.String()
			if got := strings.Contains(body, `name="return_to"`); got != tt.wantHidden {
				t.Errorf("return_to の hidden フィールドの有無 = %v, want %v", got, tt.wantHidden)
			}
			if tt.wantHidden && !strings.Contains(body, tt.wantAttribute) {
				t.Errorf("body does not contain %q", tt.wantAttribute)
			}
			if !strings.Contains(body, noIndexTag) {
				t.Error("return_to 付きの /sign_in に noindex が含まれていない")
			}
		})
	}
}
