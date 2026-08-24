package settings_email_confirmation

import (
	"log/slog"
	"net/http"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/templates/layouts"
	settingsemailconfirmationpage "github.com/groobb/groobb/go/internal/templates/pages/settings_email_confirmation"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// New GET /settings/email/confirmation/new - renders the code-entry form with a
// fresh CSRF token. It is registered behind RequireAuth, which guarantees a
// signed-in user. The form is only meaningful while an email-change confirmation
// is pending, but that is not checked here: an expired, exhausted, or missing
// confirmation is caught when the code is submitted (Create), keeping New a thin
// render (no database round-trip). The page is per-user and behind authentication,
// so it is marked noindex to keep it out of search indexes. The CSRF token is read
// from the context the CSRF middleware populated.
//
// [Ja] New GET /settings/email/confirmation/new - 新しい CSRF トークン付きでコード入力
// フォームを描画します。RequireAuth の背後に登録され、サインイン済みユーザーが保証され
// ます。このフォームはメール変更の確認が保留中のときだけ意味を持ちますが、ここでは確認
// しません。期限切れ・使い切り・不在の確認はコード送信時 (Create) に捕捉するため、New は
// 薄い描画 (DB 往復なし) に留めます。このページはユーザー固有かつ認証の背後にあるため、
// 検索インデックスから除外するよう noindex を付けます。CSRF トークンは CSRF ミドルウェアが
// 格納した context から読みます。
func (h *Handler) New(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	h.renderNew(w, r, http.StatusOK, settingsemailconfirmationpage.NewPageData{
		CSRFToken: middleware.CSRFTokenFromContext(ctx),
	})
}

// renderNew renders the code-entry form with the given status and data. It is
// shared by New (200) and Create's re-render after a validation error (422). The
// status is written before rendering, so callers pass the final status here rather
// than setting it separately.
//
// [Ja] renderNew は指定したステータスとデータでコード入力フォームを描画します。New (200)
// と、Create のバリデーションエラー後の再描画 (422) で共有します。ステータスは描画前に
// 書き込むため、呼び出し側は別途設定せずここに最終ステータスを渡します。
func (h *Handler) renderNew(w http.ResponseWriter, r *http.Request, status int, data settingsemailconfirmationpage.NewPageData) {
	ctx := r.Context()

	meta := viewmodel.DefaultPageMeta(ctx, h.cfg)
	meta.Title = i18n.T(ctx, "settings_email_confirmation_new_title")
	meta.Description = i18n.T(ctx, "settings_email_confirmation_new_description")
	meta.NoIndex = true
	meta.SignedIn = true

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := layouts.Default(meta, settingsemailconfirmationpage.New(data)).Render(ctx, w); err != nil {
		// The status and headers are already sent, so this can only be logged, not
		// turned into a 500.
		//
		// [Ja] ステータスとヘッダーは既に送出済みのため、ここでは 500 に変えられず
		// ログに記録するのみとする。
		slog.ErrorContext(ctx, "メールアドレス変更確認ページのレンダリングに失敗", "error", err)
	}
}
