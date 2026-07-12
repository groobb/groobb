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

// Create POST /sign_in - authenticates the submitted email and password and, on
// success, issues a session and signs the user in. On a validation error
// (including a failed credential check) the form is re-rendered with the messages
// (422); the email is echoed back but the password is not. The CSRF check is
// enforced upstream by the CSRF middleware, so it is not repeated here.
//
// [Ja] Create POST /sign_in - 送信された email とパスワードを認証し、成功時はセッションを
// 発行してユーザーをサインインさせます。バリデーションエラー (資格情報チェックの失敗を
// 含む) のときはメッセージ付きでフォームを再描画します (422)。email はエコーバックします
// が、パスワードはしません。CSRF 検証は上流の CSRF ミドルウェアが強制するため、ここでは
// 繰り返しません。
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	email := r.FormValue("email")
	password := r.FormValue("password")

	// Verify the Turnstile token before the UseCase, as a request-level bot gate:
	// a non-pass (an unsolved / bot-forged widget) and a siteverify failure both
	// stop the request here so it never reaches the UseCase (no credential check
	// runs and no session is issued), which is what blunts credential stuffing.
	// Both are logged at warn — Groobb has no error tracker, so warn is the ceiling
	// — and surfaced to the user as a form-wide message, 422. The disabled dev /
	// test setup (empty secret key) passes verification, so this gate is
	// transparent there.
	//
	// [Ja] UseCase の前に Turnstile トークンを検証する。リクエストレベルの Bot ゲートとして、
	// 非通過 (未解決 / Bot による偽造ウィジェット) と siteverify の失敗のどちらもここで
	// リクエストを止め、UseCase へは到達させない (資格情報チェックもセッション発行も走らない)。
	// これがクレデンシャルスタッフィングを鈍らせる。どちらも warn でログする (Groobb は
	// エラートラッカー未連携のため warn が上限)。ユーザーにはフォーム全体のメッセージとして
	// 422 で表示する。無効化された dev / test 構成 (シークレットキー空) は検証を通過するため、
	// そこではこのゲートは透過的に働く。
	if passed, err := h.turnstile.Verify(ctx, r.FormValue("cf-turnstile-response")); err != nil || !passed {
		// A plain non-pass (passed == false, err == nil) is an expected bot
		// rejection, not a system error, so attach the error attribute only when
		// there actually is one — otherwise the log carries an empty error=<nil>.
		//
		// [Ja] 単なる非通過 (passed == false, err == nil) は想定内の Bot 拒否であり
		// システムエラーではないため、error 属性は実際にエラーがあるときだけ付ける。
		// そうしないとログに空の error=<nil> が残る。
		attrs := []any{}
		if err != nil {
			attrs = append(attrs, "error", err)
		}
		slog.WarnContext(ctx, "Turnstile 検証を通過しなかったためサインインを受け付けない", attrs...)
		formErrors := model.NewValidationError()
		formErrors.AddGlobal(i18n.T(ctx, "validation_turnstile_failed"))
		h.renderNew(w, r, http.StatusUnprocessableEntity, signinpage.NewPageData{
			CSRFToken:  middleware.CSRFTokenFromContext(ctx),
			Email:      email,
			FormErrors: formErrors,
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
			h.renderNew(w, r, http.StatusUnprocessableEntity, signinpage.NewPageData{
				CSRFToken:  middleware.CSRFTokenFromContext(ctx),
				Email:      email,
				FormErrors: ve,
			})
			return
		}

		slog.ErrorContext(ctx, "サインインに失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// A 2FA-enabled account does not sign in from the password alone. Hold the
	// authenticated user in a short-lived pending cookie (no session issued yet)
	// and send them to the TOTP challenge, which completes the sign-in on a valid
	// code. Accounts without 2FA fall through and get a session immediately.
	//
	// [Ja] 2FA 有効なアカウントはパスワードだけではサインインしない。認証済みユーザーを
	// 短命の pending Cookie に保持し (この時点ではセッションを発行しない)、TOTP チャレンジへ
	// 送る。チャレンジが正しいコードでサインインを完了させる。2FA 無しのアカウントはここを
	// 素通りして即座にセッションを得る。
	if signInOutput.UserTwoFactorAuth != nil {
		h.sessionMgr.SetTwoFactorPendingUserID(w, signInOutput.User.ID)
		http.Redirect(w, r, templates.SignInTwoFactorNewPath().String(), http.StatusSeeOther)
		return
	}

	// Issue the session and store its token in the session cookie. A failure here
	// means the credentials were valid but the session could not be created;
	// surface it as a 500 rather than silently leaving the user signed out.
	//
	// [Ja] セッションを発行しそのトークンをセッション Cookie に格納する。ここでの失敗は
	// 資格情報は正しかったがセッションを作成できなかったことを意味するため、ユーザーを黙って
	// 未サインインのまま放置せず 500 として表面化する。
	sessionOutput, err := h.createSessionUC.Execute(ctx, usecase.CreateSessionInput{
		UserID:    signInOutput.User.ID,
		IPAddress: clientip.GetClientIP(r),
		UserAgent: r.UserAgent(),
	})
	if err != nil {
		slog.ErrorContext(ctx, "セッションの作成に失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.sessionMgr.SetSessionCookie(w, sessionOutput.Token)

	http.Redirect(w, r, "/", http.StatusSeeOther)
}
