// Package email_confirmation provides the handlers for the code-entry step of
// sign-up: showing the form (GET /email_confirmation/new) and verifying the
// code the user types back (POST /email_confirmation).
//
// [Ja] email_confirmation パッケージはサインアップのコード入力ステップのハンドラーを
// 提供します。フォーム表示 (GET /email_confirmation/new) と、ユーザーが入力し返した
// コードの検証 (POST /email_confirmation) です。
package email_confirmation

import (
	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/usecase"
)

// Handler is the HTTP handler for the email-confirmation code-entry flow.
//
// [Ja] Handler はメール確認コード入力フローの HTTP ハンドラーです。
type Handler struct {
	cfg                       *config.Config
	sessionMgr                *session.Manager
	verifyEmailConfirmationUC *usecase.VerifyEmailConfirmationUsecase
}

// NewHandler creates an email-confirmation Handler.
//
// [Ja] NewHandler はメール確認 Handler を生成します。
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
