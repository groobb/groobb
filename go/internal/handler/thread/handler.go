// Package thread provides the handler for a thread's page (GET /t/{id}), which
// shows the posts written in that thread.
//
// [Ja] thread パッケージはスレッドページ (GET /t/{id}) のハンドラーを提供します。
// そのスレッドに書かれた投稿を表示するページです。
package thread

import (
	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/httperror"
	"github.com/groobb/groobb/go/internal/usecase"
)

// Handler is the HTTP handler for a thread's page. It holds the shared error
// renderer because an id naming no thread is answered with the same 404 page an
// unknown URL gets, rather than a second not-found page of its own.
//
// The board's thread listing is read by its own UseCase, the one a board's page
// uses, rather than being folded into the thread read: it is the same listing
// shown at /b/{slug}, and reading it twice from two places would let the two
// pages disagree about what the board holds.
//
// [Ja] Handler はスレッドページの HTTP ハンドラーです。共通のエラーレンダラーを保持
// するのは、どのスレッドも指さない id に、未知の URL が受け取るのと同じ 404 ページで
// 応答するためです。このページ専用の 2 つ目の not-found ページは持ちません。
//
// 掲示板のスレッド一覧はスレッドの読み取りに畳み込まず、掲示板のページが使うのと同じ
// UseCase が読みます。それは /b/{slug} が表示するのと同じ一覧であり、2 箇所から別々に
// 読めば、2 つのページが掲示板の中身について食い違いうるためです。
type Handler struct {
	cfg                      *config.Config
	errorRenderer            *httperror.Renderer
	getCommunityNavigationUC *usecase.GetCommunityNavigationUsecase
	getThreadUC              *usecase.GetThreadUsecase
	getBoardThreadsUC        *usecase.GetBoardThreadsUsecase
}

// NewHandler creates a new thread Handler.
//
// [Ja] NewHandler は新しい thread Handler を作成します。
func NewHandler(
	cfg *config.Config,
	errorRenderer *httperror.Renderer,
	getCommunityNavigationUC *usecase.GetCommunityNavigationUsecase,
	getThreadUC *usecase.GetThreadUsecase,
	getBoardThreadsUC *usecase.GetBoardThreadsUsecase,
) *Handler {
	return &Handler{
		cfg:                      cfg,
		errorRenderer:            errorRenderer,
		getCommunityNavigationUC: getCommunityNavigationUC,
		getThreadUC:              getThreadUC,
		getBoardThreadsUC:        getBoardThreadsUC,
	}
}
