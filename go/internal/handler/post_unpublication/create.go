package post_unpublication

import (
	"log/slog"
	"net/http"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/templates"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// Create POST /t/{id}/posts/{number}/unpublication - アドレスが名指す投稿を視界から
// 外し、それが書かれたスレッドで応答します。スレッドは今、投稿が立っていた場所に占位を運んで
// います。CSRFの検証は上流のミドルウェアが強制するため、ここでは繰り返しません。
//
// スレッドで応答するのは、投稿のアンカーではなくスレッド自身のアドレスでです。管理者に伝えら
// れることを運ぶのはページ上端のフラッシュであり、投稿の番号で終わるアドレスは、その知らせを
// 画面の外へ送ったままスレッドを開くことになるためです。
//
// 長すぎる注記は、書かれたものを保ったまま確認ページに戻ってきます。管理者が短くできるもので
// あるためです。それ以外の拒否にはページで応答します。そのときに戻るべき先は無いためです。
// 投稿がこの訪問者の働きかけてよいものではないか、スレッドのまだ示しているものでないかです。
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, number, ok := h.target(w, r)
	if !ok {
		return
	}

	actor := middleware.UserFromContext(ctx)
	if actor == nil {
		slog.ErrorContext(ctx, "投稿の非公開にユーザー無しで到達")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// 注記の改行は、送信を読むこの場所で確定させる。記録される値と描き戻される値の
	// どちらも1回の正規化から出るようにするためである。
	reason := model.NormalizeLineBreaks(r.PostFormValue("reason"))

	if err := h.unpublishPostUC.Execute(ctx, usecase.UnpublishPostInput{
		Actor:    usecase.UserActor(actor.ID),
		ThreadID: id,
		Number:   number,
		Reason:   reason,
	}); err != nil {
		h.createRefused(w, r, id, number, reason, err)
		return
	}

	h.flashMgr.SetSuccess(w, i18n.T(ctx, "flash_post_unpublished"))
	http.Redirect(w, r, templates.ThreadPath(viewmodel.ThreadID(id)).String(), http.StatusSeeOther)
}

// createRefusedは、行われなかった非公開に応答し、UseCaseが何を理由に拒否したかを
// 訪問者が受け取る応答に変えます。
//
// 確認ページを描き直すために読み直すのは、そのページが投稿を名指す一方、送信が運ぶのが注記
// だけであるためです。読み取りは、ページが開かれたときと同じUseCaseを通ります。ページを開いて
// から送信するまでの間に視界の外へ移された投稿は、もう対象ではないものを対象として描かれるので
// はなく、入るときと同じ答えを受け取ります。
func (h *Handler) createRefused(w http.ResponseWriter, r *http.Request, id model.ThreadID, number int, reason string, err error) {
	ctx := r.Context()

	ve := model.AsValidationError(err)
	if ve == nil {
		if h.refused(w, r, err) {
			return
		}
		slog.ErrorContext(ctx, "投稿の非公開に失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	actor := middleware.UserFromContext(ctx)
	resolved, readErr := h.getModerationUC.Execute(ctx, usecase.GetThreadModerationInput{
		Actor:    usecase.UserActor(actor.ID),
		ThreadID: id,
		Number:   &number,
	})
	if readErr != nil {
		if h.refused(w, r, readErr) {
			return
		}
		slog.ErrorContext(ctx, "投稿の非公開の確認ページの再描画のための投稿の取得に失敗", "error", readErr)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.renderNew(w, r, http.StatusUnprocessableEntity, newPageData(ctx, resolved, reason, ve))
}
