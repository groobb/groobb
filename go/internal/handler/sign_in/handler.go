// sign_inパッケージはサインインのハンドラーを提供します。サインインフォームの表示
// (GET /sign_in) と、emailとパスワードの認証後にセッションを発行する処理 (POST /sign_in)
// です。
package sign_in

import (
	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/turnstile"
	"github.com/groobb/groobb/go/internal/usecase"
)

// HandlerはサインインフローのHTTPハンドラーです。
type Handler struct {
	cfg             *config.Config
	sessionMgr      *session.Manager
	createSignInUC  *usecase.CreateSignInUsecase
	createSessionUC *usecase.CreateSessionUsecase
	turnstile       turnstile.Verifier
}

// NewHandlerはサインインHandlerを生成します。
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
