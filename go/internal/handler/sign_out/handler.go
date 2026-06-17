// Package sign_out provides the handler for signing out: deleting the current
// session and clearing the session cookie (POST /sign_out).
//
// [Ja] sign_out パッケージはサインアウトのハンドラーを提供します。現在のセッションを削除し、
// セッション Cookie を消去します (POST /sign_out)。
package sign_out

import (
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/usecase"
)

// Handler is the HTTP handler for the sign-out flow.
//
// [Ja] Handler はサインアウトフローの HTTP ハンドラーです。
type Handler struct {
	sessionMgr      *session.Manager
	deleteSessionUC *usecase.DeleteSessionUsecase
}

// NewHandler creates a sign-out Handler.
//
// [Ja] NewHandler はサインアウト Handler を生成します。
func NewHandler(
	sessionMgr *session.Manager,
	deleteSessionUC *usecase.DeleteSessionUsecase,
) *Handler {
	return &Handler{
		sessionMgr:      sessionMgr,
		deleteSessionUC: deleteSessionUC,
	}
}
