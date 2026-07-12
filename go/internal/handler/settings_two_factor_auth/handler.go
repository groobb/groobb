// Package settings_two_factor_auth provides the handlers for setting up, enabling,
// and disabling TOTP-based two-factor authentication. GET /settings/two_factor_auth/new
// shows the enrollment form with a QR code when 2FA is off, or the disable
// confirmation form when it is already on. POST /settings/two_factor_auth enables 2FA
// after the user confirms a code from their authenticator app (activating the setting
// and showing the one-time recovery codes), and DELETE /settings/two_factor_auth
// disables it after re-authentication. All routes are behind RequireAuth, and the
// account is the signed-in user resolved from the session.
//
// [Ja] settings_two_factor_auth パッケージは TOTP による 2 段階認証の設定・有効化・無効化の
// ハンドラーを提供します。GET /settings/two_factor_auth/new は、2FA が無効なら QR コード付きの
// 登録フォームを、既に有効なら無効化の確認フォームを表示します。POST /settings/two_factor_auth は
// ユーザーが認証アプリのコードを確認した後に 2FA を有効化し (設定をアクティブにし、1 回使い切りの
// リカバリーコードを表示する)、DELETE /settings/two_factor_auth は再認証の後に無効化します。
// すべてのルートは RequireAuth の背後にあり、設定対象はセッションから解決したサインイン済み
// ユーザーです。
package settings_two_factor_auth

import (
	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/usecase"
)

// Handler is the HTTP handler for the two-factor authentication setup flow.
//
// [Ja] Handler は 2 段階認証の設定フローの HTTP ハンドラーです。
type Handler struct {
	cfg       *config.Config
	flashMgr  *session.FlashManager
	prepareUC *usecase.PrepareTwoFactorAuthUsecase
	enableUC  *usecase.EnableTwoFactorAuthUsecase
	disableUC *usecase.DisableTwoFactorAuthUsecase
}

// NewHandler creates a settings_two_factor_auth Handler.
//
// [Ja] NewHandler は settings_two_factor_auth Handler を生成します。
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
