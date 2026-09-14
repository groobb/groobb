package thread_lock

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/templates/layouts"
	threadlockpage "github.com/groobb/groobb/go/internal/templates/pages/thread_lock"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// New GET /t/{id}/lock/new - renders the page a thread's lock is confirmed on:
// the thread about to be closed, and the note the history will keep. It is
// registered behind RequireAuth, so someone is signed in; whether that someone
// may act on a thread at all is settled by the UseCase, and a refusal is
// answered with the shared 403 page.
//
// The id is checked before anything is read, as it is on the thread's own page.
// A spelling of the same id that is not the canonical one is not redirected
// here, unlike there: this page is behind authentication and marked noindex, so
// there is no second address for anyone to find, and the form it draws names
// the thread by the id that was parsed rather than by the path that was walked.
//
// [Ja] New GET /t/{id}/lock/new - スレッドのロックを確認するページを描画します。これから
// 閉じられるスレッドと、履歴が保つことになる注記です。RequireAuthの背後に登録されるため、
// 誰かがサインインしています。その誰かがそもそもスレッドに対して働きかけてよいかどうかを
// 決めるのはUseCaseで、拒否には共通の403ページで応答します。
//
// idは、スレッド自身のページと同じく、何かを読む前に検査します。同じidの正規でない綴りを
// あちらと違ってリダイレクトしないのは、このページが認証の背後にありnoindexであるためです。
// 誰かが見つけうる2つ目のアドレスは無く、描かれるフォームがスレッドを名指すのも、辿られた
// パスではなく読み取られたidです。
func (h *Handler) New(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, ok := model.ParseThreadID(chi.URLParam(r, "id"))
	if !ok {
		h.errorRenderer.NotFound(w, r)
		return
	}

	actor := middleware.UserFromContext(ctx)
	if actor == nil {
		slog.ErrorContext(ctx, "ロックの確認ページにユーザー無しで到達")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	resolved, err := h.getModerationUC.Execute(ctx, usecase.GetThreadModerationInput{
		Actor:    usecase.UserActor(actor.ID),
		ThreadID: id,
	})
	if err != nil {
		if h.refused(w, r, err) {
			return
		}
		slog.ErrorContext(ctx, "ロックの確認ページのスレッドの取得に失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.renderNew(w, r, http.StatusOK, threadlockpage.NewPageData{
		ThreadID:  viewmodel.ThreadID(resolved.Thread.ID),
		Title:     resolved.Thread.Title,
		Language:  viewmodel.NewThreadLanguage(resolved.Thread.Language),
		CSRFToken: middleware.CSRFTokenFromContext(ctx),
	})
}

// renderNew renders the confirmation page with the given status and data. It is
// shared by New (200) and Create's re-render after a note that was refused for
// its length (422). The status is written before rendering, so callers pass the
// final status here rather than setting it separately.
//
// The page is marked noindex: it is behind authentication, admitted to a few
// people, and what stands on it is a button rather than anything to find.
//
// [Ja] renderNewは、指定したステータスとデータで確認ページを描画します。New (200) と、
// 長さを理由に拒否された注記の後のCreateの再描画 (422) で共有します。ステータスは描画前に
// 書き込むため、呼び出し側は別途設定せずここに最終ステータスを渡します。
//
// このページにはnoindexを付けます。認証の背後にあり、許されるのは数人であり、ここに立って
// いるのは見つけるべき何かではなくボタンであるためです。
func (h *Handler) renderNew(w http.ResponseWriter, r *http.Request, status int, data threadlockpage.NewPageData) {
	ctx := r.Context()

	meta := viewmodel.DefaultPageMeta(ctx, h.cfg)
	meta.Title = i18n.T(ctx, "thread_lock_new_title")
	meta.Description = i18n.T(ctx, "thread_lock_new_description")
	meta.NoIndex = true
	meta.SignedIn = true

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := layouts.Default(meta, threadlockpage.New(data)).Render(ctx, w); err != nil {
		// The status and headers are already sent, so this can only be logged,
		// not turned into a 500.
		//
		// [Ja] ステータスとヘッダーは既に送出済みのため、ここでは500に変えられずログに
		// 記録するのみとする。
		slog.ErrorContext(ctx, "ロックの確認ページのレンダリングに失敗", "error", err)
	}
}
