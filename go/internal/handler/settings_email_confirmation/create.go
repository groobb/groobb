package settings_email_confirmation

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/model"
	settingsemailconfirmationpage "github.com/groobb/groobb/go/internal/templates/pages/settings_email_confirmation"
	"github.com/groobb/groobb/go/internal/usecase"
)

// Create POST /settings/email/confirmation - verifies the submitted code against
// the signed-in user's pending email-change confirmation and, on success, applies
// the new address and redirects to settings with a success flash. It is registered
// behind RequireAuth, so the user from the context is non-nil. On a validation
// error (a malformed, wrong, or expired code, or the rare address-taken race) the
// form is re-rendered with the messages (422); the CSRF check is enforced upstream
// by the CSRF middleware, so it is not repeated here.
//
// [Ja] Create POST /settings/email/confirmation - 送信されたコードをサインイン済み
// ユーザーの保留中のメール変更の確認に対して検証し、成功時に新しいアドレスを適用して
// フラッシュ付きで設定へリダイレクトします。RequireAuth の背後に登録されるため、context の
// ユーザーは非 nil です。バリデーションエラー時 (コードの形式不正・不一致・期限切れ、または
// 稀なアドレス取得の競合) はメッセージ付きでフォームを再描画します (422)。CSRF 検証は上流の
// CSRF ミドルウェアが強制するため、ここでは繰り返しません。
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)

	code := r.FormValue("code")

	_, err := h.verifyEmailChangeUC.Execute(ctx, usecase.VerifyEmailChangeInput{
		UserID: user.ID,
		Code:   code,
	})
	if err != nil {
		var ve *model.ValidationError
		if errors.As(err, &ve) {
			h.renderNew(w, r, http.StatusUnprocessableEntity, settingsemailconfirmationpage.NewPageData{
				CSRFToken:  middleware.CSRFTokenFromContext(ctx),
				Code:       code,
				FormErrors: ve,
			})
			return
		}

		slog.ErrorContext(ctx, "メールアドレス変更の確認に失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// The email is changed. Set a success flash and send the user to the settings
	// hub, which renders it. The settings route arrives with a later phase; until
	// then this redirect points at a route that does not yet exist, but the
	// change-form UI is not linked until that phase either, so no user reaches this
	// dead end.
	//
	// [Ja] email が変更された。成功フラッシュを設定し、それを描画する設定ハブへユーザーを
	// 送る。settings ルートは後続フェーズで追加されるため、それまでこのリダイレクト先は
	// まだ存在しないルートを指すが、変更フォームの UI もそのフェーズまで繋がないため、
	// この行き止まりに到達するユーザーはいない。
	h.flashMgr.SetSuccess(w, i18n.T(ctx, "flash_email_changed"))
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}
