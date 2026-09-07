// Package post provides the handler for the posts of a thread: the submission
// of a reply (POST /t/{id}/posts).
//
// The posts of a thread are only ever written at this address. They are read at
// the thread's own page, where each of them is addressed by the reply number it
// was given (ADR 0009), so this package holds the writing half of a resource
// whose reading half belongs to the thread.
//
// [Ja] post パッケージはスレッドの投稿のハンドラー、すなわち返信の送信
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

// Handler is the HTTP handler for the posts of a thread. It holds the shared
// error renderer because an id naming no thread is answered with the same 404
// page an unknown URL gets, rather than with a not-found page of its own.
//
// The thread is read back by the UseCase that reads a thread without its posts,
// rather than by the one a thread's page uses. This handler only ever draws a
// page when a submission was refused, and that page names the thread instead of
// showing the conversation: reading every post of a thread that is full — which
// is the refusal it answers most — would cost a thousand rows to draw none of
// them.
//
// [Ja] Handler はスレッドの投稿の HTTP ハンドラーです。共通のエラーレンダラーを保持する
// のは、どのスレッドも指さない id に、専用の not-found ページではなく、未知の URL が
// 受け取るのと同じ 404 ページで応答するためです。
//
// スレッドの読み戻しには、スレッドのページが使う UseCase ではなく、投稿を伴わずに
// スレッドを読む UseCase を使います。このハンドラーがページを描くのは送信が拒否された
// ときだけであり、そのページは会話を見せずにスレッドを名指します。満杯のスレッド (ここが
// 最も多く応答する拒否) の投稿をすべて読めば、1 件も描かないために 1000 行を支払うことに
// なります。
type Handler struct {
	cfg                      *config.Config
	errorRenderer            *httperror.Renderer
	getCommunityNavigationUC *usecase.GetCommunityNavigationUsecase
	getThreadSummaryUC       *usecase.GetThreadSummaryUsecase
	createPostUC             *usecase.CreatePostUsecase
}

// NewHandler creates a new post Handler.
//
// [Ja] NewHandler は新しい post Handler を作成します。
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
