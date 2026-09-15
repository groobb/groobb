// postパッケージはスレッドの投稿のハンドラー、すなわち返信の送信
// (POST /t/{id}/posts) を提供します。
//
// スレッドの投稿がこのアドレスへ来るのは書き込むときだけです。投稿はスレッド自身のページで
// 読まれ、そこでは与えられたレス番号で名指されます (ADR 0009)。したがって本パッケージは、
// 読む側がスレッドに属するリソースの、書く側を持ちます。
package post

import (
	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/httperror"
	"github.com/groobb/groobb/go/internal/usecase"
)

// Handlerはスレッドの投稿のHTTPハンドラーです。共通のエラーレンダラーを保持する
// のは、どのスレッドも指さないidに、専用のnot-foundページではなく、未知のURLが
// 受け取るのと同じ404ページで応答するためです。
//
// スレッドの読み戻しには、スレッドのページが使うUseCaseではなく、投稿を伴わずに
// スレッドを読むUseCaseを使います。このハンドラーがページを描くのは送信が拒否された
// ときだけであり、そのページは会話を見せずにスレッドを名指します。満杯のスレッド (ここが
// 最も多く応答する拒否) の投稿をすべて読めば、1件も描かないために1000行を支払うことに
// なります。
type Handler struct {
	cfg                      *config.Config
	errorRenderer            *httperror.Renderer
	getCommunityNavigationUC *usecase.GetCommunityNavigationUsecase
	getThreadSummaryUC       *usecase.GetThreadSummaryUsecase
	createPostUC             *usecase.CreatePostUsecase
}

// NewHandlerは新しいpost Handlerを作成します。
func NewHandler(
	cfg *config.Config,
	errorRenderer *httperror.Renderer,
	getCommunityNavigationUC *usecase.GetCommunityNavigationUsecase,
	getThreadSummaryUC *usecase.GetThreadSummaryUsecase,
	createPostUC *usecase.CreatePostUsecase,
) *Handler {
	return &Handler{
		cfg:                      cfg,
		errorRenderer:            errorRenderer,
		getCommunityNavigationUC: getCommunityNavigationUC,
		getThreadSummaryUC:       getThreadSummaryUC,
		createPostUC:             createPostUC,
	}
}
