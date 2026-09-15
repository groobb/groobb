// user_sessionパッケージはユーザーセッションリソースのハンドラーを提供します。
// サインアウトは現在のセッションを削除し、セッションCookieを消去します
// (DELETE /user_session)。
package user_session

import (
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/usecase"
)

// Handlerはユーザーセッションリソース (サインアウト) のHTTPハンドラーです。
type Handler struct {
	sessionMgr      *session.Manager
	flashMgr        *session.FlashManager
	deleteSessionUC *usecase.DeleteSessionUsecase
}

// NewHandlerはユーザーセッションHandlerを生成します。
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
