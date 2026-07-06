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

// New GET /sign_up - renders the sign-up form with a fresh CSRF token.
//
// [Ja] New GET /sign_up - 新しい CSRF トークン付きでサインアップフォームを描画します。
func (h *Handler) New(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	h.renderNew(w, r, http.StatusOK, signuppage.NewPageData{
		CSRFToken: middleware.CSRFTokenFromContext(ctx),
	})
}

// renderNew renders the sign-up form with the given status and data. It is
// shared by New (200) and Create's re-render after a validation error (422). The
// status is written before rendering, so callers pass the final status here
// rather than setting it separately.
//
// [Ja] renderNew は指定したステータスとデータでサインアップフォームを描画します。
// New (200) と、Create のバリデーションエラー後の再描画 (422) で共有します。ステータスは
// 描画前に書き込むため、呼び出し側は別途設定せずここに最終ステータスを渡します。
func (h *Handler) renderNew(w http.ResponseWriter, r *http.Request, status int, data signuppage.NewPageData) {
	ctx := r.Context()

	// The Turnstile site key is the same for every render (it comes from config,
	// not the request), so set it here once rather than at each call site. An empty
	// key (the disabled dev / test setup) makes the widget render nothing.
	//
	// [Ja] Turnstile のサイトキーはどの描画でも同じ (リクエストではなく config 由来) なので、
	// 各呼び出し側ではなくここで一度だけ設定する。キーが空 (無効化された dev / test 構成) の
	// ときはウィジェットを何も描画しない。
	data.TurnstileSiteKey = h.cfg.TurnstileSiteKey

	meta := viewmodel.DefaultPageMeta(ctx, h.cfg)
	meta.Title = i18n.T(ctx, "sign_up_new_title")
	meta.Description = i18n.T(ctx, "sign_up_new_description")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := layouts.Default(meta, signuppage.New(data)).Render(ctx, w); err != nil {
		// The status and headers are already sent, so this can only be logged,
		// not turned into a 500.
		//
		// [Ja] ステータスとヘッダーは既に送出済みのため、ここでは 500 に変えられず
		// ログに記録するのみとする。
		slog.ErrorContext(ctx, "サインアップページのレンダリングに失敗", "error", err)
	}
}
