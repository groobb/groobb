package email_confirmation

import (
	"log/slog"
	"net/http"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/templates/layouts"
	emailconfirmationpage "github.com/groobb/groobb/go/internal/templates/pages/email_confirmation"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// New GET /email_confirmation/new - 新しいCSRFトークン付きでコード入力フォームを
// 描画します。このフォームは確認が保留中のときだけ意味を持つため、受け渡しCookieの
// 無いリクエスト (例: 直接アクセス) はフローを始めさせるためサインアップへ戻します。
// ここではCookieのidをDBに照合しません。期限切れや使用済みのコードはコード送信時
// (Create) に捕捉するため、Newは薄い描画に留めます。
func (h *Handler) New(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if _, ok := h.sessionMgr.GetEmailConfirmationID(r); !ok {
		http.Redirect(w, r, "/sign_up", http.StatusSeeOther)
		return
	}

	h.renderNew(w, r, http.StatusOK, emailconfirmationpage.NewPageData{
		CSRFToken: middleware.CSRFTokenFromContext(ctx),
	})
}

// renderNewは指定したステータスとデータでコード入力フォームを描画します。
// New (200) と、Createのバリデーションエラー後の再描画 (422) で共有します。ステータスは
// 描画前に書き込むため、呼び出し側は別途設定せずここに最終ステータスを渡します。
func (h *Handler) renderNew(w http.ResponseWriter, r *http.Request, status int, data emailconfirmationpage.NewPageData) {
	ctx := r.Context()

	meta := viewmodel.DefaultPageMeta(ctx, h.cfg)
	meta.Title = i18n.T(ctx, "email_confirmation_new_title")
	meta.Description = i18n.T(ctx, "email_confirmation_new_description")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := layouts.Default(meta, emailconfirmationpage.New(data)).Render(ctx, w); err != nil {
		// ステータスとヘッダーは既に送出済みのため、ここでは500に変えられず
		// ログに記録するのみとする。
		slog.ErrorContext(ctx, "メール確認ページのレンダリングに失敗", "error", err)
	}
}
