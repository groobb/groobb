// Package settings_email_confirmation provides the handlers for the code-entry
// step of the email-change flow: showing the form (GET
// /settings/email/confirmation/new) and verifying the code the user types back,
// which applies the new address on success (POST /settings/email/confirmation).
// Both routes are behind RequireAuth, and the pending confirmation is resolved
// from the signed-in user rather than a handoff cookie.
//
// [Ja] settings_email_confirmation パッケージはメールアドレス変更フローのコード入力
// ステップのハンドラーを提供します。フォーム表示 (GET /settings/email/confirmation/new)
// と、ユーザーが入力し返したコードの検証 (成功時に新しいアドレスを適用する。
// POST /settings/email/confirmation) です。どちらのルートも RequireAuth の背後にあり、
// 保留中の確認は受け渡し Cookie ではなくサインイン済みユーザーから解決します。
package settings_email_confirmation

import (
	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/usecase"
)

// Handler is the HTTP handler for the email-change code-entry flow.
//
// [Ja] Handler はメールアドレス変更のコード入力フローの HTTP ハンドラーです。
type Handler struct {
	cfg                 *config.Config
	flashMgr            *session.FlashManager
	verifyEmailChangeUC *usecase.VerifyEmailChangeUsecase
}

// NewHandler creates a settings_email_confirmation Handler.
//
// [Ja] NewHandler は settings_email_confirmation Handler を生成します。
func NewHandler(
	cfg *config.Config,
	flashMgr *session.FlashManager,
	verifyEmailChangeUC *usecase.VerifyEmailChangeUsecase,
) *Handler {
	return &Handler{
		cfg:                 cfg,
		flashMgr:            flashMgr,
		verifyEmailChangeUC: verifyEmailChangeUC,
	}
}
