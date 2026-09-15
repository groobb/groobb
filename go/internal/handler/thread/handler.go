// threadパッケージはコミュニティのスレッドのハンドラーを提供します。スレッド
// ページ (GET /t/{id}。そこに書かれた投稿を表示するページ)、スレッドを立てるフォーム
// (GET /b/{slug}/threads/new)、そしてそのフォームの送信 (POST /b/{slug}/threads) です。
package thread

import (
	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/httperror"
	"github.com/groobb/groobb/go/internal/usecase"
)

// Handlerはスレッドの各ページのHTTPハンドラーです。共通のエラーレンダラーを保持
// するのは、どのスレッドも指さないidと、どの掲示板も指さないslugに、未知のURLが
// 受け取るのと同じ404ページで応答するためです。それら専用のnot-foundページは持ちません。
//
// 掲示板のスレッド一覧はスレッドの読み取りに畳み込まず、掲示板のページが使うのと同じ
// UseCaseが読みます。それは /b/{slug} が表示するのと同じ一覧であり、2箇所から別々に
// 読めば、2つのページが掲示板の中身について食い違いうるためです。掲示板自身も、掲示板の
// ページがそれを解決するのと同じUseCaseで解決します。これにより /b/{slug}/threads/newの
// 作成フォームは、未知のslugや綴りの違うslugに掲示板と同じ応答を返します。
type Handler struct {
	cfg                      *config.Config
	errorRenderer            *httperror.Renderer
	getCommunityNavigationUC *usecase.GetCommunityNavigationUsecase
	getBoardUC               *usecase.GetBoardUsecase
	getThreadUC              *usecase.GetThreadUsecase
	getBoardThreadsUC        *usecase.GetBoardThreadsUsecase
	createThreadUC           *usecase.CreateThreadUsecase
}

// NewHandlerは新しいthread Handlerを作成します。
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
