package sign_in

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/groobb/groobb/go/internal/clientip"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/model"
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
