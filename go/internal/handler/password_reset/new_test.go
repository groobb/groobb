package password_reset_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/handler/password_reset"
	"github.com/groobb/groobb/go/internal/i18n"
)

// getPasswordResetNew builds a GET /password_reset/new request with the locale
// set in its context.
//
// [Ja] getPasswordResetNew は GET /password_reset/new リクエストを組み立て、context に
// ロケールを設定する。
func getPasswordResetNew(locale string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/password_reset/new", nil)
	return req.WithContext(i18n.SetLocale(req.Context(), locale))
}

// TestNew verifies that GET /password_reset/new renders the request form (200)
// with the email field, the CSRF hidden field, and the heading.
//
// [Ja] TestNew は、GET /password_reset/new が email フィールド・CSRF hidden
// フィールド・見出しを伴って申請フォームを描画 (200) することを検証する。
func TestNew(t *testing.T) {
	t.Parallel()

	handler, _, _ := newPasswordResetHandler(t)

	rec := httptest.NewRecorder()
	handler.New(rec, getPasswordResetNew(i18n.LangJa))

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}

	body := rec.Body.String()
	checks := []string{
		`action="/password_reset"`,
		`name="email"`,
		`name="csrf_token"`,
		"パスワードを再設定",
	}
	for _, want := range checks {
		if !strings.Contains(body, want) {
			t.Errorf("レスポンスに %q が含まれていない", want)
		}
	}
}

// TestNew_English verifies the form localizes to English.
//
// [Ja] TestNew_English はフォームが英語にローカライズされることを検証する。
func TestNew_English(t *testing.T) {
	t.Parallel()

	handler, _, _ := newPasswordResetHandler(t)

	rec := httptest.NewRecorder()
	handler.New(rec, getPasswordResetNew(i18n.LangEn))

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), "Reset your password") {
		t.Error("英語の見出しが描画されていない")
	}
}

// TestNew_RendersTurnstileWidget verifies that when a Turnstile site key is
// configured, GET /password_reset/new renders the widget — the cf-turnstile div
// carrying the site key and the api.js script — confirming renderNew forwards
// cfg.TurnstileSiteKey into the form. New does not verify tokens, so the UseCase
// and verifier are left nil. The site key is Cloudflare's dummy testing key, kept
// to fixtures.
//
// [Ja] TestNew_RendersTurnstileWidget は、Turnstile のサイトキーが設定されているとき
// GET /password_reset/new がウィジェット (サイトキーを持つ cf-turnstile div と api.js
// スクリプト) を描画することを検証し、renderNew が cfg.TurnstileSiteKey をフォームへ
// 渡していることを確認する。New はトークンを検証しないため、UseCase と検証器は nil の
// ままにする。サイトキーは Cloudflare のダミーテストキーで、フィクスチャに留める。
func TestNew_RendersTurnstileWidget(t *testing.T) {
	t.Parallel()

	// Cloudflare's always-passing dummy site key, kept to test fixtures.
	//
	// [Ja] Cloudflare の「常に成功」ダミーサイトキー (テスト専用)。
	const dummySiteKey = "1x00000000000000000000AA"

	handler := password_reset.NewHandler(&config.Config{Env: "test", TurnstileSiteKey: dummySiteKey}, nil, nil)

	rec := httptest.NewRecorder()
	handler.New(rec, getPasswordResetNew(i18n.LangJa))

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
