package sign_in_two_factor_recovery

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/groobb/groobb/go/internal/clientip"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/templates"
	signintwofactorrecoverypage "github.com/groobb/groobb/go/internal/templates/pages/sign_in_two_factor_recovery"
	"github.com/groobb/groobb/go/internal/usecase"
)

// Create POST /sign_in/two_factor/recovery - 送信されたリカバリーコードを保留中
// ユーザーの保存済みコードに対して検証し、成功時にその1回使い切りのコードを消費し、
// セッションを発行し、サインインを完了させ、pending Cookieを消去します。pending Cookieの
// 無いリクエストは検証対象が無いため、サインインへ戻します。バリデーションエラー時 (コードの
// 未入力・不正・未知、または失われたチャレンジ) はメッセージ付きでフォームを再描画します
// (422)。コードはエコーバックします。コードの消費とセッションの発行はUseCase内でアトミックに
// 行われるため、ハンドラーは返したトークンからセッションCookieを設定するだけで済みます。
// CSRF検証は上流のCSRFミドルウェアが強制するため、ここでは繰り返しません。
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// チャレンジ完了後の着地先。パスワードのステップからフォームが運んでくるため、
	// リダイレクト先として使う前にここで検証する。
	returnTo := middleware.SanitizeReturnTo(r.FormValue(templates.ReturnToParam))

	userID, ok := h.sessionMgr.GetTwoFactorPendingUserID(r)
	if !ok {
		http.Redirect(w, r, templates.SignInPath().WithReturnTo(returnTo).String(), http.StatusSeeOther)
		return
	}

	code := r.FormValue("code")

	output, err := h.createSignInTwoFactorRecoveryUC.Execute(ctx, usecase.CreateSignInTwoFactorRecoveryInput{
		UserID:    userID,
		Code:      code,
		IPAddress: clientip.GetClientIP(r, h.cfg.TrustedProxies),
		UserAgent: r.UserAgent(),
	})
	if err != nil {
		var ve *model.ValidationError
		if errors.As(err, &ve) {
			h.renderNew(w, r, http.StatusUnprocessableEntity, signintwofactorrecoverypage.NewPageData{
				CSRFToken:  middleware.CSRFTokenFromContext(ctx),
				Code:       code,
				FormErrors: ve,
				ReturnTo:   returnTo,
			})
			return
		}

		slog.ErrorContext(ctx, "2段階認証リカバリーコードチャレンジの検証に失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// 第2要素が確認でき、リカバリーコードの消費とセッションの発行が一緒に行われた。
	// トークンをセッションCookieに格納し、続いてpending Cookieを破棄して完了した
	// チャレンジが残らないようにする。
	h.sessionMgr.SetSessionCookie(w, output.Token)
	h.sessionMgr.DeleteTwoFactorPendingUserID(w)

	http.Redirect(w, r, templates.AfterSignInPath(returnTo).String(), http.StatusSeeOther)
}
