// admin_userパッケージは管理画面の利用者一覧 (GET /admin/users) のハンドラーを
// 提供します。コミュニティのアカウントを読み、その人たちにロールを渡すページです。
package admin_user

import (
	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/httperror"
	"github.com/groobb/groobb/go/internal/usecase"
)

// Handlerは管理画面の利用者一覧のHTTPハンドラーです。共通のエラーRendererを
// 保持するのは、この一覧が自身で2種類の拒否に応答するためです。これを読んではならない
// サインイン済みの訪問者と、整数として読めない、または最初のページより前の値を運ぶアドレスです。
type Handler struct {
	cfg             *config.Config
	errorRenderer   *httperror.Renderer
	getAdminUsersUC *usecase.GetAdminUsersUsecase
}

// NewHandlerは新しいadmin_user Handlerを作成します。
func NewHandler(
	cfg *config.Config,
	errorRenderer *httperror.Renderer,
	getAdminUsersUC *usecase.GetAdminUsersUsecase,
) *Handler {
	return &Handler{cfg: cfg, errorRenderer: errorRenderer, getAdminUsersUC: getAdminUsersUC}
}
