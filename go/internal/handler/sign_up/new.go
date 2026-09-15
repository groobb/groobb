package sign_up

import (
	"log/slog"
	"net/http"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/templates/layouts"
	signuppage "github.com/groobb/groobb/go/internal/templates/pages/sign_up"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// New GET /sign_up - 新しいCSRFトークン付きでサインアップフォームを描画します。
func (h *Handler) New(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	h.renderNew(w, r, http.StatusOK, signuppage.NewPageData{
		CSRFToken: middleware.CSRFTokenFromContext(ctx),
	})
}

// renderNewは指定したステータスとデータでサインアップフォームを描画します。
// New (200) と、Createのバリデーションエラー後の再描画 (422) で共有します。ステータスは
// 描画前に書き込むため、呼び出し側は別途設定せずここに最終ステータスを渡します。
func (h *Handler) renderNew(w http.ResponseWriter, r *http.Request, status int, data signuppage.NewPageData) {
	ctx := r.Context()

	// Turnstileのサイトキーはどの描画でも同じ (リクエストではなくconfig由来) なので、
	// 各呼び出し側ではなくここで一度だけ設定する。キーが空 (無効化されたdev / test構成) の
	// ときはウィジェットを何も描画しない。
	data.TurnstileSiteKey = h.cfg.TurnstileSiteKey

	meta := viewmodel.DefaultPageMeta(ctx, h.cfg)
	meta.Title = i18n.T(ctx, "sign_up_new_title")
	meta.Description = i18n.T(ctx, "sign_up_new_description")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := layouts.Default(meta, signuppage.New(data)).Render(ctx, w); err != nil {
		// ステータスとヘッダーは既に送出済みのため、ここでは500に変えられず
		// ログに記録するのみとする。
		slog.ErrorContext(ctx, "サインアップページのレンダリングに失敗", "error", err)
	}
}
