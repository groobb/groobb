// boardパッケージは掲示板ページ (GET /b/{slug}) のハンドラーを提供します。
// その掲示板に立っているスレッドを並べるページです。
package board

import (
	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/httperror"
	"github.com/groobb/groobb/go/internal/usecase"
)

// Handlerは掲示板ページのHTTPハンドラーです。共通のエラーレンダラーを保持
// するのは、どの掲示板も指さないslugに、未知のURLが受け取るのと同じ404ページで
// 応答するためです。このページ専用の2つ目のnot-foundページは持ちません。
//
// 掲示板の解決とそのスレッドの読み取りを1つではなく2つのUseCaseにしているのは、
// 掲示板だけで応答が決まるリクエスト (404、および正規URLへのリダイレクト) が、この
// ページで件数に上限の無い部分である一覧の分を支払わないようにするためです。
type Handler struct {
	cfg                      *config.Config
	errorRenderer            *httperror.Renderer
	getCommunityNavigationUC *usecase.GetCommunityNavigationUsecase
	getBoardUC               *usecase.GetBoardUsecase
	getBoardThreadsUC        *usecase.GetBoardThreadsUsecase
}

// NewHandlerは新しいboard Handlerを作成します。
func NewHandler(
	cfg *config.Config,
	errorRenderer *httperror.Renderer,
	getCommunityNavigationUC *usecase.GetCommunityNavigationUsecase,
	getBoardUC *usecase.GetBoardUsecase,
	getBoardThreadsUC *usecase.GetBoardThreadsUsecase,
) *Handler {
	return &Handler{
		cfg:                      cfg,
		errorRenderer:            errorRenderer,
		getCommunityNavigationUC: getCommunityNavigationUC,
		getBoardUC:               getBoardUC,
		getBoardThreadsUC:        getBoardThreadsUC,
	}
}
