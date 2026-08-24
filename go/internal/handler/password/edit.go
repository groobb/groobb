package password

import (
	"log/slog"
	"net/http"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/templates/layouts"
	passwordpage "github.com/groobb/groobb/go/internal/templates/pages/password"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// Edit GET /password/edit - renders the new-password form, carrying the reset
// token from the link's ?token= query into a hidden field so the submit names
// which token to spend. The token is not validated here: a click on a stale link
// still shows the form, and validity (unknown / used / expired) is checked when
// the form is submitted (PATCH /password), where a token error can be shown in
// context. The CSRF token is read from the context the CSRF middleware populated.
//
// [Ja] Edit GET /password/edit - 新パスワードフォームを描画し、リンクの ?token= クエリの
// リセットトークンを hidden フィールドに運んで、送信がどのトークンを消費するか示します。
// ここではトークンを検証しません。失効したリンクのクリックでもフォームを表示し、有効性
// (未知 / 使用済み / 期限切れ) は送信時 (PATCH /password) に判定して、トークンエラーを
// その文脈で表示します。CSRF トークンは CSRF ミドルウェアが格納した context から読みます。
func (h *Handler) Edit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	h.renderEdit(w, r, http.StatusOK, passwordpage.EditPageData{
		CSRFToken: middleware.CSRFTokenFromContext(ctx),
		Token:     r.URL.Query().Get("token"),
	})
}

// renderEdit renders the new-password form with the given status and data. It is
// shared by Edit (200) and Update's re-render after a validation error (422). The
// status is written before rendering, so callers pass the final status here
// rather than setting it separately. The body can carry the plaintext password
// reset token, so every response rendered here uniformly uses Cache-Control:
// no-store to prevent HTTP caches from retaining that bearer secret.
//
// [Ja] renderEdit は指定したステータスとデータで新パスワードフォームを描画します。
// Edit (200) と、Update のバリデーションエラー後の再描画 (422) で共有します。ステータスは
// 描画前に書き込むため、呼び出し側は別途設定せずここに最終ステータスを渡します。ボディは
// 平文のパスワードリセットトークンを運びうるため、ここで描画するすべてのレスポンスに一律で
// Cache-Control: no-store を付け、HTTP キャッシュにその bearer secret が残らないようにします。
func (h *Handler) renderEdit(w http.ResponseWriter, r *http.Request, status int, data passwordpage.EditPageData) {
	ctx := r.Context()

	meta := viewmodel.DefaultPageMeta(ctx, h.cfg)
	meta.Title = i18n.T(ctx, "password_edit_title")
	meta.Description = i18n.T(ctx, "password_edit_description")

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := layouts.Default(meta, passwordpage.Edit(data)).Render(ctx, w); err != nil {
		// The status and headers are already sent, so this can only be logged,
		// not turned into a 500.
		//
		// [Ja] ステータスとヘッダーは既に送出済みのため、ここでは 500 に変えられず
		// ログに記録するのみとする。
		slog.ErrorContext(ctx, "パスワード更新ページのレンダリングに失敗", "error", err)
	}
}
