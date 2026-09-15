// settings_two_factor_authパッケージはTOTPによる2段階認証の設定・有効化・無効化の
// ハンドラーを提供します。GET /settings/two_factor_auth/newは、2FAが無効ならQRコード付きの
// 登録フォームを、既に有効なら無効化の確認フォームを表示します。POST /settings/two_factor_authは
// ユーザーが認証アプリのコードを確認した後に2FAを有効化し (設定をアクティブにし、1回使い切りの
// リカバリーコードを表示する)、DELETE /settings/two_factor_authは再認証の後に無効化します。
// すべてのルートはRequireAuthの背後にあり、設定対象はセッションから解決したサインイン済み
// ユーザーです。
package settings_two_factor_auth

import (
	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/usecase"
)

// Handlerは2段階認証の設定フローのHTTPハンドラーです。
type Handler struct {
	cfg       *config.Config
	flashMgr  *session.FlashManager
	prepareUC *usecase.PrepareTwoFactorAuthUsecase
	enableUC  *usecase.EnableTwoFactorAuthUsecase
	disableUC *usecase.DisableTwoFactorAuthUsecase
}

// NewHandlerはsettings_two_factor_auth Handlerを生成します。
func NewHandler(
	cfg *config.Config,
	flashMgr *session.FlashManager,
	prepareUC *usecase.PrepareTwoFactorAuthUsecase,
	enableUC *usecase.EnableTwoFactorAuthUsecase,
	disableUC *usecase.DisableTwoFactorAuthUsecase,
) *Handler {
	return &Handler{
		cfg:       cfg,
		flashMgr:  flashMgr,
		prepareUC: prepareUC,
		enableUC:  enableUC,
		disableUC: disableUC,
	}
}
