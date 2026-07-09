package settings_email_confirmation_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/handler/settings_email_confirmation"
	"github.com/groobb/groobb/go/internal/i18n"
)

// TestNew verifies that GET /settings/email/confirmation/new returns HTTP 200 with
// an HTML body that renders the localized heading, the code field, and the form
// driving POST /settings/email/confirmation with the CSRF hidden field, plus the
// noindex robots meta, for each supported locale. New does not read the user or the
// database, so the handler runs without them; the flash manager and UseCase are
// unused by New, so nil is passed.
//
// [Ja] TestNew は GET /settings/email/confirmation/new が HTTP 200 と、サポートする各
// ロケールについて、ローカライズされた見出し・コードフィールド・CSRF hidden フィールド付きで
// POST /settings/email/confirmation を動かすフォーム、そして noindex の robots メタを描画した
// HTML ボディを返すことを検証します。New はユーザーも DB も読まないため、それらなしで
// ハンドラーを走らせます。フラッシュマネージャと UseCase は New では使われないため nil を
// 渡します。
func TestNew(t *testing.T) {
	t.Parallel()

	handler := settings_email_confirmation.NewHandler(&config.Config{Env: "dev"}, nil, nil)

	tests := []struct {
		name        string
		locale      string
		wantHeading string
		wantSubmit  string
	}{
		{name: "Japanese", locale: i18n.LangJa, wantHeading: "確認コードの入力", wantSubmit: "メールアドレスを変更"},
		{name: "English", locale: i18n.LangEn, wantHeading: "Enter confirmation code", wantSubmit: "Change email address"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/settings/email/confirmation/new", nil)
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
			wants := []string{
				tt.wantHeading,
				tt.wantSubmit,
				`action="/settings/email/confirmation"`,
				`method="POST"`,
				`name="csrf_token"`,
				`name="code"`,
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
