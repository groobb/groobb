// homeパッケージはサインイン済みユーザーのホームページ (GET /home) の
// ハンドラーを提供します。サインイン後に最初に着地するページです。
package home

import (
	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/usecase"
)

// HandlerはホームページのHTTPハンドラーです。
type Handler struct {
	cfg                      *config.Config
	getCommunityNavigationUC *usecase.GetCommunityNavigationUsecase
	getCommunityHomeUC       *usecase.GetCommunityHomeUsecase
}

// NewHandlerは新しいhome Handlerを作成します。
func NewHandler(
	cfg *config.Config,
	getCommunityNavigationUC *usecase.GetCommunityNavigationUsecase,
	getCommunityHomeUC *usecase.GetCommunityHomeUsecase,
) *Handler {
	return &Handler{
		cfg:                      cfg,
		getCommunityNavigationUC: getCommunityNavigationUC,
		getCommunityHomeUC:       getCommunityHomeUC,
	}
}
