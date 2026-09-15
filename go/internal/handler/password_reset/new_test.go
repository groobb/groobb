package password_reset_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/handler/password_reset"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/testutil"
)

// getPasswordResetNewはGET /password_reset/newリクエストを組み立て、contextに
// ロケールを設定する。
func getPasswordResetNew(locale model.Locale) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/password_reset/new", nil)
	return req.WithContext(i18n.SetLocale(req.Context(), locale))
}

// TestNewは、GET /password_reset/newがemailフィールド・CSRF hidden
// フィールド・見出しを伴って申請フォームを描画 (200) することを検証する。
func TestNew(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler, _, _ := newPasswordResetHandler(t, db)

	rec := httptest.NewRecorder()
	handler.New(rec, getPasswordResetNew(model.LocaleJa))

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Errorf("Content-Type = %q、期待値 = text/html", ct)
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

// TestNew_Englishはフォームが英語にローカライズされることを検証する。
func TestNew_English(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler, _, _ := newPasswordResetHandler(t, db)

	rec := httptest.NewRecorder()
	handler.New(rec, getPasswordResetNew(model.LocaleEn))

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), "Reset your password") {
		t.Error("英語の見出しが描画されていない")
	}
}

// TestNew_RendersTurnstileWidgetは、Turnstileのサイトキーが設定されているとき
// GET /password_reset/newがウィジェット (サイトキーを持つcf-turnstile divとapi.js
// スクリプト) を描画することを検証し、renderNewがcfg.TurnstileSiteKeyをフォームへ
// 渡していることを確認する。Newはトークンを検証しないため、UseCaseと検証器はnilの
// ままにする。サイトキーはCloudflareのダミーテストキーで、フィクスチャに留める。
func TestNew_RendersTurnstileWidget(t *testing.T) {
	t.Parallel()

	// Cloudflareの「常に成功」ダミーサイトキー (テスト専用)。
	const dummySiteKey = "1x00000000000000000000AA"

	handler := password_reset.NewHandler(&config.Config{Env: "test", TurnstileSiteKey: dummySiteKey}, nil, nil)

	rec := httptest.NewRecorder()
	handler.New(rec, getPasswordResetNew(model.LocaleJa))

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
