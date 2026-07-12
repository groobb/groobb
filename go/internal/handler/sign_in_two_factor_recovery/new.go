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

// New GET /sign_in/two_factor/recovery/new - renders the recovery-code entry form
// with a fresh CSRF token. The form is only meaningful while a sign-in is pending
// its second factor, so a request without the pending cookie (e.g. direct
// navigation) is sent back to sign-in to start over. The cookie's user id is not
// resolved against the database here; a stale cookie or a wrong code is caught when
// the code is submitted (Create), keeping New a thin render.
//
// [Ja] New GET /sign_in/two_factor/recovery/new - 新しい CSRF トークン付きで
// リカバリーコード入力フォームを描画します。このフォームはサインインが第 2 要素を保留して
// いる間だけ意味を持つため、pending Cookie の無いリクエスト (例: 直接アクセス) はやり直させる
// ためサインインへ戻します。ここでは Cookie のユーザー id を DB に照合しません。失効した
// Cookie や誤ったコードはコード送信時 (Create) に捕捉するため、New は薄い描画に留めます。
func (h *Handler) New(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if _, ok := h.sessionMgr.GetTwoFactorPendingUserID(r); !ok {
		http.Redirect(w, r, templates.SignInPath().String(), http.StatusSeeOther)
		return
	}

	h.renderNew(w, r, http.StatusOK, signintwofactorrecoverypage.NewPageData{
		CSRFToken: middleware.CSRFTokenFromContext(ctx),
	})
}

// renderNew renders the recovery-code entry form with the given status and data. It
// is shared by New (200) and Create's re-render after a validation error (422). The
// status is written before rendering, so callers pass the final status here rather
// than setting it separately. The page is a transient, non-public authentication
// interstitial, so it is marked noindex to keep it out of search results.
//
// [Ja] renderNew は指定したステータスとデータでリカバリーコード入力フォームを描画します。
// New (200) と、Create のバリデーションエラー後の再描画 (422) で共有します。ステータスは
// 描画前に書き込むため、呼び出し側は別途設定せずここに最終ステータスを渡します。このページは
// 一時的で非公開の認証の中間ページのため、検索結果に出さないよう noindex を付けます。
func (h *Handler) renderNew(w http.ResponseWriter, r *http.Request, status int, data signintwofactorrecoverypage.NewPageData) {
	ctx := r.Context()

	meta := viewmodel.DefaultPageMeta(ctx, h.cfg)
	meta.Title = i18n.T(ctx, "sign_in_two_factor_recovery_new_title")
	meta.Description = i18n.T(ctx, "sign_in_two_factor_recovery_new_description")
	meta.NoIndex = true

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := layouts.Default(meta, signintwofactorrecoverypage.New(data)).Render(ctx, w); err != nil {
		// The status and headers are already sent, so this can only be logged, not
		// turned into a 500.
		//
		// [Ja] ステータスとヘッダーは既に送出済みのため、ここでは 500 に変えられず
		// ログに記録するのみとする。
		slog.ErrorContext(ctx, "リカバリーコードチャレンジページのレンダリングに失敗", "error", err)
	}
}
