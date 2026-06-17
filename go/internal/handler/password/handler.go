// Package password provides the handlers for the password reset update flow:
// showing the new-password form from the emailed link (GET /password/edit) and
// setting the new password (PATCH /password). Requesting the reset link is a
// separate flow (handler/password_reset).
//
// [Ja] password パッケージはパスワードリセット更新フローのハンドラーを提供します。
// メールのリンクからの新パスワードフォーム表示 (GET /password/edit) と、新しいパスワードの
// 設定 (PATCH /password) です。リセットリンクの申請は別フロー (handler/password_reset)
// です。
package password

import (
	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/usecase"
)

// Handler is the HTTP handler for the password reset update flow.
//
// [Ja] Handler はパスワードリセット更新フローの HTTP ハンドラーです。
type Handler struct {
	cfg                   *config.Config
	updatePasswordResetUC *usecase.UpdatePasswordResetUsecase
}

// NewHandler creates a password Handler.
//
// [Ja] NewHandler は password Handler を生成します。
func NewHandler(
	cfg *config.Config,
	updatePasswordResetUC *usecase.UpdatePasswordResetUsecase,
) *Handler {
	return &Handler{
		cfg:                   cfg,
		updatePasswordResetUC: updatePasswordResetUC,
	}
}
