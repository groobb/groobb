package welcome

import (
	"log/slog"
	"net/http"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/templates/layouts"
	welcomepage "github.com/groobb/groobb/go/internal/templates/pages/welcome"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// Show GET / - renders the top page.
//
// [Ja] Show GET / - トップページを描画します。
func (h *Handler) Show(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	meta := viewmodel.DefaultPageMeta(ctx, h.cfg)
	meta.Title = i18n.T(ctx, "welcome_show_title")
	meta.Description = i18n.T(ctx, "welcome_show_description")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := layouts.Default(meta, welcomepage.Show()).Render(ctx, w); err != nil {
		slog.ErrorContext(ctx, "トップページのレンダリングに失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
