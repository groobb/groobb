package password_reset

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/templates/layouts"
	passwordresetpage "github.com/groobb/groobb/go/internal/templates/pages/password_reset"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// Create POST /password_reset - accepts an email and, when it belongs to an
// account, issues a reset link and mails it. It always renders the same "sent"
// confirmation on success so the response never reveals whether the email is
// registered. A malformed email re-renders the form with the message (422); the
// CSRF check is enforced upstream by the CSRF middleware, so it is not repeated
// here. A genuine system error is logged but still shown as the sent page, so a
// failure on the existing-account path cannot leak account existence either.
//
// [Ja] Create POST /password_reset - email を受け付け、アカウントに属する場合はリセット
// リンクを発行してメールします。成功時は常に同じ "送信しました" の確認を描画するため、
// レスポンスはその email が登録済みかどうかを決して明かしません。形式不正の email は
// メッセージ付きでフォームを再描画します (422)。CSRF 検証は上流の CSRF ミドルウェアが
// 強制するため、ここでは繰り返しません。本物のシステムエラーはログに記録しつつも送信済み
// ページを表示し、実在アカウントの経路での失敗もアカウントの存在を漏らさないようにします。
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	email := r.FormValue("email")

	_, err := h.createPasswordResetTokenUC.Execute(ctx, usecase.CreatePasswordResetTokenInput{
		Email:  email,
		Locale: i18n.GetLocale(ctx),
	})
	if err != nil {
		var ve *model.ValidationError
		if errors.As(err, &ve) {
			h.renderNew(w, r, http.StatusUnprocessableEntity, passwordresetpage.NewPageData{
				CSRFToken:  middleware.CSRFTokenFromContext(ctx),
				Email:      email,
				FormErrors: ve,
			})
			return
		}

		// A genuine system failure (e.g. the database is unreachable). Log the
		// detail, but fall through to the same sent page rather than a 500: an
		// error response only on this path (reached only after the account is
		// found) would leak that the address belongs to an account.
		//
		// [Ja] 本物のシステム障害 (例: データベースに到達できない)。詳細はログに記録するが、
		// 500 ではなく同じ送信済みページにフォールスルーする。この経路 (アカウントが見つかった
		// 後にのみ到達する) でだけエラーレスポンスを返すと、そのアドレスがアカウントに属する
		// ことを漏らすため。
		slog.ErrorContext(ctx, "パスワードリセット申請の処理に失敗", "error", err)
	}

	h.renderSent(w, r)
}

// renderSent renders the post-submission confirmation page. It is shown for
// every accepted submission—an existing account, an unknown email, or even an
// internal error—so the response is identical regardless of whether the address
// is registered.
//
// [Ja] renderSent は送信後の確認ページを描画します。受理されたすべての送信 (実在
// アカウント・未知の email・内部エラーであっても) で表示するため、レスポンスはその
// アドレスが登録済みかどうかによらず同一になります。
func (h *Handler) renderSent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	meta := viewmodel.DefaultPageMeta(ctx, h.cfg)
	meta.Title = i18n.T(ctx, "password_reset_sent_title")
	meta.Description = i18n.T(ctx, "password_reset_sent_description")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := layouts.Default(meta, passwordresetpage.Sent()).Render(ctx, w); err != nil {
		slog.ErrorContext(ctx, "パスワードリセット送信完了ページのレンダリングに失敗", "error", err)
	}
}
