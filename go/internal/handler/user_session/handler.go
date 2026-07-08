// Package user_session provides the handler for the user session resource:
// signing out deletes the current session and clears the session cookie
// (DELETE /user_session).
//
// [Ja] user_session パッケージはユーザーセッションリソースのハンドラーを提供します。
// サインアウトは現在のセッションを削除し、セッション Cookie を消去します
// (DELETE /user_session)。
package user_session

import (
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/usecase"
)

// Handler is the HTTP handler for the user session resource (sign-out).
//
// [Ja] Handler はユーザーセッションリソース (サインアウト) の HTTP ハンドラーです。
type Handler struct {
	sessionMgr      *session.Manager
	flashMgr        *session.FlashManager
	deleteSessionUC *usecase.DeleteSessionUsecase
}

// NewHandler creates a user session Handler.
//
// [Ja] NewHandler はユーザーセッション Handler を生成します。
func NewHandler(
	sessionMgr *session.Manager,
	flashMgr *session.FlashManager,
	deleteSessionUC *usecase.DeleteSessionUsecase,
) *Handler {
	return &Handler{
		sessionMgr:      sessionMgr,
		flashMgr:        flashMgr,
		deleteSessionUC: deleteSessionUC,
	}
}
