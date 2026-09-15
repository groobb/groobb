// password_resetパッケージはパスワードリセット申請フローのハンドラーを提供します。
// フォーム表示 (GET /password_reset/new) と、リセットリンクを発行するためのemail受付
// (POST /password_reset) です。メールのリンクから新しいパスワードを設定するのは別フロー
// です。
package password_reset

import (
	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/turnstile"
	"github.com/groobb/groobb/go/internal/usecase"
)

// Handlerはパスワードリセット申請フローのHTTPハンドラーです。
type Handler struct {
	cfg                        *config.Config
	createPasswordResetTokenUC *usecase.CreatePasswordResetTokenUsecase
	turnstile                  turnstile.Verifier
}

// NewHandlerはパスワードリセットHandlerを生成します。
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
