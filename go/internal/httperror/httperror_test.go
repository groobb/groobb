package httperror_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/httperror"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
)

// TestNotFoundはnot-foundのレスポンスがHTTP 404とHTMLボディを返し、
// サポートする各ロケールについてローカライズされた見出し・説明文・トップページへの
// リンクを描画することを検証します。ステータスをボディと併せて検証するのは、「見つから
// ない」と読めるページが200で応答する状態がソフト404だからです。クローラーや
// クライアントが従うのはステータスのほうです。
func TestNotFound(t *testing.T) {
	t.Parallel()

	renderer := httperror.NewRenderer(&config.Config{Env: "dev"})

	tests := []struct {
		name        string
		locale      model.Locale
		wantHeading string
		wantMessage string
		wantLink    string
	}{
		{
			name:        "日本語",
			locale:      model.LocaleJa,
			wantHeading: "ページが見つかりません",
			wantMessage: "お探しのページは見つかりませんでした。",
			wantLink:    "トップページへ",
		},
		{
			name:        "英語",
			locale:      model.LocaleEn,
			wantHeading: "Page not found",
			wantMessage: "The page you were looking for was not found.",
			wantLink:    "Go to the home page",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/no-such-page", nil)
			req = req.WithContext(i18n.SetLocale(req.Context(), tt.locale))
			rec := httptest.NewRecorder()

			renderer.NotFound(rec, req)

			if rec.Code != http.StatusNotFound {
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusNotFound)
			}

			if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
				t.Errorf("Content-Type = %q、期待値 = %q", got, "text/html; charset=utf-8")
			}
			if got := rec.Header().Get("Cache-Control"); got != "private, no-store" {
				t.Errorf("Cache-Control = %q、期待値 = %q", got, "private, no-store")
			}

			body := rec.Body.String()
			wants := []string{
				tt.wantHeading,
				tt.wantMessage,
				tt.wantLink,
				`href="/"`,
				`lang="` + string(tt.locale) + `"`,
			}
			for _, want := range wants {
				if !strings.Contains(body, want) {
					t.Errorf("レスポンスボディに %q が含まれていない", want)
				}
			}
		})
	}
}

// TestNotFoundFallsBackToPlainTextは、描画に失敗しても同じキャッシュ方針を持つ
// 平文の404レスポンスを返すことを検証します。キャンセル済みのリクエストcontextに
// よって、templのRendererはページを書き込む前に失敗します。
func TestNotFoundFallsBackToPlainText(t *testing.T) {
	t.Parallel()

	renderer := httperror.NewRenderer(&config.Config{Env: "dev"})

	req := httptest.NewRequest(http.MethodGet, "/no-such-page", nil)
	ctx, cancel := context.WithCancel(req.Context())
	cancel()
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	renderer.NotFound(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusNotFound)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type = %q、期待値 = %q", got, "text/plain; charset=utf-8")
	}
	if got := rec.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Errorf("Cache-Control = %q、期待値 = %q", got, "private, no-store")
	}
	if got := rec.Body.String(); got != "Not Found\n" {
		t.Errorf("レスポンスボディ = %q、期待値 = %q", got, "Not Found\n")
	}
}

// TestNotFoundHasNoSignedInHeaderは404ページがサインイン済みページ共通の
// ヘッダーを描画しないことを検証します。ここが応じるルートには誰でも到達し、Rendererは
// ユーザーを解決しないため、認証の背後のページへ導くリンクを持つヘッダーは出しては
// なりません。
func TestNotFoundHasNoSignedInHeader(t *testing.T) {
	t.Parallel()

	renderer := httperror.NewRenderer(&config.Config{Env: "dev"})

	req := httptest.NewRequest(http.MethodGet, "/no-such-page", nil)
	req = req.WithContext(i18n.SetLocale(req.Context(), model.LocaleJa))
	rec := httptest.NewRecorder()

	renderer.NotFound(rec, req)

	if got := rec.Body.String(); strings.Contains(got, `aria-label="グローバルナビゲーション"`) {
		t.Error("レスポンスボディにサインイン済みのヘッダーナビゲーションが含まれている")
	}
}

// TestForbiddenはforbiddenのレスポンスがHTTP 403とHTMLボディを返し、サポート
// する各ロケールについてローカライズされた見出し・説明文・トップページへのリンクを描画する
// ことを検証します。あわせてnoindexの印も検証します。ここが応じるアドレスは実在する画面の
// ものであり、これが無ければクローラーに記録すべきページを与えてしまうためです。
func TestForbidden(t *testing.T) {
	t.Parallel()

	renderer := httperror.NewRenderer(&config.Config{Env: "dev"})

	tests := []struct {
		name        string
		locale      model.Locale
		wantHeading string
		wantMessage string
		wantLink    string
	}{
		{
			name:        "日本語",
			locale:      model.LocaleJa,
			wantHeading: "権限がありません",
			wantMessage: "この操作を行う権限がありません。",
			wantLink:    "トップページへ",
		},
		{
			name:        "英語",
			locale:      model.LocaleEn,
			wantHeading: "Access denied",
			wantMessage: "You do not have permission to do this.",
			wantLink:    "Go to the home page",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/admin", nil)
			req = req.WithContext(i18n.SetLocale(req.Context(), tt.locale))
			rec := httptest.NewRecorder()

			renderer.Forbidden(rec, req)

			if rec.Code != http.StatusForbidden {
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusForbidden)
			}

			if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
				t.Errorf("Content-Type = %q、期待値 = %q", got, "text/html; charset=utf-8")
			}
			if got := rec.Header().Get("Cache-Control"); got != "private, no-store" {
				t.Errorf("Cache-Control = %q、期待値 = %q", got, "private, no-store")
			}

			body := rec.Body.String()
			wants := []string{
				tt.wantHeading,
				tt.wantMessage,
				tt.wantLink,
				`href="/"`,
				`<meta name="robots" content="noindex"`,
				`lang="` + string(tt.locale) + `"`,
			}
			for _, want := range wants {
				if !strings.Contains(body, want) {
					t.Errorf("レスポンスボディに %q が含まれていない", want)
				}
			}
		})
	}
}

