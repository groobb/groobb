package home

import (
	"log/slog"
	"net/http"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/templates/layouts"
	homepage "github.com/groobb/groobb/go/internal/templates/pages/home"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// Show GET /home - renders the signed-in user's home page: a greeting and a
// sign-out control. It is registered behind RequireAuth, which guarantees a
// signed-in user, so the user from the context is non-nil and the handler does
// not nil-check it. The page is per-user and behind authentication, so it is
// marked noindex to keep it out of search indexes. The sign-out form carries the
// CSRF token from the context for the double-submit-cookie check.
//
// [Ja] Show GET /home - サインイン済みユーザーのホームページ (挨拶とサインアウト操作) を
// 描画します。RequireAuth の背後に登録され、サインイン済みユーザーが保証されるため、
// context のユーザーは非 nil であり、ハンドラーは nil チェックを持ちません。このページは
// ユーザー固有かつ認証の背後にあるため、検索インデックスから除外するよう noindex を
// 付けます。サインアウトフォームは double-submit cookie 検証のため context の CSRF
// トークンを載せます。
func (h *Handler) Show(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)

	meta := viewmodel.DefaultPageMeta(ctx, h.cfg)
	meta.Title = i18n.T(ctx, "home_show_title")
	meta.NoIndex = true

	data := homepage.ShowPageData{
		Atname:    user.Atname,
		CSRFToken: middleware.CSRFTokenFromContext(ctx),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := layouts.Default(meta, homepage.Show(data)).Render(ctx, w); err != nil {
		slog.ErrorContext(ctx, "ホームページのレンダリングに失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
