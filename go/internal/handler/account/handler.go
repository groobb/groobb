// Package account provides the handlers for the final sign-up step: showing the
// password-setup form (GET /account/new) and creating the account, then signing
// the user in (POST /account). The email comes from the verified email
// confirmation carried in the handoff cookie, so this step only collects a
// password.
//
// [Ja] account パッケージはサインアップ最終ステップのハンドラーを提供します。パスワード
// 設定フォームの表示 (GET /account/new) と、アカウント作成後のサインイン
// (POST /account) です。email は受け渡し Cookie が運ぶ検証済みのメール確認から来るため、
// 本ステップはパスワードのみを収集します。
package account

import (
	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/usecase"
)

// Handler is the HTTP handler for the account-creation flow.
//
// [Ja] Handler はアカウント作成フローの HTTP ハンドラーです。
type Handler struct {
	cfg             *config.Config
	sessionMgr      *session.Manager
	createAccountUC *usecase.CreateAccountUsecase
	createSessionUC *usecase.CreateSessionUsecase
}

// NewHandler creates an account Handler.
//
// [Ja] NewHandler はアカウント Handler を生成します。
func NewHandler(
	cfg *config.Config,
	sessionMgr *session.Manager,
	createAccountUC *usecase.CreateAccountUsecase,
	createSessionUC *usecase.CreateSessionUsecase,
) *Handler {
	return &Handler{
		cfg:             cfg,
		sessionMgr:      sessionMgr,
		createAccountUC: createAccountUC,
		createSessionUC: createSessionUC,
	}
}
