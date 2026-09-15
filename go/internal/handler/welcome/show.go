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

// Show GET / - ゲスト向けトップページを描画し、サインイン済みユーザーは /homeへ
// リダイレクトします。トップページはゲストの着地面 (サインアップ / サインインへのCTAを
// 備えたウェルカム) です。サインイン後の着地面は /homeのため、ゲストページを見せずに
// そちらへ引き渡します。このルートにはSetUserを掛けており、セッションCookieから現在の
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
