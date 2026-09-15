// passwordパッケージはパスワードリセット更新フローのハンドラーを提供します。
// メールのリンクからの新パスワードフォーム表示 (GET /password/edit) と、新しいパスワードの
// 設定 (PATCH /password) です。リセットリンクの申請は別フロー (handler/password_reset)
// です。
package password

import (
	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/usecase"
)

// Handlerはパスワードリセット更新フローのHTTPハンドラーです。
type Handler struct {
	cfg                   *config.Config
	updatePasswordResetUC *usecase.UpdatePasswordResetUsecase
}

// NewHandlerはpassword Handlerを生成します。
func NewHandler(
	cfg *config.Config,
	updatePasswordResetUC *usecase.UpdatePasswordResetUsecase,
) *Handler {
	return &Handler{
		cfg:                   cfg,
		updatePasswordResetUC: updatePasswordResetUC,
	}
}
