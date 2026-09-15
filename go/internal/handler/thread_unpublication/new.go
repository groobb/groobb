package thread_unpublication

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/templates/layouts"
	threadunpublicationpage "github.com/groobb/groobb/go/internal/templates/pages/thread_unpublication"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// New GET /t/{id}/unpublication/new - スレッドの非公開を確認するページを描画します。
// これからコミュニティの視界を去るスレッドと、履歴が保つことになる注記です。RequireAuthの
// 背後に登録されるため、誰かがサインインしています。その誰かがそもそもスレッドに対して働き
// かけてよいかどうかを決めるのはUseCaseで、拒否には共通の403ページで応答します。
//
// idは何かを読む前に検査し、同じidの正規でない綴りは、ロックの確認ページと同じくリダイレクト
// しません。このページは認証の背後にありnoindexであるため、誰かが見つけうる2つ目のアドレスは
// 無く、描かれるフォームがスレッドを名指すのも、辿られたパスではなく読み取られたidです。
func (h *Handler) New(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, ok := model.ParseThreadID(chi.URLParam(r, "id"))
	if !ok {
		h.errorRenderer.NotFound(w, r)
		return
	}

	actor := middleware.UserFromContext(ctx)
	if actor == nil {
		slog.ErrorContext(ctx, "スレッドの非公開の確認ページにユーザー無しで到達")
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
		slog.ErrorContext(ctx, "スレッドの非公開の確認ページのスレッドの取得に失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.renderNew(w, r, http.StatusOK, threadunpublicationpage.NewPageData{
		ThreadID:  viewmodel.ThreadID(resolved.Thread.ID),
		Title:     resolved.Thread.Title,
		Language:  viewmodel.NewThreadLanguage(resolved.Thread.Language),
		CSRFToken: middleware.CSRFTokenFromContext(ctx),
	})
}

// renderNewは、指定したステータスとデータで確認ページを描画します。New (200) と、
// 長さを理由に拒否された注記の後のCreateの再描画 (422) で共有します。ステータスは描画前に
// 書き込むため、呼び出し側は別途設定せずここに最終ステータスを渡します。
//
// このページにはnoindexを付けます。認証の背後にあり、許されるのは数人であり、ここに立って
// いるのは見つけるべき何かではなくボタンであるためです。
func (h *Handler) renderNew(w http.ResponseWriter, r *http.Request, status int, data threadunpublicationpage.NewPageData) {
	ctx := r.Context()

	meta := viewmodel.DefaultPageMeta(ctx, h.cfg)
	meta.Title = i18n.T(ctx, "thread_unpublication_new_title")
	meta.Description = i18n.T(ctx, "thread_unpublication_new_description")
	meta.NoIndex = true
	meta.SignedIn = true

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := layouts.Default(meta, threadunpublicationpage.New(data)).Render(ctx, w); err != nil {
		// ステータスとヘッダーは既に送出済みのため、ここでは500に変えられずログに
		// 記録するのみとする。
		slog.ErrorContext(ctx, "スレッドの非公開の確認ページのレンダリングに失敗", "error", err)
	}
}
