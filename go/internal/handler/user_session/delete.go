package user_session

import (
	"log/slog"
	"net/http"

	"github.com/groobb/groobb/go/internal/i18n"
)

// Delete DELETE /user_session - signs the user out by deleting the session row
// for the request's token and clearing the session cookie, then redirects to the
// top page. The session row is deleted first so a stolen token cannot resolve to
// a user even after the cookie is cleared; the delete is a no-op when the request
// is not signed in. The CSRF check is enforced upstream by the CSRF middleware.
//
// [Ja] Delete DELETE /user_session - リクエストのトークンのセッション行を削除し
// セッション Cookie を消去してユーザーをサインアウトさせ、トップページへリダイレクト
// します。セッション行を先に削除するため、Cookie を消した後でも盗まれたトークンが
// ユーザーに解決しません。未サインインのリクエストでは削除は何もしません。CSRF 検証は
// 上流の CSRF ミドルウェアが強制します。
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := h.deleteSessionUC.Execute(ctx, h.sessionMgr.SessionToken(r)); err != nil {
		slog.ErrorContext(ctx, "サインアウトに失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.sessionMgr.DeleteSessionCookie(w)

	// Set a one-off success flash before redirecting so the top page renders the
	// "signed out" toast. The flash cookie is read and cleared by the flash
	// middleware on the redirected request.
	//
	// [Ja] リダイレクト前に一度きりの成功フラッシュを設定し、トップページで「サインアウト
	// しました」の toast を描画させる。フラッシュ Cookie はリダイレクト先のリクエストで
	// フラッシュミドルウェアが読み取って消去する。
	h.flashMgr.SetSuccess(w, i18n.T(ctx, "flash_sign_out_success"))

	http.Redirect(w, r, "/", http.StatusSeeOther)
}
