// Package category provides the handler for a category's page
// (GET /c/{slug}), which lists the boards that category groups.
//
// [Ja] category パッケージはカテゴリーページ (GET /c/{slug}) のハンドラーを提供します。
// そのカテゴリーがまとめる掲示板を並べるページです。
package category

import (
	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/httperror"
	"github.com/groobb/groobb/go/internal/usecase"
)

// Handler is the HTTP handler for a category's page. It holds the shared error
// renderer because a slug naming no category is answered with the same 404 page
// an unknown URL gets, rather than a second not-found page of its own.
//
// Resolving the category and reading its boards are two UseCases rather than
// one, so that a request whose answer is settled by the category alone — a 404,
// or a redirect to the canonical URL — does not pay for the listing.
//
// [Ja] Handler はカテゴリーページの HTTP ハンドラーです。共通のエラーレンダラーを保持
// するのは、どのカテゴリーも指さない slug に、未知の URL が受け取るのと同じ 404 ページ
// で応答するためです。このページ専用の 2 つ目の not-found ページは持ちません。
//
// カテゴリーの解決とその掲示板の読み取りを 1 つではなく 2 つの UseCase にしているのは、
// カテゴリーだけで応答が決まるリクエスト (404、および正規 URL へのリダイレクト) が
// 一覧の分を支払わないようにするためです。
type Handler struct {
	cfg                      *config.Config
	errorRenderer            *httperror.Renderer
	getCommunityNavigationUC *usecase.GetCommunityNavigationUsecase
	getCategoryUC            *usecase.GetCategoryUsecase
	getCategoryBoardsUC      *usecase.GetCategoryBoardsUsecase
}

// NewHandler creates a new category Handler.
//
// [Ja] NewHandler は新しい category Handler を作成します。
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
