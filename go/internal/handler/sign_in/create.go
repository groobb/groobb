package sign_in

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/groobb/groobb/go/internal/clientip"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/templates"
	signinpage "github.com/groobb/groobb/go/internal/templates/pages/sign_in"
	"github.com/groobb/groobb/go/internal/usecase"
)

// Create POST /sign_in - 送信されたemailとパスワードを認証し、成功時はセッションを
// 発行してユーザーをサインインさせます。バリデーションエラー (資格情報チェックの失敗を
// 含む) のときはメッセージ付きでフォームを再描画します (422)。emailはエコーバックします
// が、パスワードはしません。CSRF検証は上流のCSRFミドルウェアが強制するため、ここでは
// 繰り返しません。
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	email := r.FormValue("email")
	password := r.FormValue("password")

	// セッション発行後の着地先。訪問者が追い返された保護ルートからフォームが運んで
	// くるため、リダイレクト先として使う前、そしてフローの次のステップへ渡す前にここで
	// 検証する。
	returnTo := middleware.SanitizeReturnTo(r.FormValue(templates.ReturnToParam))

	// UseCaseの前にTurnstileトークンを検証する。リクエストレベルのBotゲートとして、
	// 非通過 (未解決 / Botによる偽造ウィジェット) とsiteverifyの失敗のどちらもここで
	// リクエストを止め、UseCaseへは到達させない (資格情報チェックもセッション発行も走らない)。
	// これがクレデンシャルスタッフィングを鈍らせる。どちらもwarnでログする (Groobbは
	// エラートラッカー未連携のためwarnが上限)。ユーザーにはフォーム全体のメッセージとして
	// 422で表示する。無効化されたdev / test構成 (シークレットキー空) は検証を通過するため、
	// そこではこのゲートは透過的に働く。
	if passed, err := h.turnstile.Verify(ctx, r.FormValue("cf-turnstile-response")); err != nil || !passed {
		// 単なる非通過 (passed == false, err == nil) は想定内のBot拒否であり
		// システムエラーではないため、error属性は実際にエラーがあるときだけ付ける。
		// そうしないとログに空のerror=<nil> が残る。
		attrs := []any{}
		if err != nil {
			attrs = append(attrs, "error", err)
		}
		slog.WarnContext(ctx, "Turnstile検証を通過しなかったためサインインを受け付けない", attrs...)
		formErrors := model.NewValidationError()
		formErrors.AddGlobal(i18n.T(ctx, "validation_turnstile_failed"))
		h.renderNew(w, r, http.StatusUnprocessableEntity, false, signinpage.NewPageData{
			CSRFToken:  middleware.CSRFTokenFromContext(ctx),
			Email:      email,
			FormErrors: formErrors,
			ReturnTo:   returnTo,
		})
		return
	}

	signInOutput, err := h.createSignInUC.Execute(ctx, usecase.CreateSignInInput{
		Email:    email,
		Password: password,
	})
	if err != nil {
		var ve *model.ValidationError
		if errors.As(err, &ve) {
			h.renderNew(w, r, http.StatusUnprocessableEntity, false, signinpage.NewPageData{
				CSRFToken:  middleware.CSRFTokenFromContext(ctx),
				Email:      email,
				FormErrors: ve,
				ReturnTo:   returnTo,
			})
			return
		}

		slog.ErrorContext(ctx, "サインインに失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// 2FA有効なアカウントはパスワードだけではサインインしない。認証済みユーザーを
	// 短命のpending Cookieに保持し (この時点ではセッションを発行しない)、TOTPチャレンジへ
	// 送る。チャレンジが正しいコードでサインインを完了させる。2FA無しのアカウントはここを
	// 素通りして即座にセッションを得る。
	if signInOutput.UserTwoFactorAuth != nil {
		h.sessionMgr.SetTwoFactorPendingUserID(w, signInOutput.User.ID)
		http.Redirect(w, r, templates.SignInTwoFactorNewPath().WithReturnTo(returnTo).String(), http.StatusSeeOther)
		return
	}

	// セッションを発行しそのトークンをセッションCookieに格納する。ここでの失敗は
	// 資格情報は正しかったがセッションを作成できなかったことを意味するため、ユーザーを黙って
	// 未サインインのまま放置せず500として表面化する。
	sessionOutput, err := h.createSessionUC.Execute(ctx, usecase.CreateSessionInput{
		UserID:    signInOutput.User.ID,
		IPAddress: clientip.GetClientIP(r, h.cfg.TrustedProxies),
		UserAgent: r.UserAgent(),
	})
	if err != nil {
		slog.ErrorContext(ctx, "セッションの作成に失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.sessionMgr.SetSessionCookie(w, sessionOutput.Token)

	http.Redirect(w, r, templates.AfterSignInPath(returnTo).String(), http.StatusSeeOther)
}
