package settings_withdrawal

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/model"
	settingswithdrawalpage "github.com/groobb/groobb/go/internal/templates/pages/settings_withdrawal"
	"github.com/groobb/groobb/go/internal/usecase"
)

// Delete DELETE /settings/withdrawal - 現在のパスワードを再確認し、成功時に
// アカウントを退会させ (ユーザーを論理削除・匿名化し、その全セッションを削除する)、この
// ブラウザのセッションCookieを消去して完了フラッシュ付きでトップページへリダイレクト
// します。ルートはHTMLフォームから _method=DELETEのオーバーライドで到達します。
// RequireAuthの背後に登録されるため、contextのユーザーは非nilです。バリデーション
// エラー時 (現在のパスワードの未入力・誤り) は確認フォームをメッセージ付きで再描画します
// (422)。CSRF検証は上流のCSRFミドルウェアが強制するため、ここでは繰り返しません。
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)

	currentPassword := r.FormValue("current_password")

	if err := h.deleteAccountUC.Execute(ctx, usecase.DeleteAccountInput{
		UserID:          user.ID,
		CurrentPassword: currentPassword,
	}); err != nil {
		var ve *model.ValidationError
		if errors.As(err, &ve) {
			// 現在のパスワードは意図的にエコーバックしない。値付きでパスワード
			// フィールドを再描画するのは資格情報の漏えいリスクのためで、ユーザーに再入力させる。
			h.renderNew(w, r, http.StatusUnprocessableEntity, settingswithdrawalpage.NewPageData{
				CSRFToken:  middleware.CSRFTokenFromContext(ctx),
				FormErrors: ve,
			})
			return
		}

		slog.ErrorContext(ctx, "退会に失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// 退会に成功。UseCaseは既にそのユーザーの全セッション行を削除している (全端末) ため、
	// このブラウザに残った孤児のセッションCookieも消去する (サインアウトと同様)。続いて完了
	// フラッシュを設定し、サインアウト済みとなったユーザーをそれを描画するトップページへ送る。
	h.sessionMgr.DeleteSessionCookie(w)
	h.flashMgr.SetSuccess(w, i18n.T(ctx, "flash_account_withdrawn"))
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
