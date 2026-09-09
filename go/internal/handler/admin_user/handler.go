// Package admin_user provides the handler for the admin user listing
// (GET /admin/users), the page the community's accounts are read on and their
// roles are handed out from.
//
// [Ja] admin_user パッケージは管理画面の利用者一覧 (GET /admin/users) のハンドラーを
// 提供します。コミュニティのアカウントを読み、その人たちにロールを渡すページです。
package admin_user

import (
	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/httperror"
	"github.com/groobb/groobb/go/internal/usecase"
)

// Handler is the HTTP handler for the admin user listing. It holds the shared
// error renderer because the listing answers two refusals of its own: a
// signed-in visitor who may not read it, and an address carrying a page value
// that is not a whole number or is below the first page.
//
// [Ja] Handler は管理画面の利用者一覧の HTTP ハンドラーです。共通のエラー Renderer を
// 保持するのは、この一覧が自身で 2 種類の拒否に応答するためです。これを読んではならない
// サインイン済みの訪問者と、整数として読めない、または最初のページより前の値を運ぶアドレスです。
type Handler struct {
	cfg             *config.Config
	errorRenderer   *httperror.Renderer
	getAdminUsersUC *usecase.GetAdminUsersUsecase
}

// NewHandler creates a new admin_user Handler.
//
// [Ja] NewHandler は新しい admin_user Handler を作成します。
func NewHandler(
	cfg *config.Config,
	errorRenderer *httperror.Renderer,
	getAdminUsersUC *usecase.GetAdminUsersUsecase,
) *Handler {
	return &Handler{cfg: cfg, errorRenderer: errorRenderer, getAdminUsersUC: getAdminUsersUC}
}
