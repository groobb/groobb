package post_unpublication

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/templates/layouts"
	postunpublicationpage "github.com/groobb/groobb/go/internal/templates/pages/post_unpublication"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// New GET /t/{id}/posts/{number}/unpublication/new - 投稿の非公開を確認するページを
// 描画します。これから視界から外される投稿を、スレッドが示すとおりに示し、履歴が保つことに
// なる注記を添えます。RequireAuthの背後に登録されるため、誰かがサインインしています。その
// 誰かがそもそもスレッドに対して働きかけてよいかどうかを決めるのはUseCaseで、拒否には共通の
// 403ページで応答します。
//
// 既に視界の外にある投稿には、スレッドが一度も持たなかった投稿と同じ答えを返します。この
// ページが管理者の前に置くのは投稿の本文であり、そこに付いた印はその本文を持ち去っている
// ため、ここに判断すべきものは無いからです。
func (h *Handler) New(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, number, ok := h.target(w, r)
	if !ok {
		return
	}

	actor := middleware.UserFromContext(ctx)
	if actor == nil {
		slog.ErrorContext(ctx, "投稿の非公開の確認ページにユーザー無しで到達")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	resolved, err := h.getModerationUC.Execute(ctx, usecase.GetThreadModerationInput{
		Actor:    usecase.UserActor(actor.ID),
		ThreadID: id,
		Number:   &number,
	})
	if err != nil {
		if h.refused(w, r, err) {
			return
		}
		slog.ErrorContext(ctx, "投稿の非公開の確認ページの投稿の取得に失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.renderNew(w, r, http.StatusOK, newPageData(ctx, resolved, "", nil))
}

// newPageDataは、確認ページが描かれる元を、UseCaseが解決したものから組み立てます。
// Newと、Createの再描画がこれを共有するため、注記が戻ってくるページは、それが書かれたページと
// 同じ形で投稿を名指します。
func newPageData(
	ctx context.Context,
	resolved *usecase.GetThreadModerationOutput,
	reason string,
	formErrors *model.ValidationError,
) postunpublicationpage.NewPageData {
	return postunpublicationpage.NewPageData{
		ThreadID:   viewmodel.ThreadID(resolved.Thread.ID),
		Number:     resolved.Post.Number,
		Author:     authorAtname(resolved.PostAuthor),
		Body:       resolved.Post.Body,
		Reason:     reason,
		CSRFToken:  middleware.CSRFTokenFromContext(ctx),
		FormErrors: formErrors,
	}
}

// authorAtnameは投稿の作者が名乗っている名前を返し、名指すアカウントが無いときは""を
// 返します。ページはその行を空にするのではなく作者が退会した旨を述べるため、描かれるのは空白
// ではなく不在です。
func authorAtname(author *model.User) string {
	if author == nil {
		return ""
	}
	return author.Atname
}

// renderNewは、指定したステータスとデータで確認ページを描画します。New (200) と、
// 長さを理由に拒否された注記の後のCreateの再描画 (422) で共有します。ステータスは描画前に
// 書き込むため、呼び出し側は別途設定せずここに最終ステータスを渡します。
//
// このページにはnoindexを付けます。認証の背後にあり、許されるのは数人であり、ここに立って
// いるのは見つけるべき何かではなくボタンであるためです。
func (h *Handler) renderNew(w http.ResponseWriter, r *http.Request, status int, data postunpublicationpage.NewPageData) {
	ctx := r.Context()

	meta := viewmodel.DefaultPageMeta(ctx, h.cfg)
	meta.Title = i18n.T(ctx, "post_unpublication_new_title")
	meta.Description = i18n.T(ctx, "post_unpublication_new_description")
	meta.NoIndex = true
	meta.SignedIn = true

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := layouts.Default(meta, postunpublicationpage.New(data)).Render(ctx, w); err != nil {
		// ステータスとヘッダーは既に送出済みのため、ここでは500に変えられずログに
		// 記録するのみとする。
		slog.ErrorContext(ctx, "投稿の非公開の確認ページのレンダリングに失敗", "error", err)
	}
}
