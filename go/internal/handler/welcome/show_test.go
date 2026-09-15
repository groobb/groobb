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

// TestShowはトップページがHTTP 200と、サポートする各ロケールについて、
// ローカライズされたヒーロー文言・サインアップ / サインインのCTA・リクエスト
// ロケールのlang属性・フッター・バージョン付きのアセット参照を描画したHTML
// ボディを返すこと、そして認証の背後のページに属するサインイン済みページ共通の
// ヘッダーを持たないことを検証します。
func TestShow(t *testing.T) {
	t.Parallel()

	handler := welcome.NewHandler(&config.Config{Env: "dev"})

	tests := []struct {
		name           string
		locale         model.Locale
		wantHeading    string
		wantSignUpLink string
		wantSignInLink string
		noHeaderNav    string
	}{
		{name: "日本語", locale: model.LocaleJa, wantHeading: "あなたの掲示板を、つくろう。", wantSignUpLink: "アカウント登録", wantSignInLink: "ログイン", noHeaderNav: "グローバルナビゲーション"},
		{name: "英語", locale: model.LocaleEn, wantHeading: "Create your own bulletin board.", wantSignUpLink: "Sign up", wantSignInLink: "Sign in", noHeaderNav: "Global navigation"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req = req.WithContext(i18n.SetLocale(req.Context(), tt.locale))
			rec := httptest.NewRecorder()

			handler.Show(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
			}

			if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
				t.Errorf("Content-Type = %q、期待値の接頭辞 = %q", got, "text/html")
			}

			body := rec.Body.String()
			wants := []string{
				tt.wantHeading,
				tt.wantSignUpLink,
				tt.wantSignInLink,
				`href="/sign_up"`,
				`href="/sign_in"`,
				`lang="` + string(tt.locale) + `"`,
				"<footer",
				"Groobb",
				"/static/css/style.css?v=",
				"/static/js/main.js?v=",
			}
			for _, want := range wants {
				if !strings.Contains(body, want) {
					t.Errorf("レスポンスボディに %q が含まれていない", want)
				}
			}

			if strings.Contains(body, `aria-label="`+tt.noHeaderNav+`"`) {
				t.Error("未サインインのトップページにサインイン済みページ共通のヘッダーが含まれている")
			}
		})
	}
}

// TestShow_SignedInRedirectsToHomeは、トップページに来たサインイン済みの訪問者が
// ゲスト向けウェルカムではなく /homeへリダイレクトされることを検証します。既にサインイン
// 済みのユーザーが自分のホームページに着地するためです。ユーザーは (SetUserがするように)
// contextに直接載せ、認証ミドルウェアやDBなしでハンドラーを走らせます。
func TestShow_SignedInRedirectsToHome(t *testing.T) {
	t.Parallel()

	handler := welcome.NewHandler(&config.Config{Env: "dev"})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := i18n.SetLocale(req.Context(), model.LocaleJa)
	ctx = middleware.SetUserToContext(ctx, &model.User{Atname: "alice"})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.Show(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/home" {
		t.Errorf("Location = %q、期待値 = %q", loc, "/home")
	}
}
