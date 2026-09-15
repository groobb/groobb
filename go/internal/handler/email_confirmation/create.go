package email_confirmation

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/model"
	emailconfirmationpage "github.com/groobb/groobb/go/internal/templates/pages/email_confirmation"
	"github.com/groobb/groobb/go/internal/usecase"
)

// Create POST /email_confirmation - 送信されたコードを保留中の確認に対して検証し、
// 成功時にユーザーをアカウント作成へ進めます。受け渡しCookieの無いリクエストは検証
// 対象が無いため、サインアップへ戻します。バリデーションエラー時はメッセージ付きで
// フォームを再描画します (422)。CSRF検証は上流のCSRFミドルウェアが強制するため、
// ここでは繰り返しません。
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, ok := h.sessionMgr.GetEmailConfirmationID(r)
	if !ok {
		http.Redirect(w, r, "/sign_up", http.StatusSeeOther)
		return
	}

	code := r.FormValue("code")

	_, err := h.verifyEmailConfirmationUC.Execute(ctx, usecase.VerifyEmailConfirmationInput{
		ID:   id,
		Code: code,
	})
	if err != nil {
		var ve *model.ValidationError
		if errors.As(err, &ve) {
			h.renderNew(w, r, http.StatusUnprocessableEntity, emailconfirmationpage.NewPageData{
				CSRFToken:  middleware.CSRFTokenFromContext(ctx),
				Code:       code,
				FormErrors: ve,
			})
			return
		}

		slog.ErrorContext(ctx, "メール確認の検証に失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// emailが検証された。次のステップが成功済みの確認を見つけられるよう受け渡し
	// Cookieはそのまま残し、パスワード設定とアカウント作成へユーザーを送る。account
	// ルートはアカウント作成タスクで追加されるため、それまでこのリダイレクト先は
	// まだ存在しないルートを指す。
	http.Redirect(w, r, "/account/new", http.StatusSeeOther)
}
