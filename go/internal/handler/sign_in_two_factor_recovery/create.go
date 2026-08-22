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

// Create POST /sign_in/two_factor/recovery - verifies the submitted recovery code
// against the pending user's stored codes and, on success, consumes that one-time
// code, issues the session, and completes sign-in, clearing the pending cookie. A
// request without the pending cookie has nothing to verify, so it is sent back to
// sign-in. On a validation error (a missing, malformed, or unknown code, or a gone
// challenge) the form is re-rendered with the message (422); the code is echoed
// back. Consuming the code and issuing the session happen atomically in the
// UseCase, so the handler only needs to set the session cookie from the returned
// token. The CSRF check is enforced upstream by the CSRF middleware, so it is not
// repeated here.
//
// [Ja] Create POST /sign_in/two_factor/recovery - 送信されたリカバリーコードを保留中
// ユーザーの保存済みコードに対して検証し、成功時にその 1 回使い切りのコードを消費し、
// セッションを発行し、サインインを完了させ、pending Cookie を消去します。pending Cookie の
// 無いリクエストは検証対象が無いため、サインインへ戻します。バリデーションエラー時 (コードの
// 未入力・不正・未知、または失われたチャレンジ) はメッセージ付きでフォームを再描画します
// (422)。コードはエコーバックします。コードの消費とセッションの発行は UseCase 内でアトミックに
// 行われるため、ハンドラーは返したトークンからセッション Cookie を設定するだけで済みます。
// CSRF 検証は上流の CSRF ミドルウェアが強制するため、ここでは繰り返しません。
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Where to land once the challenge completes. The form carries it from the
	// password step, so it is validated here before it is used as a redirect target.
	//
	// [Ja] チャレンジ完了後の着地先。パスワードのステップからフォームが運んでくるため、
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

		slog.ErrorContext(ctx, "2 段階認証リカバリーコードチャレンジの検証に失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// The second factor checked out and the recovery code was consumed together with
	// issuing the session. Store the token in the session cookie, then drop the
	// pending cookie so the completed challenge does not linger.
	//
	// [Ja] 第 2 要素が確認でき、リカバリーコードの消費とセッションの発行が一緒に行われた。
	// トークンをセッション Cookie に格納し、続いて pending Cookie を破棄して完了した
	// チャレンジが残らないようにする。
	h.sessionMgr.SetSessionCookie(w, output.Token)
	h.sessionMgr.DeleteTwoFactorPendingUserID(w)

	http.Redirect(w, r, templates.AfterSignInPath(returnTo).String(), http.StatusSeeOther)
}
