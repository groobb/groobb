// Package settings_withdrawal provides the handlers for the account-withdrawal
// flow: showing the confirmation form (GET /settings/withdrawal/new) and executing
// the withdrawal (DELETE /settings/withdrawal), which soft-deletes and anonymizes
// the account and deletes all of its sessions. Both routes are behind RequireAuth,
// and the account to withdraw is the signed-in user resolved from the session.
//
// [Ja] settings_withdrawal パッケージは退会フローのハンドラーを提供します。確認フォームの
// 表示 (GET /settings/withdrawal/new) と、退会の実行 (DELETE /settings/withdrawal) です。
// 退会の実行はアカウントを論理削除・匿名化し、その全セッションを削除します。どちらのルートも
// RequireAuth の背後にあり、退会対象はセッションから解決したサインイン済みユーザーです。
package settings_withdrawal

import (
	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/usecase"
)

// Handler is the HTTP handler for the account-withdrawal flow.
//
// [Ja] Handler は退会フローの HTTP ハンドラーです。
type Handler struct {
	cfg             *config.Config
	sessionMgr      *session.Manager
	flashMgr        *session.FlashManager
	deleteAccountUC *usecase.DeleteAccountUsecase
}

// NewHandler creates a settings_withdrawal Handler.
//
// [Ja] NewHandler は settings_withdrawal Handler を生成します。
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
