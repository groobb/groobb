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
// heading for each supported locale. New does not touch the session manager or
// UseCases, so they are left nil here.
//
// [Ja] TestNew は GET /sign_in が HTTP 200 と、email・password フィールド・CSRF hidden
// フィールドを持つ HTML フォームを、サポートする各ロケールのローカライズ済み見出しと
// ともに返すことを検証します。New はセッションマネージャや UseCase に触れないため、ここ
// では nil にします。
func TestNew(t *testing.T) {
	t.Parallel()

	handler := sign_in.NewHandler(&config.Config{Env: "test"}, nil, nil, nil)

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
