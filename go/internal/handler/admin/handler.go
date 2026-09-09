// Package admin provides the handler for the admin hub (GET /admin), the page
// that links to the community's administration screens such as the user list.
//
// [Ja] admin パッケージは管理ハブ (GET /admin) のハンドラーを提供します。利用者一覧など、
// コミュニティの管理画面へリンクするページです。
package admin

import (
	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/httperror"
	"github.com/groobb/groobb/go/internal/usecase"
)

// Handler is the HTTP handler for the admin hub. It holds the shared error
// renderer because the page is one a signed-in visitor may be refused: the hub is
// behind RequireAuth, and being signed in is not by itself permission to open it.
//
// [Ja] Handler は管理ハブの HTTP ハンドラーです。共通のエラー Renderer を保持するのは、
// このページがサインイン済みの訪問者でも拒まれうるページであるためです。ハブは RequireAuth の
// 背後にありますが、サインインしていること自体はそれを開いてよいという意味ではありません。
type Handler struct {
	cfg            *config.Config
	errorRenderer  *httperror.Renderer
	getAdminHomeUC *usecase.GetAdminHomeUsecase
}

// NewHandler creates a new admin Handler.
//
// [Ja] NewHandler は新しい admin Handler を作成します。
func NewHandler(
	cfg *config.Config,
	errorRenderer *httperror.Renderer,
	getAdminHomeUC *usecase.GetAdminHomeUsecase,
) *Handler {
	return &Handler{cfg: cfg, errorRenderer: errorRenderer, getAdminHomeUC: getAdminHomeUC}
}
