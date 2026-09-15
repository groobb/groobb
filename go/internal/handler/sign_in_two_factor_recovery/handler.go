// sign_in_two_factor_recoveryパッケージはサインインのリカバリーコードチャレンジ
// ステップのハンドラーを提供します。リカバリーコード入力フォームの表示
// (GET /sign_in/two_factor/recovery/new) と、サインインを完了させるためのコード検証
// (POST /sign_in/two_factor/recovery) です。これらはサインインの途中で通る公開ルートで、
// パスワードのステップが2FA有効なアカウントを迂回させ、保留中ユーザーを短命のCookieに
// 保持した後に到達し、認証アプリを使えないときのフォールバックです。正しいコードはその
// 1回使い切りのリカバリーコードを消費し、セッションを発行し、pending Cookieを消去して、
// ユーザーをサインインさせます。消費とセッション発行はUseCase内でアトミックに行われます。
package sign_in_two_factor_recovery

import (
	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/usecase"
)

// HandlerはサインインのリカバリーコードチャレンジフローのHTTPハンドラーです。
type Handler struct {
	cfg                             *config.Config
	sessionMgr                      *session.Manager
	createSignInTwoFactorRecoveryUC *usecase.CreateSignInTwoFactorRecoveryUsecase
}

// NewHandlerはsign_in_two_factor_recovery Handlerを生成します。
func NewHandler(
	cfg *config.Config,
	sessionMgr *session.Manager,
	createSignInTwoFactorRecoveryUC *usecase.CreateSignInTwoFactorRecoveryUsecase,
) *Handler {
	return &Handler{
		cfg:                             cfg,
		sessionMgr:                      sessionMgr,
		createSignInTwoFactorRecoveryUC: createSignInTwoFactorRecoveryUC,
	}
}
