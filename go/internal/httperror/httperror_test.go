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

// TestNotFound verifies that the not-found response carries HTTP 404 with an
// HTML body, and renders the localized heading, explanation, and the link on to
// the top page for each supported locale. The status is asserted alongside the
// body because a page that reads as "not found" while answering 200 is a soft
// 404: the status is what crawlers and clients act on.
//
// [Ja] TestNotFound は not-found のレスポンスが HTTP 404 と HTML ボディを返し、
// サポートする各ロケールについてローカライズされた見出し・説明文・トップページへの
// リンクを描画することを検証します。ステータスをボディと併せて検証するのは、「見つから
// ない」と読めるページが 200 で応答する状態がソフト 404 だからです。クローラーや
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
			name:        "Japanese",
			locale:      model.LocaleJa,
			wantHeading: "ページが見つかりません",
			wantMessage: "お探しのページは見つかりませんでした。",
			wantLink:    "トップページへ",
		},
		{
			name:        "English",
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
				t.Errorf("status code = %d, want %d", rec.Code, http.StatusNotFound)
			}

			if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
				t.Errorf("Content-Type = %q, want %q", got, "text/html; charset=utf-8")
			}
			if got := rec.Header().Get("Cache-Control"); got != "private, no-store" {
				t.Errorf("Cache-Control = %q, want %q", got, "private, no-store")
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
					t.Errorf("response body does not contain %q", want)
				}
			}
		})
	}
}

// TestNotFoundFallsBackToPlainText verifies that a render failure still
// returns a plain-text 404 response with the same cache policy. A canceled
// request context makes the templ renderer fail before it writes the page.
//
// [Ja] TestNotFoundFallsBackToPlainText は、描画に失敗しても同じキャッシュ方針を持つ
// 平文の 404 レスポンスを返すことを検証します。キャンセル済みのリクエスト context に
// よって、templ の Renderer はページを書き込む前に失敗します。
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
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type = %q, want %q", got, "text/plain; charset=utf-8")
	}
	if got := rec.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Errorf("Cache-Control = %q, want %q", got, "private, no-store")
	}
	if got := rec.Body.String(); got != "Not Found\n" {
		t.Errorf("response body = %q, want %q", got, "Not Found\n")
	}
}

// TestNotFoundHasNoSignedInHeader verifies that the 404 page does not render the
// signed-in header. The route it answers is reached by anyone, and the renderer
// resolves no user, so the header — whose link leads to a page behind
// authentication — must not appear on it.
//
// [Ja] TestNotFoundHasNoSignedInHeader は 404 ページがサインイン済みページ共通の
// ヘッダーを描画しないことを検証します。ここが応じるルートには誰でも到達し、Renderer は
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
		t.Error("response body contains the signed-in header navigation")
	}
}

// TestForbidden verifies that the forbidden response carries HTTP 403 with an
// HTML body, and renders the localized heading, explanation, and the link on to
// the top page for each supported locale. It also asserts the noindex marker: the
// address it answers for is a screen that exists, so without it a crawler would
// have a page to record.
//
// [Ja] TestForbidden は forbidden のレスポンスが HTTP 403 と HTML ボディを返し、サポート
// する各ロケールについてローカライズされた見出し・説明文・トップページへのリンクを描画する
// ことを検証します。あわせて noindex の印も検証します。ここが応じるアドレスは実在する画面の
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
			name:        "Japanese",
			locale:      model.LocaleJa,
			wantHeading: "権限がありません",
			wantMessage: "この操作を行う権限がありません。",
			wantLink:    "トップページへ",
		},
		{
			name:        "English",
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
				t.Errorf("status code = %d, want %d", rec.Code, http.StatusForbidden)
			}

			if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
				t.Errorf("Content-Type = %q, want %q", got, "text/html; charset=utf-8")
			}
			if got := rec.Header().Get("Cache-Control"); got != "private, no-store" {
				t.Errorf("Cache-Control = %q, want %q", got, "private, no-store")
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
					t.Errorf("response body does not contain %q", want)
				}
			}
		})
	}
}

// TestForbiddenFallsBackToPlainText verifies that a render failure still returns
// a plain-text 403 response with the same cache policy. A canceled request
// context makes the templ renderer fail before it writes the page.
//
// [Ja] TestForbiddenFallsBackToPlainText は、描画に失敗しても同じキャッシュ方針を持つ
// 平文の 403 レスポンスを返すことを検証します。キャンセル済みのリクエスト context に
// よって、templ の Renderer はページを書き込む前に失敗します。
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
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type = %q, want %q", got, "text/plain; charset=utf-8")
	}
	if got := rec.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Errorf("Cache-Control = %q, want %q", got, "private, no-store")
	}
	if got := rec.Body.String(); got != "Forbidden\n" {
		t.Errorf("response body = %q, want %q", got, "Forbidden\n")
	}
}

// TestUnpublished verifies that the unpublished response carries HTTP 404 with
// an HTML body, and renders the localized heading, explanation, and the link on
// to the top page for each supported locale. It also asserts the noindex
// marker: the address it answers for is one the community answered with a
// thread, so without it a crawler would record this page in that thread's place.
//
// The status is asserted alongside the body for the reason TestNotFound gives,
// and because it is where this page differs from the guesses that would be made
// for it: a removed thread answers 404 rather than the 410 that would say the
// address will never answer again.
//
// [Ja] TestUnpublished は、非公開のレスポンスが HTTP 404 と HTML ボディを返し、サポートする
// 各ロケールについてローカライズされた見出し・説明文・トップページへのリンクを描画すること
// を検証します。あわせて noindex の印も検証します。ここが応じるアドレスは、コミュニティが
// スレッドで応答していたものであり、これが無ければクローラーはそのスレッドの代わりにこの
// ページを記録してしまいます。
//
// ステータスをボディと併せて検証するのは TestNotFound が述べる理由に加え、このページについて
// なされうる推測との違いがそこにあるためです。取り除かれたスレッドは 404 で応答します。
// そのアドレスが二度と応答しないことを述べる 410 ではありません。
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
			name:        "Japanese",
			locale:      model.LocaleJa,
			wantHeading: "このページは公開されていません",
			wantMessage: "このページは管理者により非公開にされました。",
			wantLink:    "トップページへ",
		},
		{
			name:        "English",
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
				t.Errorf("status code = %d, want %d", rec.Code, http.StatusNotFound)
			}

			if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
				t.Errorf("Content-Type = %q, want %q", got, "text/html; charset=utf-8")
			}
			if got := rec.Header().Get("Cache-Control"); got != "private, no-store" {
				t.Errorf("Cache-Control = %q, want %q", got, "private, no-store")
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
					t.Errorf("response body does not contain %q", want)
				}
			}
		})
	}
}

// TestUnpublishedFallsBackToPlainText verifies that a render failure still
// returns a plain-text 404 response with the same cache policy. A canceled
// request context makes the templ renderer fail before it writes the page.
//
// [Ja] TestUnpublishedFallsBackToPlainText は、描画に失敗しても同じキャッシュ方針を持つ
// 平文の 404 レスポンスを返すことを検証します。キャンセル済みのリクエスト context に
// よって、templ の Renderer はページを書き込む前に失敗します。
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
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type = %q, want %q", got, "text/plain; charset=utf-8")
	}
	if got := rec.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Errorf("Cache-Control = %q, want %q", got, "private, no-store")
	}
	if got := rec.Body.String(); got != "Not Found\n" {
		t.Errorf("response body = %q, want %q", got, "Not Found\n")
	}
}
