package welcome

import (
	"log/slog"
	"net/http"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/templates/layouts"
	welcomepage "github.com/groobb/groobb/go/internal/templates/pages/welcome"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// Show GET / - renders the guest top page, or redirects a signed-in user to
// /home. The top page is the guest landing (a welcome with sign-up / sign-in
// calls to action); once signed in, a user's landing is /home, so this hands
// them off there instead of showing the guest page. This route is wrapped with
// SetUser, which resolves the current user from the session cookie, so a
// signed-in visitor is detected here.
//
// [Ja] Show GET / - ゲスト向けトップページを描画し、サインイン済みユーザーは /home へ
// リダイレクトします。トップページはゲストの着地面 (サインアップ / サインインへの CTA を
// 備えたウェルカム) です。サインイン後の着地面は /home のため、ゲストページを見せずに
// そちらへ引き渡します。このルートには SetUser を掛けており、セッション Cookie から現在の
// ユーザーを解決するため、サインイン済みの訪問者をここで検知できます。
func (h *Handler) Show(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if middleware.UserFromContext(ctx) != nil {
		http.Redirect(w, r, "/home", http.StatusSeeOther)
		return
	}

	meta := viewmodel.DefaultPageMeta(ctx, h.cfg)
	meta.Title = i18n.T(ctx, "welcome_show_title")
	meta.Description = i18n.T(ctx, "welcome_show_description")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := layouts.Default(meta, welcomepage.Show()).Render(ctx, w); err != nil {
		slog.ErrorContext(ctx, "トップページのレンダリングに失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
