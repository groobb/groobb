package account

import (
	"log/slog"
	"net/http"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/templates/layouts"
	accountpage "github.com/groobb/groobb/go/internal/templates/pages/account"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// New GET /account/new - 新しいCSRFトークン付きでパスワード設定フォームを描画
// します。このフォームはサインアップの受け渡しが進行中のときだけ意味を持つため、受け渡し
// Cookieの無いリクエスト (例: 直接アクセス) はフローを始めさせるためサインアップへ戻し
// ます。ここではCookieの確認をDBに照合しません。それが検証済みの確認を指すかは
// フォーム送信時 (Create) に強制するため、Newは薄い描画に留めます。
func (h *Handler) New(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if _, ok := h.sessionMgr.GetEmailConfirmationID(r); !ok {
		http.Redirect(w, r, "/sign_up", http.StatusSeeOther)
		return
	}

	h.renderNew(w, r, http.StatusOK, accountpage.NewPageData{
		CSRFToken: middleware.CSRFTokenFromContext(ctx),
	})
}

// renderNewは指定したステータスとデータでパスワード設定フォームを描画します。
// New (200) と、Createのバリデーションエラー後の再描画 (422) で共有します。ステータスは
// 描画前に書き込むため、呼び出し側は別途設定せずここに最終ステータスを渡します。
func (h *Handler) renderNew(w http.ResponseWriter, r *http.Request, status int, data accountpage.NewPageData) {
	ctx := r.Context()

	meta := viewmodel.DefaultPageMeta(ctx, h.cfg)
	meta.Title = i18n.T(ctx, "account_new_title")
	meta.Description = i18n.T(ctx, "account_new_description")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := layouts.Default(meta, accountpage.New(data)).Render(ctx, w); err != nil {
		// ステータスとヘッダーは既に送出済みのため、ここでは500に変えられず
		// ログに記録するのみとする。
		slog.ErrorContext(ctx, "アカウント作成ページのレンダリングに失敗", "error", err)
	}
}
