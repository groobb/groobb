// settings_email_confirmationパッケージはメールアドレス変更フローのコード入力
// ステップのハンドラーを提供します。フォーム表示 (GET /settings/email/confirmation/new)
// と、ユーザーが入力し返したコードの検証 (成功時に新しいアドレスを適用する。
// POST /settings/email/confirmation) です。どちらのルートもRequireAuthの背後にあり、
// 保留中の確認は受け渡しCookieではなくサインイン済みユーザーから解決します。
package settings_email_confirmation

import (
	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/usecase"
)

// Handlerはメールアドレス変更のコード入力フローのHTTPハンドラーです。
type Handler struct {
	cfg                 *config.Config
	flashMgr            *session.FlashManager
	verifyEmailChangeUC *usecase.VerifyEmailChangeUsecase
}

// NewHandlerはsettings_email_confirmation Handlerを生成します。
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
