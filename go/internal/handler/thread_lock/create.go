package thread_lock

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/templates"
	threadlockpage "github.com/groobb/groobb/go/internal/templates/pages/thread_lock"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// Create POST /t/{id}/lock - closes the thread the address names to new
// replies and answers with the thread itself, which now states that it is
// closed. The CSRF check is enforced upstream by the middleware, so it is not
// repeated here.
//
// Who is locking is the account the session names. The submission carries no
// field for it and none would be read: a form saying who is locking would let
// anyone lock as an administrator.
//
// A note that is too long comes back on the confirmation page holding what was
// written, since it is something the administrator can shorten. Every other
// refusal is answered with a page, because there is then nothing to come back
// to: the thread is not one this visitor may act on, or not one the community
// still shows.
//
// [Ja] Create POST /t/{id}/lock - アドレスが名指すスレッドを新しい返信に対して締め切り、
// そのスレッド自身で応答します。スレッドは今や、閉じられていることを述べます。CSRFの検証は
// 上流のミドルウェアが強制するため、ここでは繰り返しません。
//
// ロックする側はセッションが名指すアカウントです。送信はそのためのフィールドを持たず、あっても
// 読みません。誰がロックするのかを述べるフォームは、誰もが管理者としてロックできることを
// 意味するためです。
//
// 長すぎる注記は、書かれたものを保ったまま確認ページに戻ってきます。管理者が短くできるもので
// あるためです。それ以外の拒否にはページで応答します。そのときに戻るべき先は無いためです。
// スレッドがこの訪問者の働きかけてよいものではないか、コミュニティのまだ示しているもので
// ないかです。
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, ok := model.ParseThreadID(chi.URLParam(r, "id"))
	if !ok {
		h.errorRenderer.NotFound(w, r)
		return
	}

	actor := middleware.UserFromContext(ctx)
	if actor == nil {
		slog.ErrorContext(ctx, "スレッドのロックにユーザー無しで到達")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// The note's line endings are settled here, where the submission is read, so
	// that the value recorded and the value drawn back both come out of one
	// normalization.
	//
	// [Ja] 注記の改行は、送信を読むこの場所で確定させる。記録される値と描き戻される値の
	// どちらも1回の正規化から出るようにするためである。
	reason := model.NormalizeLineBreaks(r.PostFormValue("reason"))

	if err := h.lockThreadUC.Execute(ctx, usecase.LockThreadInput{
		Actor:    usecase.UserActor(actor.ID),
		ThreadID: id,
		Reason:   reason,
	}); err != nil {
		h.createRefused(w, r, id, reason, err)
		return
	}

	h.flashMgr.SetSuccess(w, i18n.T(ctx, "flash_thread_locked"))
	http.Redirect(w, r, templates.ThreadPath(viewmodel.ThreadID(id)).String(), http.StatusSeeOther)
}

// createRefused answers a lock that was not placed, turning what the UseCase
// refused it for into the response the visitor gets.
//
// The confirmation page is read again to be drawn again, because it names the
// thread and the submission carries only the note. The read goes through the
// same UseCase the page was opened with, so a thread that was taken out of view
// between opening the page and submitting is answered the way it would be on
// the way in rather than drawn as a target that is no longer one.
//
// [Ja] createRefusedは、掛けられなかったロックに応答し、UseCaseが何を理由に拒否したかを
// 訪問者が受け取る応答に変えます。
//
// 確認ページを描き直すために読み直すのは、そのページがスレッドを名指す一方、送信が運ぶのが
// 注記だけであるためです。読み取りは、ページが開かれたときと同じUseCaseを通ります。ページを
// 開いてから送信するまでの間に視界の外へ移されたスレッドは、もう対象ではないものを対象として
// 描かれるのではなく、入るときと同じ答えを受け取ります。
func (h *Handler) createRefused(w http.ResponseWriter, r *http.Request, id model.ThreadID, reason string, err error) {
	ctx := r.Context()

	ve := model.AsValidationError(err)
	if ve == nil {
		if h.refused(w, r, err) {
			return
		}
		slog.ErrorContext(ctx, "スレッドのロックに失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	actor := middleware.UserFromContext(ctx)
	resolved, readErr := h.getModerationUC.Execute(ctx, usecase.GetThreadModerationInput{
		Actor:    usecase.UserActor(actor.ID),
		ThreadID: id,
	})
	if readErr != nil {
		if h.refused(w, r, readErr) {
			return
		}
		slog.ErrorContext(ctx, "ロックの確認ページの再描画のためのスレッドの取得に失敗", "error", readErr)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.renderNew(w, r, http.StatusUnprocessableEntity, threadlockpage.NewPageData{
		ThreadID:   viewmodel.ThreadID(resolved.Thread.ID),
		Title:      resolved.Thread.Title,
		Language:   viewmodel.NewThreadLanguage(resolved.Thread.Language),
		Reason:     reason,
		CSRFToken:  middleware.CSRFTokenFromContext(ctx),
		FormErrors: ve,
	})
}
