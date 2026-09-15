package user_session

import (
	"log/slog"
	"net/http"

	"github.com/groobb/groobb/go/internal/i18n"
)

// Delete DELETE /user_session - リクエストのトークンのセッション行を削除し
// セッションCookieを消去してユーザーをサインアウトさせ、トップページへリダイレクト
// します。セッション行を先に削除するため、Cookieを消した後でも盗まれたトークンが
// ユーザーに解決しません。未サインインのリクエストでは削除は何もしません。CSRF検証は
// 上流のCSRFミドルウェアが強制します。
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := h.deleteSessionUC.Execute(ctx, h.sessionMgr.SessionToken(r)); err != nil {
		slog.ErrorContext(ctx, "サインアウトに失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.sessionMgr.DeleteSessionCookie(w)

	// リダイレクト前に一度きりの成功フラッシュを設定し、トップページで「サインアウト
	// しました」のtoastを描画させる。フラッシュCookieはリダイレクト先のリクエストで
	// フラッシュミドルウェアが読み取って消去する。
	h.flashMgr.SetSuccess(w, i18n.T(ctx, "flash_sign_out_success"))

	http.Redirect(w, r, "/", http.StatusSeeOther)
}
