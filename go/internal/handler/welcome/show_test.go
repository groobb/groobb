package welcome_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/handler/welcome"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/model"
)

// TestShow verifies that the top page returns HTTP 200 with an HTML body that
// renders the localized hero text, the sign-up and sign-in calls to action,
// the request-locale lang attribute, the footer, and the versioned asset
// references for each supported locale.
//
// [Ja] TestShow はトップページが HTTP 200 と、サポートする各ロケールについて、
// ローカライズされたヒーロー文言・サインアップ / サインインの CTA・リクエスト
// ロケールの lang 属性・フッター・バージョン付きのアセット参照を描画した HTML
// ボディを返すことを検証します。
func TestShow(t *testing.T) {
	t.Parallel()

	handler := welcome.NewHandler(&config.Config{Env: "dev"})

	tests := []struct {
		name           string
		locale         string
		wantHeading    string
		wantSignUpLink string
		wantSignInLink string
	}{
		{name: "Japanese", locale: i18n.LangJa, wantHeading: "あなたの掲示板を、つくろう。", wantSignUpLink: "アカウント登録", wantSignInLink: "ログイン"},
		{name: "English", locale: i18n.LangEn, wantHeading: "Create your own bulletin board.", wantSignUpLink: "Sign up", wantSignInLink: "Sign in"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req = req.WithContext(i18n.SetLocale(req.Context(), tt.locale))
			rec := httptest.NewRecorder()

			handler.Show(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("status code = %d, want %d", rec.Code, http.StatusOK)
			}

			if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
				t.Errorf("Content-Type = %q, want prefix %q", got, "text/html")
			}

			body := rec.Body.String()
			wants := []string{
				tt.wantHeading,
				tt.wantSignUpLink,
				tt.wantSignInLink,
				`href="/sign_up"`,
				`href="/sign_in"`,
				`lang="` + tt.locale + `"`,
				"<footer",
				"Groobb",
				"/static/css/style.css?v=",
				"/static/js/main.js?v=",
			}
			for _, want := range wants {
				if !strings.Contains(body, want) {
					t.Errorf("response body does not contain %q", want)
				}
			}
		})
	}
}

// TestShow_SignedInRedirectsToHome verifies that a signed-in visitor to the top
// page is redirected to /home instead of being shown the guest welcome, so a
// user who is already signed in lands on their home page. The user is placed in
// the context directly (as SetUser would), so the handler runs without the auth
// middleware or a database.
//
// [Ja] TestShow_SignedInRedirectsToHome は、トップページに来たサインイン済みの訪問者が
// ゲスト向けウェルカムではなく /home へリダイレクトされることを検証します。既にサインイン
// 済みのユーザーが自分のホームページに着地するためです。ユーザーは (SetUser がするように)
// context に直接載せ、認証ミドルウェアや DB なしでハンドラーを走らせます。
func TestShow_SignedInRedirectsToHome(t *testing.T) {
	t.Parallel()

	handler := welcome.NewHandler(&config.Config{Env: "dev"})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := i18n.SetLocale(req.Context(), i18n.LangJa)
	ctx = middleware.SetUserToContext(ctx, &model.User{Atname: "alice"})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.Show(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/home" {
		t.Errorf("Location = %q, want %q", loc, "/home")
	}
}
