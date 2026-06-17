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

// New GET /account/new - renders the password-setup form with a fresh CSRF
// token. The form is only meaningful while the sign-up handoff is in progress,
// so a request without the handoff cookie (e.g. direct navigation) is sent back
// to sign-up to start the flow. The cookie's confirmation is not checked against
// the database here; whether it points at a verified confirmation is enforced
// when the form is submitted (Create), keeping New a thin render.
//
// [Ja] New GET /account/new - 新しい CSRF トークン付きでパスワード設定フォームを描画
// します。このフォームはサインアップの受け渡しが進行中のときだけ意味を持つため、受け渡し
// Cookie の無いリクエスト (例: 直接アクセス) はフローを始めさせるためサインアップへ戻し
// ます。ここでは Cookie の確認を DB に照合しません。それが検証済みの確認を指すかは
// フォーム送信時 (Create) に強制するため、New は薄い描画に留めます。
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

// renderNew renders the password-setup form with the given status and data. It
// is shared by New (200) and Create's re-render after a validation error (422).
// The status is written before rendering, so callers pass the final status here
// rather than setting it separately.
//
// [Ja] renderNew は指定したステータスとデータでパスワード設定フォームを描画します。
// New (200) と、Create のバリデーションエラー後の再描画 (422) で共有します。ステータスは
// 描画前に書き込むため、呼び出し側は別途設定せずここに最終ステータスを渡します。
func (h *Handler) renderNew(w http.ResponseWriter, r *http.Request, status int, data accountpage.NewPageData) {
	ctx := r.Context()

	meta := viewmodel.DefaultPageMeta(ctx, h.cfg)
	meta.Title = i18n.T(ctx, "account_new_title")
	meta.Description = i18n.T(ctx, "account_new_description")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := layouts.Default(meta, accountpage.New(data)).Render(ctx, w); err != nil {
		// The status and headers are already sent, so this can only be logged,
		// not turned into a 500.
		//
		// [Ja] ステータスとヘッダーは既に送出済みのため、ここでは 500 に変えられず
		// ログに記録するのみとする。
		slog.ErrorContext(ctx, "アカウント作成ページのレンダリングに失敗", "error", err)
	}
}
