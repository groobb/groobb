package password

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/model"
	passwordpage "github.com/groobb/groobb/go/internal/templates/pages/password"
	"github.com/groobb/groobb/go/internal/usecase"
)

// Update PATCH /password - sets the account's new password from the reset token
// and the submitted password, spending the token. The route is reached from the
// HTML form via the _method=PATCH override. On a validation error the form is
// re-rendered with the messages (422); a token error (invalid / used / expired)
// is form-wide, so the token is cleared from the re-rendered form to stop the
// dead link being re-submitted, sending the user to request a fresh link instead.
// On success the user is redirected to sign in with the new password. The CSRF
// check is enforced upstream by the CSRF middleware, so it is not repeated here.
//
// [Ja] Update PATCH /password - リセットトークンと送信されたパスワードからアカウントの
// 新しいパスワードを設定し、トークンを消費します。ルートは HTML フォームから
// _method=PATCH のオーバーライドで到達します。バリデーションエラー時はメッセージ付きで
// フォームを再描画します (422)。トークンエラー (無効 / 使用済み / 期限切れ) はフォーム
// 全体のため、再描画フォームからトークンを消去して失効リンクの再送信を止め、ユーザーを
// 新しいリンクの申請へ向かわせます。成功時はユーザーを新しいパスワードでのサインインへ
// リダイレクトします。CSRF 検証は上流の CSRF ミドルウェアが強制するため、ここでは
// 繰り返しません。
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	token := r.FormValue("token")
	password := r.FormValue("password")
	passwordConfirmation := r.FormValue("password_confirmation")

	err := h.updatePasswordResetUC.Execute(ctx, usecase.UpdatePasswordResetInput{
		Token:                token,
		Password:             password,
		PasswordConfirmation: passwordConfirmation,
	})
	if err != nil {
		var ve *model.ValidationError
		if errors.As(err, &ve) {
			// A form-wide error means the link itself is unusable (invalid, used, or
			// expired). Drop the token from the re-rendered form so submitting again
			// cannot replay the dead link; the user follows the "request a new link"
			// link instead. Field-only errors (password problems) keep the token so a
			// corrected password can still be submitted against the same valid link.
			//
			// [Ja] フォーム全体のエラーはリンク自体が使えない (無効・使用済み・期限切れ) こと
			// を意味する。再描画フォームからトークンを落とし、再送信で失効リンクを再生できない
			// ようにする。ユーザーは代わりに「新しいリンクを申請」のリンクをたどる。フィールド
			// だけのエラー (パスワードの問題) はトークンを保ち、同じ有効なリンクに対して修正した
			// パスワードを送信できるようにする。
			editToken := token
			if ve.HasGlobalError() {
				editToken = ""
			}
			h.renderEdit(w, r, http.StatusUnprocessableEntity, passwordpage.EditPageData{
				CSRFToken:  middleware.CSRFTokenFromContext(ctx),
				Token:      editToken,
				FormErrors: ve,
			})
			return
		}

		slog.ErrorContext(ctx, "パスワードの更新に失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// The password is reset but the user is signed out (a reset happens while
	// signed out), so send them to sign in with the new password rather than
	// issuing a session here.
	//
	// [Ja] パスワードはリセットされたがユーザーはサインアウト中 (リセットはサインアウト中に
	// 行われる) のため、ここでセッションを発行せず、新しいパスワードでのサインインへ送る。
	http.Redirect(w, r, "/sign_in", http.StatusSeeOther)
}
