// Package sign_in provides the handlers for signing in: showing the sign-in form
// (GET /sign_in) and authenticating an email and password, then issuing a session
// (POST /sign_in).
//
// [Ja] sign_in パッケージはサインインのハンドラーを提供します。サインインフォームの表示
// (GET /sign_in) と、email とパスワードの認証後にセッションを発行する処理 (POST /sign_in)
// です。
package sign_in

import (
	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/turnstile"
	"github.com/groobb/groobb/go/internal/usecase"
)

// Handler is the HTTP handler for the sign-in flow.
//
// [Ja] Handler はサインインフローの HTTP ハンドラーです。
type Handler struct {
	cfg             *config.Config
	sessionMgr      *session.Manager
	createSignInUC  *usecase.CreateSignInUsecase
	createSessionUC *usecase.CreateSessionUsecase
	turnstile       turnstile.Verifier
}

// NewHandler creates a sign-in Handler.
//
// [Ja] NewHandler はサインイン Handler を生成します。
func NewHandler(
	cfg *config.Config,
	sessionMgr *session.Manager,
	createSignInUC *usecase.CreateSignInUsecase,
	createSessionUC *usecase.CreateSessionUsecase,
	turnstileVerifier turnstile.Verifier,
) *Handler {
	return &Handler{
		cfg:             cfg,
		sessionMgr:      sessionMgr,
		createSignInUC:  createSignInUC,
		createSessionUC: createSessionUC,
		turnstile:       turnstileVerifier,
	}
}
