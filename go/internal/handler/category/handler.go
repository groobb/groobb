// categoryパッケージはカテゴリーページ (GET /c/{slug}) のハンドラーを提供します。
// そのカテゴリーがまとめる掲示板を並べるページです。
package category

import (
	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/httperror"
	"github.com/groobb/groobb/go/internal/usecase"
)

// HandlerはカテゴリーページのHTTPハンドラーです。共通のエラーレンダラーを保持
// するのは、どのカテゴリーも指さないslugに、未知のURLが受け取るのと同じ404ページ
// で応答するためです。このページ専用の2つ目のnot-foundページは持ちません。
//
// カテゴリーの解決とその掲示板の読み取りを1つではなく2つのUseCaseにしているのは、
// カテゴリーだけで応答が決まるリクエスト (404、および正規URLへのリダイレクト) が
// 一覧の分を支払わないようにするためです。
type Handler struct {
	cfg                      *config.Config
	errorRenderer            *httperror.Renderer
	getCommunityNavigationUC *usecase.GetCommunityNavigationUsecase
	getCategoryUC            *usecase.GetCategoryUsecase
	getCategoryBoardsUC      *usecase.GetCategoryBoardsUsecase
}

// NewHandlerは新しいcategory Handlerを作成します。
func NewHandler(
	cfg *config.Config,
	errorRenderer *httperror.Renderer,
	getCommunityNavigationUC *usecase.GetCommunityNavigationUsecase,
	getCategoryUC *usecase.GetCategoryUsecase,
	getCategoryBoardsUC *usecase.GetCategoryBoardsUsecase,
) *Handler {
	return &Handler{
		cfg:                      cfg,
		errorRenderer:            errorRenderer,
		getCommunityNavigationUC: getCommunityNavigationUC,
		getCategoryUC:            getCategoryUC,
		getCategoryBoardsUC:      getCategoryBoardsUC,
	}
}
