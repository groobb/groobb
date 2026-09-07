// Package thread provides the handlers for the threads of a community: a
// thread's page (GET /t/{id}), which shows the posts written in it, the form a
// thread is started from (GET /b/{slug}/threads/new), and the submission of that
// form (POST /b/{slug}/threads).
//
// [Ja] thread パッケージはコミュニティのスレッドのハンドラーを提供します。スレッド
// ページ (GET /t/{id}。そこに書かれた投稿を表示するページ)、スレッドを立てるフォーム
// (GET /b/{slug}/threads/new)、そしてそのフォームの送信 (POST /b/{slug}/threads) です。
package thread

import (
	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/httperror"
	"github.com/groobb/groobb/go/internal/usecase"
)

// Handler is the HTTP handler for the pages of a thread. It holds the shared
// error renderer because an id naming no thread, and a slug naming no board, are
// answered with the same 404 page an unknown URL gets, rather than with
// not-found pages of their own.
//
// The board's thread listing is read by its own UseCase, the one a board's page
// uses, rather than being folded into the thread read: it is the same listing
// shown at /b/{slug}, and reading it twice from two places would let the two
// pages disagree about what the board holds. The board itself is resolved by the
// UseCase a board's page resolves it with, so the creation form under
// /b/{slug}/threads/new answers an unknown or differently spelled slug the way
// the board does.
//
// [Ja] Handler はスレッドの各ページの HTTP ハンドラーです。共通のエラーレンダラーを保持
// するのは、どのスレッドも指さない id と、どの掲示板も指さない slug に、未知の URL が
// 受け取るのと同じ 404 ページで応答するためです。それら専用の not-found ページは持ちません。
//
// 掲示板のスレッド一覧はスレッドの読み取りに畳み込まず、掲示板のページが使うのと同じ
// UseCase が読みます。それは /b/{slug} が表示するのと同じ一覧であり、2 箇所から別々に
// 読めば、2 つのページが掲示板の中身について食い違いうるためです。掲示板自身も、掲示板の
// ページがそれを解決するのと同じ UseCase で解決します。これにより /b/{slug}/threads/new の
// 作成フォームは、未知の slug や綴りの違う slug に掲示板と同じ応答を返します。
type Handler struct {
	cfg                      *config.Config
	errorRenderer            *httperror.Renderer
	getCommunityNavigationUC *usecase.GetCommunityNavigationUsecase
	getBoardUC               *usecase.GetBoardUsecase
	getThreadUC              *usecase.GetThreadUsecase
	getBoardThreadsUC        *usecase.GetBoardThreadsUsecase
	createThreadUC           *usecase.CreateThreadUsecase
}

// NewHandler creates a new thread Handler.
//
// [Ja] NewHandler は新しい thread Handler を作成します。
func NewHandler(
	cfg *config.Config,
	errorRenderer *httperror.Renderer,
	getCommunityNavigationUC *usecase.GetCommunityNavigationUsecase,
	getBoardUC *usecase.GetBoardUsecase,
	getThreadUC *usecase.GetThreadUsecase,
	getBoardThreadsUC *usecase.GetBoardThreadsUsecase,
	createThreadUC *usecase.CreateThreadUsecase,
) *Handler {
	return &Handler{
		cfg:                      cfg,
		errorRenderer:            errorRenderer,
		getCommunityNavigationUC: getCommunityNavigationUC,
		getBoardUC:               getBoardUC,
		getThreadUC:              getThreadUC,
		getBoardThreadsUC:        getBoardThreadsUC,
		createThreadUC:           createThreadUC,
	}
}
