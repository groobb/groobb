// sign_in_two_factorパッケージはサインインのTOTPチャレンジステップのハンドラーを
// 提供します。コード入力フォームの表示 (GET /sign_in/two_factor/new) と、サインインを完了
// させるためのコード検証 (POST /sign_in/two_factor) です。これらはサインインの途中で通る
// 公開ルートで、パスワードのステップが2FA有効なアカウントをここへ迂回させ、保留中ユーザーを
// 短命のCookieに保持した後に到達します。正しいコードでセッションを発行し、pending Cookieを
// 消去して、ユーザーをサインインさせます。
package sign_in_two_factor

import (
	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/usecase"
)

// Handlerはサインイン2段階認証チャレンジフローのHTTPハンドラーです。
type Handler struct {
	cfg                     *config.Config
	sessionMgr              *session.Manager
	createSignInTwoFactorUC *usecase.CreateSignInTwoFactorUsecase
	createSessionUC         *usecase.CreateSessionUsecase
}

// NewHandlerはsign_in_two_factor Handlerを生成します。
func NewHandler(
	cfg *config.Config,
	sessionMgr *session.Manager,
	createSignInTwoFactorUC *usecase.CreateSignInTwoFactorUsecase,
	createSessionUC *usecase.CreateSessionUsecase,
) *Handler {
	return &Handler{
		cfg:                     cfg,
		sessionMgr:              sessionMgr,
		createSignInTwoFactorUC: createSignInTwoFactorUC,
		createSessionUC:         createSessionUC,
	}
}
