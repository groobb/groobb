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

// Create POST /settings/email/confirmation - 送信されたコードをサインイン済み
// ユーザーの保留中のメール変更の確認に対して検証し、成功時に新しいアドレスを適用して
// フラッシュ付きで設定へリダイレクトします。RequireAuthの背後に登録されるため、contextの
// ユーザーは非nilです。バリデーションエラー時 (コードの形式不正・不一致・期限切れ、または
// 稀なアドレス取得の競合) はメッセージ付きでフォームを再描画します (422)。CSRF検証は上流の
// CSRFミドルウェアが強制するため、ここでは繰り返しません。
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

	// emailが変更された。成功フラッシュを設定し、それを描画する設定ハブへユーザーを
	// 送る。settingsルートは後続フェーズで追加されるため、それまでこのリダイレクト先は
	// まだ存在しないルートを指すが、変更フォームのUIもそのフェーズまで繋がないため、
	// この行き止まりに到達するユーザーはいない。
	h.flashMgr.SetSuccess(w, i18n.T(ctx, "flash_email_changed"))
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}
