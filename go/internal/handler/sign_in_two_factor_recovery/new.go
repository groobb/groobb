package sign_in_two_factor_recovery

import (
	"log/slog"
	"net/http"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/templates"
	"github.com/groobb/groobb/go/internal/templates/layouts"
	signintwofactorrecoverypage "github.com/groobb/groobb/go/internal/templates/pages/sign_in_two_factor_recovery"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// New GET /sign_in/two_factor/recovery/new - 新しいCSRFトークン付きで
// リカバリーコード入力フォームを描画します。このフォームはサインインが第2要素を保留して
// いる間だけ意味を持つため、pending Cookieの無いリクエスト (例: 直接アクセス) はやり直させる
// ためサインインへ戻します。ここではCookieのユーザーidをDBに照合しません。失効した
// Cookieや誤ったコードはコード送信時 (Create) に捕捉するため、Newは薄い描画に留めます。
func (h *Handler) New(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// TOTPチャレンジがreturn_toをここへ引き継ぐため、フォームも下のTOTP
	// チャレンジへ戻るリンクも、訪問者が本来向かっていた遷移先を保てる。
	returnTo := middleware.SanitizeReturnTo(r.URL.Query().Get(templates.ReturnToParam))

	if _, ok := h.sessionMgr.GetTwoFactorPendingUserID(r); !ok {
		http.Redirect(w, r, templates.SignInPath().WithReturnTo(returnTo).String(), http.StatusSeeOther)
		return
	}

	h.renderNew(w, r, http.StatusOK, signintwofactorrecoverypage.NewPageData{
		CSRFToken: middleware.CSRFTokenFromContext(ctx),
		ReturnTo:  returnTo,
	})
}

// renderNewは指定したステータスとデータでリカバリーコード入力フォームを描画します。
// New (200) と、Createのバリデーションエラー後の再描画 (422) で共有します。ステータスは
// 描画前に書き込むため、呼び出し側は別途設定せずここに最終ステータスを渡します。このページは
// 一時的で非公開の認証の中間ページのため、検索結果に出さないようnoindexを付けます。
func (h *Handler) renderNew(w http.ResponseWriter, r *http.Request, status int, data signintwofactorrecoverypage.NewPageData) {
	ctx := r.Context()

	meta := viewmodel.DefaultPageMeta(ctx, h.cfg)
	meta.Title = i18n.T(ctx, "sign_in_two_factor_recovery_new_title")
	meta.Description = i18n.T(ctx, "sign_in_two_factor_recovery_new_description")
	meta.NoIndex = true

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := layouts.Default(meta, signintwofactorrecoverypage.New(data)).Render(ctx, w); err != nil {
		// ステータスとヘッダーは既に送出済みのため、ここでは500に変えられず
		// ログに記録するのみとする。
		slog.ErrorContext(ctx, "リカバリーコードチャレンジページのレンダリングに失敗", "error", err)
	}
}
