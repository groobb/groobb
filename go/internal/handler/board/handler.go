// Package board provides the handler for a board's page (GET /b/{slug}), which
// lists the threads posted in that board.
//
// [Ja] board パッケージは掲示板ページ (GET /b/{slug}) のハンドラーを提供します。
// その掲示板に立っているスレッドを並べるページです。
package board

import (
	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/httperror"
	"github.com/groobb/groobb/go/internal/usecase"
)

// Handler is the HTTP handler for a board's page. It holds the shared error
// renderer because a slug naming no board is answered with the same 404 page an
// unknown URL gets, rather than a second not-found page of its own.
//
// Resolving the board and reading its threads are two UseCases rather than one,
// so that a request whose answer is settled by the board alone — a 404, or a
// redirect to the canonical URL — does not pay for the listing, which is the
// unbounded part of the page.
//
// [Ja] Handler は掲示板ページの HTTP ハンドラーです。共通のエラーレンダラーを保持
// するのは、どの掲示板も指さない slug に、未知の URL が受け取るのと同じ 404 ページで
// 応答するためです。このページ専用の 2 つ目の not-found ページは持ちません。
//
// 掲示板の解決とそのスレッドの読み取りを 1 つではなく 2 つの UseCase にしているのは、
// 掲示板だけで応答が決まるリクエスト (404、および正規 URL へのリダイレクト) が、この
// ページで件数に上限の無い部分である一覧の分を支払わないようにするためです。
type Handler struct {
	cfg                      *config.Config
	errorRenderer            *httperror.Renderer
	getCommunityNavigationUC *usecase.GetCommunityNavigationUsecase
	getBoardUC               *usecase.GetBoardUsecase
	getBoardThreadsUC        *usecase.GetBoardThreadsUsecase
}

// NewHandler creates a new board Handler.
//
// [Ja] NewHandler は新しい board Handler を作成します。
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
