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

// Edit GET /password/edit - 新パスワードフォームを描画し、リンクの ?token= クエリの
// リセットトークンをhiddenフィールドに運んで、送信がどのトークンを消費するか示します。
// ここではトークンを検証しません。失効したリンクのクリックでもフォームを表示し、有効性
// (未知 / 使用済み / 期限切れ) は送信時 (PATCH /password) に判定して、トークンエラーを
// その文脈で表示します。CSRFトークンはCSRFミドルウェアが格納したcontextから読みます。
func (h *Handler) Edit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	h.renderEdit(w, r, http.StatusOK, passwordpage.EditPageData{
		CSRFToken: middleware.CSRFTokenFromContext(ctx),
		Token:     r.URL.Query().Get("token"),
	})
}

// renderEditは指定したステータスとデータで新パスワードフォームを描画します。
// Edit (200) と、Updateのバリデーションエラー後の再描画 (422) で共有します。ステータスは
// 描画前に書き込むため、呼び出し側は別途設定せずここに最終ステータスを渡します。ボディは
// 平文のパスワードリセットトークンを運びうるため、ここで描画するすべてのレスポンスに一律で
// Cache-Control: no-storeを付け、HTTPキャッシュにそのbearer secretが残らないようにします。
func (h *Handler) renderEdit(w http.ResponseWriter, r *http.Request, status int, data passwordpage.EditPageData) {
	ctx := r.Context()

	meta := viewmodel.DefaultPageMeta(ctx, h.cfg)
	meta.Title = i18n.T(ctx, "password_edit_title")
	meta.Description = i18n.T(ctx, "password_edit_description")

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := layouts.Default(meta, passwordpage.Edit(data)).Render(ctx, w); err != nil {
		// ステータスとヘッダーは既に送出済みのため、ここでは500に変えられず
		// ログに記録するのみとする。
		slog.ErrorContext(ctx, "パスワード更新ページのレンダリングに失敗", "error", err)
	}
}
