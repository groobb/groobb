// Package thread_unpublication provides the handlers for taking a thread out of
// the community's view: the confirmation page
// (GET /t/{id}/unpublication/new) and the unpublication itself
// (POST /t/{id}/unpublication). Both are behind RequireAuth, and whether a
// signed-in visitor may carry it out is settled by the UseCase each of them
// calls.
//
// There is no route for putting the thread back. The mark is what hides the
// thread and nothing under it is destroyed, so taking the mark off would bring
// it back whole; this instance simply offers no screen that does so, and the
// confirmation page says as much before the button is pressed.
//
// [Ja] thread_unpublicationパッケージは、スレッドをコミュニティの視界から外すためのハンドラー
// を提供します。確認ページ (GET /t/{id}/unpublication/new) と、非公開そのもの
// (POST /t/{id}/unpublication) です。どちらもRequireAuthの背後にあり、サインイン済みの
// 訪問者がそれを行ってよいかどうかは、それぞれが呼ぶUseCaseが決めます。
//
// スレッドを戻すルートはありません。スレッドを隠しているのは印であり、その下の何も失われて
// いないため、印を外せばスレッドは丸ごと戻ります。このインスタンスがそれを行う画面を持たない
// というだけであり、確認ページはボタンが押される前にそのことを述べます。
package thread_unpublication

import (
	"log/slog"
	"net/http"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/httperror"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/usecase"
)

// Handler is the HTTP handler for a thread's unpublication. It holds the shared
// error renderer because both routes answer the same three refusals with a page
// rather than with the thread: a visitor who may not moderate, an address naming
// no thread, and a thread the community has already taken out of view. The flash
// manager carries what happened across the redirect, since the thread the
// operation was performed on is no longer somewhere to land.
//
// [Ja] HandlerはスレッドのHTTPハンドラーで、その非公開を扱います。共通のエラーRendererを
// 保持するのは、2つのルートがいずれも同じ3種類の拒否に、スレッドではなくページで応答する
// ためです。モデレーションを許されていない訪問者、どのスレッドも名指していないアドレス、そして
// コミュニティが既に視界の外へ移したスレッドです。フラッシュManagerは、何が起きたのかを
// リダイレクトの先へ運びます。操作が行われたスレッドは、もう降り立つ場所ではないためです。
type Handler struct {
	cfg               *config.Config
	errorRenderer     *httperror.Renderer
	flashMgr          *session.FlashManager
	getModerationUC   *usecase.GetThreadModerationUsecase
	unpublishThreadUC *usecase.UnpublishThreadUsecase
}

// NewHandler creates a new thread_unpublication Handler.
//
// [Ja] NewHandlerは新しいthread_unpublication Handlerを作成します。
func NewHandler(
	cfg *config.Config,
	errorRenderer *httperror.Renderer,
	flashMgr *session.FlashManager,
	getModerationUC *usecase.GetThreadModerationUsecase,
	unpublishThreadUC *usecase.UnpublishThreadUsecase,
) *Handler {
	return &Handler{
		cfg:               cfg,
		errorRenderer:     errorRenderer,
		flashMgr:          flashMgr,
		getModerationUC:   getModerationUC,
		unpublishThreadUC: unpublishThreadUC,
	}
}

// refused answers a request the UseCase did not carry out, turning what it
// refused for into the response the visitor gets.
//
// The three known refusals are answered with a page, because none of them is
// something the unpublication screen can say anything about: the visitor may not
// be here, the address names no thread, or the thread is already out of view. It
// reports whether it answered, so a caller with a refusal of its own to handle —
// a note that is too long, which comes back on the form — asks this first and
// draws the form when the answer was not settled here.
//
// [Ja] refusedは、UseCaseが実行しなかった要求に応答し、拒否の理由を訪問者が受け取る応答へ
// 変えます。
//
// 既知の3つの拒否にはページで応答します。どれも非公開の画面が何かを述べられるものではない
// ためです。訪問者がここに居てはならないか、アドレスがどのスレッドも名指していないか、
// スレッドが既に視界の外にあるかです。応答したかどうかを返すのは、自身で扱う拒否を持つ
// 呼び出し元 (長すぎる注記。これはフォームに載って戻ってきます) が、まずこれに尋ね、ここで
// 答えが決まらなかったときにフォームを描けるようにするためです。
func (h *Handler) refused(w http.ResponseWriter, r *http.Request, err error) bool {
	ctx := r.Context()

	ae := model.AsAppError(err)
	if ae == nil {
		return false
	}

	switch ae.Code {
	case model.AppErrCodeForbidden:
		slog.InfoContext(ctx, ae.LogString())
		h.errorRenderer.Forbidden(w, r)
	case model.AppErrCodeResourceNotFound:
		slog.InfoContext(ctx, ae.LogString())
		h.errorRenderer.NotFound(w, r)
	case model.AppErrCodeResourceUnpublished:
		slog.InfoContext(ctx, ae.LogString())
		h.errorRenderer.Unpublished(w, r)
	default:
		slog.ErrorContext(ctx, ae.LogString())
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
	return true
}
