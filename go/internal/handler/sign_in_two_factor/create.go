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

// Create POST /sign_in/two_factor - 送信されたTOTPコードを保留中ユーザーの有効な2FA
// 設定に対して検証し、成功時にセッションを発行してサインインを完了させ、pending Cookieを
// 消去します。pending Cookieの無いリクエストは検証対象が無いため、サインインへ戻します。
// バリデーションエラー時 (コードの未入力・不正・不一致、または失われたチャレンジ) は
// メッセージ付きでフォームを再描画します (422)。コードはエコーバックします。CSRF検証は上流の
// CSRFミドルウェアが強制するため、ここでは繰り返しません。
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

		slog.ErrorContext(ctx, "2段階認証チャレンジの検証に失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// 第2要素が確認できた。保留中ユーザーのセッションを発行しそのトークンをセッション
	// Cookieに格納し、続いてpending Cookieを破棄して完了したチャレンジが残らないようにする。
	// ここでの失敗はコードは正しかったがセッションを作成できなかったことを意味するため、ユーザーを
	// 使い切ったチャレンジとともに未サインインのまま放置せず500として表面化する。
	sessionOutput, err := h.createSessionUC.Execute(ctx, usecase.CreateSessionInput{
		UserID:    userID,
		IPAddress: clientip.GetClientIP(r, h.cfg.TrustedProxies),
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
