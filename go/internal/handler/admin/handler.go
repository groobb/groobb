// adminパッケージは管理ハブ (GET /admin) のハンドラーを提供します。利用者一覧など、
// コミュニティの管理画面へリンクするページです。
package admin

import (
	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/httperror"
	"github.com/groobb/groobb/go/internal/usecase"
)

// Handlerは管理ハブのHTTPハンドラーです。共通のエラーRendererを保持するのは、
// このページがサインイン済みの訪問者でも拒まれうるページであるためです。ハブはRequireAuthの
// 背後にありますが、サインインしていること自体はそれを開いてよいという意味ではありません。
type Handler struct {
	cfg            *config.Config
	errorRenderer  *httperror.Renderer
	getAdminHomeUC *usecase.GetAdminHomeUsecase
}

// NewHandlerは新しいadmin Handlerを作成します。
func NewHandler(
	cfg *config.Config,
	errorRenderer *httperror.Renderer,
	getAdminHomeUC *usecase.GetAdminHomeUsecase,
) *Handler {
	return &Handler{cfg: cfg, errorRenderer: errorRenderer, getAdminHomeUC: getAdminHomeUC}
}
