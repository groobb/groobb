// settings_emailパッケージはメールアドレス変更申請フローのハンドラーを提供します。
// 現在のアドレスを表示する変更フォーム (GET /settings/email/edit) と、新しいアドレスと
// 現在のパスワードを受け付けて確認コードを発行する処理 (PATCH /settings/email) です。
// コードの検証と変更の適用は別フロー (handler/settings_email_confirmation) です。
package settings_email

import (
	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/usecase"
)

// Handlerはメールアドレス変更申請フローのHTTPハンドラーです。
type Handler struct {
	cfg                 *config.Config
	createEmailChangeUC *usecase.CreateEmailChangeUsecase
}

// NewHandlerはsettings_email Handlerを生成します。
func NewHandler(
	cfg *config.Config,
	createEmailChangeUC *usecase.CreateEmailChangeUsecase,
) *Handler {
	return &Handler{
		cfg:                 cfg,
		createEmailChangeUC: createEmailChangeUC,
	}
}
