// Package sign_in_two_factor provides the handlers for the TOTP challenge step of
// signing in: showing the code-entry form (GET /sign_in/two_factor/new) and
// verifying the code to complete sign-in (POST /sign_in/two_factor). These are
// public routes reached mid-sign-in, after the password step diverted a 2FA-enabled
// account here and held the pending user in a short-lived cookie; a valid code
// issues the session, clears the pending cookie, and signs the user in.
//
// [Ja] sign_in_two_factor パッケージはサインインの TOTP チャレンジステップのハンドラーを
// 提供します。コード入力フォームの表示 (GET /sign_in/two_factor/new) と、サインインを完了
// させるためのコード検証 (POST /sign_in/two_factor) です。これらはサインインの途中で通る
// 公開ルートで、パスワードのステップが 2FA 有効なアカウントをここへ迂回させ、保留中ユーザーを
// 短命の Cookie に保持した後に到達します。正しいコードでセッションを発行し、pending Cookie を
// 消去して、ユーザーをサインインさせます。
package sign_in_two_factor

import (
	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/usecase"
)

// Handler is the HTTP handler for the sign-in two-factor challenge flow.
//
// [Ja] Handler はサインイン 2 段階認証チャレンジフローの HTTP ハンドラーです。
type Handler struct {
	cfg                     *config.Config
	sessionMgr              *session.Manager
	createSignInTwoFactorUC *usecase.CreateSignInTwoFactorUsecase
	createSessionUC         *usecase.CreateSessionUsecase
}

// NewHandler creates a sign_in_two_factor Handler.
//
// [Ja] NewHandler は sign_in_two_factor Handler を生成します。
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
