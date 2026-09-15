package password_reset

import (
	"log/slog"
	"net/http"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/templates/layouts"
	passwordresetpage "github.com/groobb/groobb/go/internal/templates/pages/password_reset"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// New GET /password_reset/new - 新しいCSRFトークン付きでパスワードリセット申請
// フォームを描画します。
func (h *Handler) New(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	h.renderNew(w, r, http.StatusOK, passwordresetpage.NewPageData{
		CSRFToken: middleware.CSRFTokenFromContext(ctx),
	})
}

// renderNewは指定したステータスとデータでパスワードリセット申請フォームを描画
// します。New (200) と、Createのバリデーションエラー後の再描画 (422) で共有します。
// ステータスは描画前に書き込むため、呼び出し側は別途設定せずここに最終ステータスを渡します。
func (h *Handler) renderNew(w http.ResponseWriter, r *http.Request, status int, data passwordresetpage.NewPageData) {
	ctx := r.Context()

	// Turnstileのサイトキーはどの描画でも同じ (リクエストではなくconfig由来) なので、
	// 各呼び出し側ではなくここで一度だけ設定する。キーが空 (無効化されたdev / test構成) の
	// ときはウィジェットを何も描画しない。
	data.TurnstileSiteKey = h.cfg.TurnstileSiteKey

	meta := viewmodel.DefaultPageMeta(ctx, h.cfg)
	meta.Title = i18n.T(ctx, "password_reset_new_title")
	meta.Description = i18n.T(ctx, "password_reset_new_description")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := layouts.Default(meta, passwordresetpage.New(data)).Render(ctx, w); err != nil {
		// ステータスとヘッダーは既に送出済みのため、ここでは500に変えられず
		// ログに記録するのみとする。
		slog.ErrorContext(ctx, "パスワードリセット申請ページのレンダリングに失敗", "error", err)
	}
}
