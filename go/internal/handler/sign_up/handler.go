// Package sign_up provides the handlers for the sign-up request flow: showing
// the form (GET /sign_up) and accepting an email to issue a confirmation code
// (POST /sign_up).
//
// [Ja] sign_up パッケージはサインアップ申請フローのハンドラーを提供します。フォーム
// 表示 (GET /sign_up) と、確認コードを発行するための email 受付 (POST /sign_up) です。
package sign_up

import (
	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/usecase"
)

// Handler is the HTTP handler for the sign-up request flow.
//
// [Ja] Handler はサインアップ申請フローの HTTP ハンドラーです。
type Handler struct {
	cfg            *config.Config
	sessionMgr     *session.Manager
	createSignUpUC *usecase.CreateSignUpUsecase
}

// NewHandler creates a sign-up Handler.
//
// [Ja] NewHandler はサインアップ Handler を生成します。
func NewHandler(
	cfg *config.Config,
	sessionMgr *session.Manager,
	createSignUpUC *usecase.CreateSignUpUsecase,
) *Handler {
	return &Handler{
		cfg:            cfg,
		sessionMgr:     sessionMgr,
		createSignUpUC: createSignUpUC,
	}
}