// TestForbiddenFallsBackToPlainTextは、描画に失敗しても同じキャッシュ方針を持つ
// 平文の403レスポンスを返すことを検証します。キャンセル済みのリクエストcontextに
// よって、templのRendererはページを書き込む前に失敗します。
func TestForbiddenFallsBackToPlainText(t *testing.T) {
	t.Parallel()

	renderer := httperror.NewRenderer(&config.Config{Env: "dev"})

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	ctx, cancel := context.WithCancel(req.Context())
	cancel()
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	renderer.Forbidden(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusForbidden)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type = %q、期待値 = %q", got, "text/plain; charset=utf-8")
	}
	if got := rec.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Errorf("Cache-Control = %q、期待値 = %q", got, "private, no-store")
	}
	if got := rec.Body.String(); got != "Forbidden\n" {
		t.Errorf("レスポンスボディ = %q、期待値 = %q", got, "Forbidden\n")
	}
}

// TestUnpublishedは、非公開のレスポンスがHTTP 404とHTMLボディを返し、サポートする
// 各ロケールについてローカライズされた見出し・説明文・トップページへのリンクを描画すること
// を検証します。あわせてnoindexの印も検証します。ここが応じるアドレスは、コミュニティが
// スレッドで応答していたものであり、これが無ければクローラーはそのスレッドの代わりにこの
// ページを記録してしまいます。
//
// ステータスをボディと併せて検証するのはTestNotFoundが述べる理由に加え、このページについて
// なされうる推測との違いがそこにあるためです。取り除かれたスレッドは404で応答します。
// そのアドレスが二度と応答しないことを述べる410ではありません。
func TestUnpublished(t *testing.T) {
	t.Parallel()

	renderer := httperror.NewRenderer(&config.Config{Env: "dev"})

	tests := []struct {
		name        string
		locale      model.Locale
		wantHeading string
		wantMessage string
		wantLink    string
	}{
		{
			name:        "日本語",
			locale:      model.LocaleJa,
			wantHeading: "このページは公開されていません",
			wantMessage: "このページは管理者により非公開にされました。",
			wantLink:    "トップページへ",
		},
		{
			name:        "英語",
			locale:      model.LocaleEn,
			wantHeading: "This page is not published",
			wantMessage: "This page was unpublished by an administrator.",
			wantLink:    "Go to the home page",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/t/1", nil)
			req = req.WithContext(i18n.SetLocale(req.Context(), tt.locale))
			rec := httptest.NewRecorder()

			renderer.Unpublished(rec, req)

			if rec.Code != http.StatusNotFound {
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusNotFound)
			}

			if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
				t.Errorf("Content-Type = %q、期待値 = %q", got, "text/html; charset=utf-8")
			}
			if got := rec.Header().Get("Cache-Control"); got != "private, no-store" {
				t.Errorf("Cache-Control = %q、期待値 = %q", got, "private, no-store")
			}

			body := rec.Body.String()
			wants := []string{
				tt.wantHeading,
				tt.wantMessage,
				tt.wantLink,
				`href="/"`,
				`<meta name="robots" content="noindex"`,
				`lang="` + string(tt.locale) + `"`,
			}
			for _, want := range wants {
				if !strings.Contains(body, want) {
					t.Errorf("レスポンスボディに %q が含まれていない", want)
				}
			}
		})
	}
}

// TestUnpublishedFallsBackToPlainTextは、描画に失敗しても同じキャッシュ方針を持つ
// 平文の404レスポンスを返すことを検証します。キャンセル済みのリクエストcontextに
// よって、templのRendererはページを書き込む前に失敗します。
func TestUnpublishedFallsBackToPlainText(t *testing.T) {
	t.Parallel()

	renderer := httperror.NewRenderer(&config.Config{Env: "dev"})

	req := httptest.NewRequest(http.MethodGet, "/t/1", nil)
	ctx, cancel := context.WithCancel(req.Context())
	cancel()
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	renderer.Unpublished(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusNotFound)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type = %q、期待値 = %q", got, "text/plain; charset=utf-8")
	}
	if got := rec.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Errorf("Cache-Control = %q、期待値 = %q", got, "private, no-store")
	}
	if got := rec.Body.String(); got != "Not Found\n" {
		t.Errorf("レスポンスボディ = %q、期待値 = %q", got, "Not Found\n")
	}
}
