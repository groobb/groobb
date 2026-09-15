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
	"github.com/groobb/groobb/go/internal/model"
)

// TestNewはGET /sign_inがHTTP 200と、email・passwordフィールド・CSRF hidden
// フィールドを持つHTMLフォームを、サポートする各ロケールのローカライズ済み見出しと必須
// マーカーとともに返し、素のサインインURLをインデックス対象に保ち、サインイン済み
// ページ共通のヘッダーを持たないことを検証します。Newはセッションマネージャ・
// UseCase・Turnstile検証器に触れないため、ここではnilにします。本ページは
// components.RequiredFieldLabelを共有するフォームの代表であり、共通マーカーが
// レイアウトとリクエストのロケールを通してページに届くことを確認します。
func TestNew(t *testing.T) {
	t.Parallel()

	handler := sign_in.NewHandler(&config.Config{Env: "test"}, nil, nil, nil, nil)

	tests := []struct {
		name         string
		locale       model.Locale
		wantHeading  string
		wantRequired string
		noHeaderNav  string
	}{
		{name: "日本語", locale: model.LocaleJa, wantHeading: "Groobbにログイン", wantRequired: "必須", noHeaderNav: "グローバルナビゲーション"},
		{name: "英語", locale: model.LocaleEn, wantHeading: "Sign in to Groobb", wantRequired: "Required", noHeaderNav: "Global navigation"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/sign_in", nil)
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
					t.Errorf("ボディに %q が含まれていない", want)
				}
			}
			if strings.Contains(body, `name="robots" content="noindex"`) {
				t.Error("素の /sign_inにnoindexが含まれている")
			}
			if strings.Contains(body, `aria-label="`+tt.noHeaderNav+`"`) {
				t.Error("未サインインのサインインページにサインイン済みページ共通のヘッダーが含まれている")
			}
		})
	}
}

// TestNew_RendersTurnstileWidgetは、Turnstileのサイトキーが設定されているとき
// GET /sign_inがウィジェット (サイトキーを持つcf-turnstile divとapi.jsスクリプト) を
// 描画することを検証し、renderNewがcfg.TurnstileSiteKeyをフォームへ渡していることを
// 確認します。Newはトークンを検証しないため、検証器はnilのままにします。サイトキーは
// Cloudflareのダミーテストキーで、フィクスチャに留めます。
func TestNew_RendersTurnstileWidget(t *testing.T) {
	t.Parallel()

	// Cloudflareの「常に成功」ダミーサイトキー (テスト専用)。
	const dummySiteKey = "1x00000000000000000000AA"

	handler := sign_in.NewHandler(&config.Config{Env: "test", TurnstileSiteKey: dummySiteKey}, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/sign_in", nil)
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

// TestNew_ReturnToは、return_toを持つすべてのGET /sign_in URLがnoindexになり、
// リダイレクト先として受理された遷移先だけがhiddenフィールドとしてフォームに
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
			name:          "同一オリジンの相対パスはhiddenフィールドで引き継ぐ",
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

	// return_toを持つURLは、値が拒否される場合も同じフォームの重複であるため、
	// パラメータ付きの全バリアントをnoindexとする。TestNewが確認する素の /sign_inは
	// インデックス対象のままである。
	const noIndexTag = `name="robots" content="noindex"`

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			target := "/sign_in?return_to=" + url.QueryEscape(tt.returnTo)
			req := httptest.NewRequest(http.MethodGet, target, nil)
			req = req.WithContext(i18n.SetLocale(req.Context(), model.LocaleJa))
			rec := httptest.NewRecorder()

			handler.New(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
			}
			body := rec.Body.String()
			if got := strings.Contains(body, `name="return_to"`); got != tt.wantHidden {
				t.Errorf("return_toのhiddenフィールドの有無 = %v、期待値 = %v", got, tt.wantHidden)
			}
			if tt.wantHidden && !strings.Contains(body, tt.wantAttribute) {
				t.Errorf("ボディに %q が含まれていない", tt.wantAttribute)
			}
			if !strings.Contains(body, noIndexTag) {
				t.Error("return_to付きの /sign_inにnoindexが含まれていない")
			}
		})
	}
}
