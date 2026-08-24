package settings_email

import (
	"log/slog"
	"net/http"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/templates/layouts"
	settingsemailpage "github.com/groobb/groobb/go/internal/templates/pages/settings_email"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// Edit GET /settings/email/edit - renders the email-change form, showing the
// account's current address and offering a new-address and current-password
// field. It is registered behind RequireAuth, which guarantees a signed-in user,
// so the user from the context is non-nil and the handler does not nil-check it.
// The page is per-user and behind authentication, so it is marked noindex to keep
// it out of search indexes. The CSRF token is read from the context the CSRF
// middleware populated.
//
// [Ja] Edit GET /settings/email/edit - メールアドレス変更フォームを描画し、アカウントの
// 現在のアドレスを表示して、新しいアドレスと現在のパスワードのフィールドを提供します。
// RequireAuth の背後に登録され、サインイン済みユーザーが保証されるため、context のユーザーは
// 非 nil であり、ハンドラーは nil チェックを持ちません。このページはユーザー固有かつ認証の
// 背後にあるため、検索インデックスから除外するよう noindex を付けます。CSRF トークンは CSRF
// ミドルウェアが格納した context から読みます。
func (h *Handler) Edit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)

	h.renderEdit(w, r, http.StatusOK, settingsemailpage.EditPageData{
		CSRFToken:    middleware.CSRFTokenFromContext(ctx),
		CurrentEmail: user.Email,
	})
}

// renderEdit renders the email-change form with the given status and data. It is
// shared by Edit (200) and Update's re-render after a validation error (422) or
// an enqueue failure (500). The status is written before rendering, so callers
// pass the final status here rather than setting it separately.
//
// [Ja] renderEdit は指定したステータスとデータでメールアドレス変更フォームを描画します。
// Edit (200) と、Update のバリデーションエラー後 (422) や enqueue 失敗後 (500) の再描画で
// 共有します。ステータスは描画前に書き込むため、呼び出し側は別途設定せずここに最終
// ステータスを渡します。
func (h *Handler) renderEdit(w http.ResponseWriter, r *http.Request, status int, data settingsemailpage.EditPageData) {
	ctx := r.Context()

	meta := viewmodel.DefaultPageMeta(ctx, h.cfg)
	meta.Title = i18n.T(ctx, "settings_email_edit_title")
	meta.Description = i18n.T(ctx, "settings_email_edit_description")
	meta.NoIndex = true
	meta.SignedIn = true

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := layouts.Default(meta, settingsemailpage.Edit(data)).Render(ctx, w); err != nil {
		// The status and headers are already sent, so this can only be logged,
		// not turned into a 500.
		//
		// [Ja] ステータスとヘッダーは既に送出済みのため、ここでは 500 に変えられず
		// ログに記録するのみとする。
		slog.ErrorContext(ctx, "メールアドレス変更ページのレンダリングに失敗", "error", err)
	}
}
