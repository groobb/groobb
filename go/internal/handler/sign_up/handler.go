// sign_upパッケージはサインアップ申請フローのハンドラーを提供します。フォーム
// 表示 (GET /sign_up) と、確認コードを発行するためのemail受付 (POST /sign_up) です。
package sign_up

import (
	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/turnstile"
	"github.com/groobb/groobb/go/internal/usecase"
)

// Handlerはサインアップ申請フローのHTTPハンドラーです。
type Handler struct {
	cfg            *config.Config
	sessionMgr     *session.Manager
	createSignUpUC *usecase.CreateSignUpUsecase
	turnstile      turnstile.Verifier
}

// NewHandlerはサインアップHandlerを生成します。
func NewHandler(
	cfg *config.Config,
	sessionMgr *session.Manager,
	createSignUpUC *usecase.CreateSignUpUsecase,
	turnstileVerifier turnstile.Verifier,
) *Handler {
	return &Handler{
		cfg:            cfg,
		sessionMgr:     sessionMgr,
		createSignUpUC: createSignUpUC,
		turnstile:      turnstileVerifier,
	}
}
