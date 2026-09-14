// Package post_unpublication provides the handlers for taking one post out of
// view while the thread around it stays as it was: the confirmation page
// (GET /t/{id}/posts/{number}/unpublication/new) and the unpublication itself
// (POST /t/{id}/posts/{number}/unpublication). Both are behind RequireAuth, and
// whether a signed-in visitor may carry it out is settled by the UseCase each of
// them calls.
//
// The post is addressed by its thread and its reply number, which is how it is
// named everywhere it is referred to (ADR 0009). It keeps that number under the
// mark, so the thread is left with a placeholder where the post stood rather
// than with a gap in its numbering.
//
// [Ja] post_unpublicationパッケージは、周りのスレッドをそのままにしたまま投稿1件を視界から
// 外すためのハンドラーを提供します。確認ページ
// (GET /t/{id}/posts/{number}/unpublication/new) と、非公開そのもの
// (POST /t/{id}/posts/{number}/unpublication) です。どちらもRequireAuthの背後にあり、
// サインイン済みの訪問者がそれを行ってよいかどうかは、それぞれが呼ぶUseCaseが決めます。
//
// 投稿はスレッドとレス番号で名指します。それが、投稿が参照されるあらゆる場所での名指し方だから
// です (ADR 0009)。投稿は印の下でもその番号を保つため、スレッドに残るのは番号の抜けではなく、
// 投稿が立っていた場所の占位です。
package post_unpublication

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/httperror"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/usecase"
)

// Handler is the HTTP handler for a post's unpublication. It holds the shared
// error renderer because both routes answer the same three refusals with a page
// rather than with the thread: a visitor who may not moderate, an address naming
// no post the thread still shows, and a thread the community has taken out of
// view. The flash manager carries what happened across the redirect, since the
// answer to an operation that did land is the thread read again.
//
// [Ja] Handlerは投稿のHTTPハンドラーで、その非公開を扱います。共通のエラーRendererを保持
// するのは、2つのルートがいずれも同じ3種類の拒否に、スレッドではなくページで応答するため
// です。モデレーションを許されていない訪問者、スレッドがまだ示している投稿をどれも名指して
// いないアドレス、そしてコミュニティが視界の外へ移したスレッドです。フラッシュManagerは、
// 何が起きたのかをリダイレクトの先へ運びます。届いた操作への答えは、読み直されたスレッドで
// あるためです。
type Handler struct {
	cfg             *config.Config
	errorRenderer   *httperror.Renderer
	flashMgr        *session.FlashManager
	getModerationUC *usecase.GetThreadModerationUsecase
	unpublishPostUC *usecase.UnpublishPostUsecase
}

// NewHandler creates a new post_unpublication Handler.
//
// [Ja] NewHandlerは新しいpost_unpublication Handlerを作成します。
func NewHandler(
	cfg *config.Config,
	errorRenderer *httperror.Renderer,
	flashMgr *session.FlashManager,
	getModerationUC *usecase.GetThreadModerationUsecase,
	unpublishPostUC *usecase.UnpublishPostUsecase,
) *Handler {
	return &Handler{
		cfg:             cfg,
		errorRenderer:   errorRenderer,
		flashMgr:        flashMgr,
		getModerationUC: getModerationUC,
		unpublishPostUC: unpublishPostUC,
	}
}

// target reads the pair that names the post out of the address, answering an
// address that spells neither with the 404 page. Both are checked before
// anything is read, as the thread's id is on the thread's own page.
//
// The two are read together because neither names a post on its own: a reply
// number belongs to one thread, and the same number in another thread is another
// post. It reports whether it read them, so the caller stops at the answer this
// has already written.
//
// [Ja] targetは、投稿を名指す組をアドレスから読み取り、そのどちらも綴っていないアドレスには
// 404ページで応答します。どちらも、スレッド自身のページでスレッドのidがそうされるのと同じく、
// 何かを読む前に検査します。
//
// 2つを一緒に読むのは、どちらも単独では投稿を名指さないためです。レス番号は1つのスレッドに
// 属し、別のスレッドの同じ番号は別の投稿です。読み取れたかどうかを返すのは、呼び出し元が、
// ここが既に書いた応答で止まれるようにするためです。
func (h *Handler) target(w http.ResponseWriter, r *http.Request) (model.ThreadID, int, bool) {
	id, ok := model.ParseThreadID(chi.URLParam(r, "id"))
	if !ok {
		h.errorRenderer.NotFound(w, r)
		return 0, 0, false
	}

	number, ok := model.ParsePostNumber(chi.URLParam(r, "number"))
	if !ok {
		h.errorRenderer.NotFound(w, r)
		return 0, 0, false
	}

	return id, number, true
}

// refused answers a request the UseCase did not carry out, turning what it
// refused for into the response the visitor gets.
//
// The three known refusals are answered with a page, because none of them is
// something the unpublication screen can say anything about: the visitor may not
// be here, the thread shows no such post, or the thread itself is no longer
// shown. It reports whether it answered, so a caller with a refusal of its own
// to handle — a note that is too long, which comes back on the form — asks this
// first and draws the form when the answer was not settled here.
//
// [Ja] refusedは、UseCaseが実行しなかった要求に応答し、拒否の理由を訪問者が受け取る応答へ
// 変えます。
//
// 既知の3つの拒否にはページで応答します。どれも非公開の画面が何かを述べられるものではない
// ためです。訪問者がここに居てはならないか、スレッドがそのような投稿を示していないか、
// スレッド自身がもう示されていないかです。応答したかどうかを返すのは、自身で扱う拒否を持つ
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
