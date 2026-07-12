package settings_withdrawal

import (
	"log/slog"
	"net/http"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/templates/layouts"
	settingswithdrawalpage "github.com/groobb/groobb/go/internal/templates/pages/settings_withdrawal"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// New GET /settings/withdrawal/new - renders the account-withdrawal confirmation
// form: an explanation of what withdrawal does and a current-password field to
// re-authenticate. It is registered behind RequireAuth, which guarantees a
// signed-in user, so New is a thin render with no database round-trip. The page is
// per-user and behind authentication, so it is marked noindex to keep it out of
// search indexes. The CSRF token is read from the context the CSRF middleware
// populated.
//
// [Ja] New GET /settings/withdrawal/new - 退会確認フォームを描画します。退会で何が
// 起きるかの説明と、再認証のための現在のパスワードフィールドです。RequireAuth の背後に
// 登録され、サインイン済みユーザーが保証されるため、New は DB 往復のない薄い描画です。
// このページはユーザー固有かつ認証の背後にあるため、検索インデックスから除外するよう
// noindex を付けます。CSRF トークンは CSRF ミドルウェアが格納した context から読みます。
func (h *Handler) New(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	h.renderNew(w, r, http.StatusOK, settingswithdrawalpage.NewPageData{
		CSRFToken: middleware.CSRFTokenFromContext(ctx),
	})
}

// renderNew renders the withdrawal confirmation form with the given status and
// data. It is shared by New (200) and Delete's re-render after a validation error
// (422). The status is written before rendering, so callers pass the final status
// here rather than setting it separately.
//
// [Ja] renderNew は指定したステータスとデータで退会確認フォームを描画します。New (200)
// と、Delete のバリデーションエラー後の再描画 (422) で共有します。ステータスは描画前に
// 書き込むため、呼び出し側は別途設定せずここに最終ステータスを渡します。
func (h *Handler) renderNew(w http.ResponseWriter, r *http.Request, status int, data settingswithdrawalpage.NewPageData) {
	ctx := r.Context()

	meta := viewmodel.DefaultPageMeta(ctx, h.cfg)
	meta.Title = i18n.T(ctx, "settings_withdrawal_new_title")
	meta.Description = i18n.T(ctx, "settings_withdrawal_new_description")
	meta.NoIndex = true

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := layouts.Default(meta, settingswithdrawalpage.New(data)).Render(ctx, w); err != nil {
		// The status and headers are already sent, so this can only be logged, not
		// turned into a 500.
		//
		// [Ja] ステータスとヘッダーは既に送出済みのため、ここでは 500 に変えられず
		// ログに記録するのみとする。
		slog.ErrorContext(ctx, "退会確認ページのレンダリングに失敗", "error", err)
	}
}
