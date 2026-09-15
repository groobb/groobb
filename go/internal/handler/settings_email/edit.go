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

// Edit GET /settings/email/edit - メールアドレス変更フォームを描画し、アカウントの
// 現在のアドレスを表示して、新しいアドレスと現在のパスワードのフィールドを提供します。
// RequireAuthの背後に登録され、サインイン済みユーザーが保証されるため、contextのユーザーは
// 非nilであり、ハンドラーはnilチェックを持ちません。このページはユーザー固有かつ認証の
// 背後にあるため、検索インデックスから除外するようnoindexを付けます。CSRFトークンはCSRF
// ミドルウェアが格納したcontextから読みます。
func (h *Handler) Edit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)

	h.renderEdit(w, r, http.StatusOK, settingsemailpage.EditPageData{
		CSRFToken:    middleware.CSRFTokenFromContext(ctx),
		CurrentEmail: user.Email,
	})
}

// renderEditは指定したステータスとデータでメールアドレス変更フォームを描画します。
// Edit (200) と、Updateのバリデーションエラー後 (422) やenqueue失敗後 (500) の再描画で
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
		// ステータスとヘッダーは既に送出済みのため、ここでは500に変えられず
		// ログに記録するのみとする。
		slog.ErrorContext(ctx, "メールアドレス変更ページのレンダリングに失敗", "error", err)
	}
}
