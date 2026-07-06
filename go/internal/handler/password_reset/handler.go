// Package password_reset provides the handlers for the password reset request
// flow: showing the form (GET /password_reset/new) and accepting an email to
// issue a reset link (POST /password_reset). Setting the new password from the
// emailed link is a separate flow.
//
// [Ja] password_reset パッケージはパスワードリセット申請フローのハンドラーを提供します。
// フォーム表示 (GET /password_reset/new) と、リセットリンクを発行するための email 受付
// (POST /password_reset) です。メールのリンクから新しいパスワードを設定するのは別フロー
// です。
package password_reset

import (
	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/turnstile"
	"github.com/groobb/groobb/go/internal/usecase"
)

// Handler is the HTTP handler for the password reset request flow.
//
// [Ja] Handler はパスワードリセット申請フローの HTTP ハンドラーです。
type Handler struct {
	cfg                        *config.Config
	createPasswordResetTokenUC *usecase.CreatePasswordResetTokenUsecase
	turnstile                  turnstile.Verifier
}

// NewHandler creates a password-reset Handler.
//
// [Ja] NewHandler はパスワードリセット Handler を生成します。
func NewHandler(
	cfg *config.Config,
	createPasswordResetTokenUC *usecase.CreatePasswordResetTokenUsecase,
	turnstileVerifier turnstile.Verifier,
) *Handler {
	return &Handler{
		cfg:                        cfg,
		createPasswordResetTokenUC: createPasswordResetTokenUC,
		turnstile:                  turnstileVerifier,
	}
}
