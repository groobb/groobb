// Package thread_lock provides the handlers for the lock a moderator puts on a
// thread: the confirmation page (GET /t/{id}/lock/new), placing the lock
// (POST /t/{id}/lock), and lifting it (DELETE /t/{id}/lock). All three are
// behind RequireAuth, and which of them a signed-in visitor may carry out is
// settled by the UseCase each of them calls.
//
// Placing and lifting share one address because they act on the one lock the
// thread carries. Lifting has no confirmation page of its own: it restores what
// the thread was before, so there is nothing to weigh beforehand and no note for
// the history to keep.
//
// [Ja] thread_lockパッケージは、モデレーターがスレッドに掛けるロックのハンドラーを提供
// します。確認ページ (GET /t/{id}/lock/new)、ロックを掛けること (POST /t/{id}/lock)、
// そしてそれを外すこと (DELETE /t/{id}/lock) です。3つともRequireAuthの背後にあり、
// サインイン済みの訪問者がそのどれを行ってよいかは、それぞれが呼ぶUseCaseが決めます。
//
// 掛けることと外すことが1つのアドレスを共有するのは、どちらもスレッドが持つ1つのロックに
// 対して行われるためです。外すことに専用の確認ページはありません。それはスレッドを元の姿に
// 戻すことであり、事前に量るべきものも、履歴が保つ注記もないためです。
package thread_lock

import (
	"log/slog"
	"net/http"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/httperror"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/usecase"
)

// Handler is the HTTP handler for a thread's lock. It holds the shared error
// renderer because all three routes answer the same three refusals with a page
// rather than with the thread: a visitor who may not moderate, an address
// naming no thread, and a thread the community has taken out of view. The flash
// manager carries what happened across the redirect, since the answer to an
// operation that did land is the thread read again.
//
// The confirmation page reads through a UseCase of its own rather than through
// the one the thread's page uses, because what it needs is the thread and the
// permission to act on it, not the conversation under it.
//
// [Ja] HandlerはスレッドのロックのHTTPハンドラーです。共通のエラーRendererを保持するのは、
// 3つのルートがいずれも同じ3種類の拒否に、スレッドではなくページで応答するためです。
// モデレーションを許されていない訪問者、どのスレッドも名指していないアドレス、そして
// コミュニティが視界の外へ移したスレッドです。フラッシュManagerは、何が起きたのかを
// リダイレクトの先へ運びます。届いた操作への答えは、読み直されたスレッドであるためです。
//
// 確認ページが、スレッドのページが使うUseCaseではなく専用のUseCaseで読むのは、それが必要と
// するのがスレッドとそれに働きかける権限であって、その下の会話ではないためです。
type Handler struct {
	cfg             *config.Config
	errorRenderer   *httperror.Renderer
	flashMgr        *session.FlashManager
	getModerationUC *usecase.GetThreadModerationUsecase
	lockThreadUC    *usecase.LockThreadUsecase
	unlockThreadUC  *usecase.UnlockThreadUsecase
}

// NewHandler creates a new thread_lock Handler.
//
// [Ja] NewHandlerは新しいthread_lock Handlerを作成します。
func NewHandler(
	cfg *config.Config,
	errorRenderer *httperror.Renderer,
	flashMgr *session.FlashManager,
	getModerationUC *usecase.GetThreadModerationUsecase,
	lockThreadUC *usecase.LockThreadUsecase,
	unlockThreadUC *usecase.UnlockThreadUsecase,
) *Handler {
	return &Handler{
		cfg:             cfg,
		errorRenderer:   errorRenderer,
		flashMgr:        flashMgr,
		getModerationUC: getModerationUC,
		lockThreadUC:    lockThreadUC,
		unlockThreadUC:  unlockThreadUC,
	}
}

// refused answers a request the UseCase did not carry out, turning what it
// refused for into the response the visitor gets.
//
// The three known refusals are answered with a page, because none of them is
// something the thread's lock screen can say anything about: the visitor may not
// be here, the address names no thread, or the thread is no longer shown. It
// reports whether it answered, so a caller with a refusal of its own to handle —
// a note that is too long, which comes back on the form — asks this first and
// draws the form when the answer was not settled here.
//
// [Ja] refusedは、UseCaseが実行しなかった要求に応答し、拒否の理由を訪問者が受け取る応答へ
// 変えます。
//
// 既知の3つの拒否にはページで応答します。どれもスレッドのロックの画面が何かを述べられる
// ものではないためです。訪問者がここに居てはならないか、アドレスがどのスレッドも名指して
// いないか、スレッドがもう示されていないかです。応答したかどうかを返すのは、自身で扱う拒否を
// 持つ呼び出し元 (長すぎる注記。これはフォームに載って戻ってきます) が、まずこれに尋ね、
// ここで答えが決まらなかったときにフォームを描けるようにするためです。
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
