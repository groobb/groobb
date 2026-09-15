// email_confirmationパッケージはサインアップのコード入力ステップのハンドラーを
// 提供します。フォーム表示 (GET /email_confirmation/new) と、ユーザーが入力し返した
// コードの検証 (POST /email_confirmation) です。
package email_confirmation

import (
	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/usecase"
)

// Handlerはメール確認コード入力フローのHTTPハンドラーです。
type Handler struct {
	cfg                       *config.Config
	sessionMgr                *session.Manager
	verifyEmailConfirmationUC *usecase.VerifyEmailConfirmationUsecase
}

// NewHandlerはメール確認Handlerを生成します。
func NewHandler(
	cfg *config.Config,
	sessionMgr *session.Manager,
	verifyEmailConfirmationUC *usecase.VerifyEmailConfirmationUsecase,
) *Handler {
	return &Handler{
		cfg:                       cfg,
		sessionMgr:                sessionMgr,
		verifyEmailConfirmationUC: verifyEmailConfirmationUC,
	}
}
