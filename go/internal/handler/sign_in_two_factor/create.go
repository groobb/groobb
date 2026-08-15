package sign_in_two_factor

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/groobb/groobb/go/internal/clientip"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/templates"
	signintwofactorpage "github.com/groobb/groobb/go/internal/templates/pages/sign_in_two_factor"
	"github.com/groobb/groobb/go/internal/usecase"
)

// Create POST /sign_in/two_factor - verifies the submitted TOTP code against the
// pending user's enabled 2FA setting and, on success, issues the session and
// completes sign-in, clearing the pending cookie. A request without the pending
// cookie has nothing to verify, so it is sent back to sign-in. On a validation error
// (a missing, malformed, or incorrect code, or a gone challenge) the form is
// re-rendered with the message (422); the code is echoed back. The CSRF check is
// enforced upstream by the CSRF middleware, so it is not repeated here.
//
// [Ja] Create POST /sign_in/two_factor - 送信された TOTP コードを保留中ユーザーの有効な 2FA
// 設定に対して検証し、成功時にセッションを発行してサインインを完了させ、pending Cookie を
// 消去します。pending Cookie の無いリクエストは検証対象が無いため、サインインへ戻します。
// バリデーションエラー時 (コードの未入力・不正・不一致、または失われたチャレンジ) は
// メッセージ付きでフォームを再描画します (422)。コードはエコーバックします。CSRF 検証は上流の
// CSRF ミドルウェアが強制するため、ここでは繰り返しません。
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

	if err := h.createSignInTwoFactorUC.Execute(ctx, usecase.CreateSignInTwoFactorInput{
		UserID: userID,
		Code:   code,
	}); err != nil {
		var ve *model.ValidationError
		if errors.As(err, &ve) {
			h.renderNew(w, r, http.StatusUnprocessableEntity, signintwofactorpage.NewPageData{
				CSRFToken:  middleware.CSRFTokenFromContext(ctx),
				Code:       code,
				FormErrors: ve,
				ReturnTo:   returnTo,
			})
			return
		}

		slog.ErrorContext(ctx, "2 段階認証チャレンジの検証に失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// The second factor checked out. Issue the session for the pending user and store
	// its token in the session cookie, then drop the pending cookie so the completed
	// challenge does not linger. A failure here means the code was valid but the
	// session could not be created; surface it as a 500 rather than leaving the user
	// signed out with a spent challenge.
	//
	// [Ja] 第 2 要素が確認できた。保留中ユーザーのセッションを発行しそのトークンをセッション
	// Cookie に格納し、続いて pending Cookie を破棄して完了したチャレンジが残らないようにする。
	// ここでの失敗はコードは正しかったがセッションを作成できなかったことを意味するため、ユーザーを
	// 使い切ったチャレンジとともに未サインインのまま放置せず 500 として表面化する。
	sessionOutput, err := h.createSessionUC.Execute(ctx, usecase.CreateSessionInput{
		UserID:    userID,
		IPAddress: clientip.GetClientIP(r),
		UserAgent: r.UserAgent(),
	})
	if err != nil {
		slog.ErrorContext(ctx, "セッションの作成に失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.sessionMgr.SetSessionCookie(w, sessionOutput.Token)
	h.sessionMgr.DeleteTwoFactorPendingUserID(w)

	http.Redirect(w, r, templates.AfterSignInPath(returnTo).String(), http.StatusSeeOther)
}
