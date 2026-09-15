// settings_withdrawalパッケージは退会フローのハンドラーを提供します。確認フォームの
// 表示 (GET /settings/withdrawal/new) と、退会の実行 (DELETE /settings/withdrawal) です。
// 退会の実行はアカウントを論理削除・匿名化し、その全セッションを削除します。どちらのルートも
// RequireAuthの背後にあり、退会対象はセッションから解決したサインイン済みユーザーです。
package settings_withdrawal

import (
	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/usecase"
)

// Handlerは退会フローのHTTPハンドラーです。
type Handler struct {
	cfg             *config.Config
	sessionMgr      *session.Manager
	flashMgr        *session.FlashManager
	deleteAccountUC *usecase.DeleteAccountUsecase
}

// NewHandlerはsettings_withdrawal Handlerを生成します。
func NewHandler(
	cfg *config.Config,
	sessionMgr *session.Manager,
	flashMgr *session.FlashManager,
	deleteAccountUC *usecase.DeleteAccountUsecase,
) *Handler {
	return &Handler{
		cfg:             cfg,
		sessionMgr:      sessionMgr,
		flashMgr:        flashMgr,
		deleteAccountUC: deleteAccountUC,
	}
}
