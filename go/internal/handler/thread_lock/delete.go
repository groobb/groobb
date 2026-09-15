package thread_lock

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/templates"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// Delete DELETE /t/{id}/lock - アドレスが名指すスレッドに管理者が掛けたロックを外し、
// そのスレッド自身で応答します。ルートはスレッドのページから_method=DELETEのオーバーライドで
// 到達し、隣の2つと同じくRequireAuthの背後に登録されます。
//
// 確認ページを持たず、注記も受け取りません。外すことはスレッドを元の姿に戻すことであり、
// 事前に量るべきものがないためです。管理者のロックを持たないスレッドには成功で応答します。
// 要求が求めたのはロックが成立していないことであり、実際に成立していないためです。
//
// そのスレッドが実際に返信を受け付けるかどうかは別の問いです。投稿数の上限に達したスレッドは
// その理由で閉じたままであり、訪問者が降り立つスレッドがそのことを述べます。
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, ok := model.ParseThreadID(chi.URLParam(r, "id"))
	if !ok {
		h.errorRenderer.NotFound(w, r)
		return
	}

	actor := middleware.UserFromContext(ctx)
	if actor == nil {
		slog.ErrorContext(ctx, "スレッドのロックの解除にユーザー無しで到達")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if err := h.unlockThreadUC.Execute(ctx, usecase.UnlockThreadInput{
		Actor:    usecase.UserActor(actor.ID),
		ThreadID: id,
	}); err != nil {
		if h.refused(w, r, err) {
			return
		}
		slog.ErrorContext(ctx, "スレッドのロックの解除に失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.flashMgr.SetSuccess(w, i18n.T(ctx, "flash_thread_unlocked"))
	http.Redirect(w, r, templates.ThreadPath(viewmodel.ThreadID(id)).String(), http.StatusSeeOther)
}
