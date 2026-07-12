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

// Delete DELETE /settings/withdrawal - re-checks the current password and, on
// success, withdraws the account (soft-deletes and anonymizes the user and deletes
// all of its sessions), then clears this browser's session cookie and redirects to
// the top page with a completion flash. The route is reached from the HTML form via
// the _method=DELETE override. It is registered behind RequireAuth, so the user
// from the context is non-nil. On a validation error (a missing or wrong current
// password) the confirmation form is re-rendered with the messages (422). The CSRF
// check is enforced upstream by the CSRF middleware, so it is not repeated here.
//
// [Ja] Delete DELETE /settings/withdrawal - 現在のパスワードを再確認し、成功時に
// アカウントを退会させ (ユーザーを論理削除・匿名化し、その全セッションを削除する)、この
// ブラウザのセッション Cookie を消去して完了フラッシュ付きでトップページへリダイレクト
// します。ルートは HTML フォームから _method=DELETE のオーバーライドで到達します。
// RequireAuth の背後に登録されるため、context のユーザーは非 nil です。バリデーション
// エラー時 (現在のパスワードの未入力・誤り) は確認フォームをメッセージ付きで再描画します
// (422)。CSRF 検証は上流の CSRF ミドルウェアが強制するため、ここでは繰り返しません。
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
			// The current password is deliberately not echoed back, since re-rendering
			// a password field with its value is a credential-leak risk; the user
			// re-enters it.
			//
			// [Ja] 現在のパスワードは意図的にエコーバックしない。値付きでパスワード
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

	// Withdrawal succeeded. The UseCase already deleted every session row for the
	// user (all devices), so this browser's now-orphaned session cookie is cleared
	// too, mirroring sign-out. Then set a completion flash and send the now-signed-
	// out user to the top page, which renders it.
	//
	// [Ja] 退会に成功。UseCase は既にそのユーザーの全セッション行を削除している (全端末) ため、
	// このブラウザに残った孤児のセッション Cookie も消去する (サインアウトと同様)。続いて完了
	// フラッシュを設定し、サインアウト済みとなったユーザーをそれを描画するトップページへ送る。
	h.sessionMgr.DeleteSessionCookie(w)
	h.flashMgr.SetSuccess(w, i18n.T(ctx, "flash_account_withdrawn"))
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
