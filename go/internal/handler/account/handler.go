// accountパッケージはサインアップ最終ステップのハンドラーを提供します。パスワード
// 設定フォームの表示 (GET /account/new) と、アカウント作成後のサインイン
// (POST /account) です。emailは受け渡しCookieが運ぶ検証済みのメール確認から来るため、
// 本ステップはパスワードのみを収集します。
package account

import (
	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/usecase"
)

// Handlerはアカウント作成フローのHTTPハンドラーです。
type Handler struct {
	cfg             *config.Config
	sessionMgr      *session.Manager
	createAccountUC *usecase.CreateAccountUsecase
	createSessionUC *usecase.CreateSessionUsecase
}

// NewHandlerはアカウントHandlerを生成します。
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
