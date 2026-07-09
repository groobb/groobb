// Package settings_email provides the handlers for the email-change request
// flow: showing the change form with the current address (GET /settings/email/edit)
// and accepting a new address and the current password to issue a confirmation
// code (PATCH /settings/email). Verifying the code and applying the change is a
// separate flow (handler/settings_email_confirmation).
//
// [Ja] settings_email パッケージはメールアドレス変更申請フローのハンドラーを提供します。
// 現在のアドレスを表示する変更フォーム (GET /settings/email/edit) と、新しいアドレスと
// 現在のパスワードを受け付けて確認コードを発行する処理 (PATCH /settings/email) です。
// コードの検証と変更の適用は別フロー (handler/settings_email_confirmation) です。
package settings_email

import (
	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/usecase"
)

// Handler is the HTTP handler for the email-change request flow.
//
// [Ja] Handler はメールアドレス変更申請フローの HTTP ハンドラーです。
type Handler struct {
	cfg                 *config.Config
	createEmailChangeUC *usecase.CreateEmailChangeUsecase
}

// NewHandler creates a settings_email Handler.
//
// [Ja] NewHandler は settings_email Handler を生成します。
func NewHandler(
	cfg *config.Config,
	createEmailChangeUC *usecase.CreateEmailChangeUsecase,
) *Handler {
	return &Handler{
		cfg:                 cfg,
		createEmailChangeUC: createEmailChangeUC,
	}
}
