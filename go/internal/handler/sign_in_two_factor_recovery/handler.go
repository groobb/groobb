// Package sign_in_two_factor_recovery provides the handlers for the recovery-code
// challenge step of signing in: showing the recovery-code entry form
// (GET /sign_in/two_factor/recovery/new) and verifying a code to complete sign-in
// (POST /sign_in/two_factor/recovery). These are public routes reached mid-sign-in,
// after the password step diverted a 2FA-enabled account and held the pending user
// in a short-lived cookie; they are the fallback for when the authenticator app is
// unavailable. A valid code consumes that one-time recovery code, issues the
// session, clears the pending cookie, and signs the user in — the consume and the
// session issuance happening atomically in the UseCase.
//
// [Ja] sign_in_two_factor_recovery パッケージはサインインのリカバリーコードチャレンジ
// ステップのハンドラーを提供します。リカバリーコード入力フォームの表示
// (GET /sign_in/two_factor/recovery/new) と、サインインを完了させるためのコード検証
// (POST /sign_in/two_factor/recovery) です。これらはサインインの途中で通る公開ルートで、
// パスワードのステップが 2FA 有効なアカウントを迂回させ、保留中ユーザーを短命の Cookie に
// 保持した後に到達し、認証アプリを使えないときのフォールバックです。正しいコードはその
// 1 回使い切りのリカバリーコードを消費し、セッションを発行し、pending Cookie を消去して、
// ユーザーをサインインさせます。消費とセッション発行は UseCase 内でアトミックに行われます。
package sign_in_two_factor_recovery

import (
	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/usecase"
)

// Handler is the HTTP handler for the sign-in recovery-code challenge flow.
//
// [Ja] Handler はサインインのリカバリーコードチャレンジフローの HTTP ハンドラーです。
type Handler struct {
	cfg                             *config.Config
	sessionMgr                      *session.Manager
	createSignInTwoFactorRecoveryUC *usecase.CreateSignInTwoFactorRecoveryUsecase
}

// NewHandler creates a sign_in_two_factor_recovery Handler.
//
// [Ja] NewHandler は sign_in_two_factor_recovery Handler を生成します。
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
