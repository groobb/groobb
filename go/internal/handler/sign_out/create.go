package sign_out

import (
	"log/slog"
	"net/http"
)

// Create POST /sign_out - signs the user out by deleting the session row for the
// request's token and clearing the session cookie, then redirects to the top
// page. The session row is deleted first so a stolen token cannot resolve to a
// user even after the cookie is cleared; the delete is a no-op when the request
// is not signed in. The CSRF check is enforced upstream by the CSRF middleware.
//
// [Ja] Create POST /sign_out - リクエストのトークンのセッション行を削除しセッション
// Cookie を消去してユーザーをサインアウトさせ、トップページへリダイレクトします。
// セッション行を先に削除するため、Cookie を消した後でも盗まれたトークンがユーザーに
// 解決しません。未サインインのリクエストでは削除は何もしません。CSRF 検証は上流の
// CSRF ミドルウェアが強制します。
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := h.deleteSessionUC.Execute(ctx, h.sessionMgr.SessionToken(r)); err != nil {
		slog.ErrorContext(ctx, "サインアウトに失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.sessionMgr.DeleteSessionCookie(w)

	http.Redirect(w, r, "/", http.StatusSeeOther)
}
