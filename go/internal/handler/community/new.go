package community

import (
	"log/slog"
	"net/http"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/templates/layouts"
	communitypage "github.com/groobb/groobb/go/internal/templates/pages/community"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// New GET /communities/new - renders the community-creation form with the CSRF
// token the middleware placed in the context. It is registered behind
// RequireAuth, which guarantees a signed-in user, so the handler does not
// resolve one itself. The page is a form behind authentication with nothing to
// offer a search result, so it is marked noindex.
//
// [Ja] New GET /communities/new - ミドルウェアが context に置いた CSRF トークン付きで
// コミュニティ作成フォームを描画します。RequireAuth の背後に登録され、サインイン済み
// ユーザーが保証されるため、ハンドラー自身はユーザーを解決しません。このページは認証の
// 背後にあるフォームで検索結果に出す内容を持たないため、noindex を付けます。
func (h *Handler) New(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	h.renderNew(w, r, http.StatusOK, communitypage.NewPageData{
		CSRFToken: middleware.CSRFTokenFromContext(ctx),
	})
}

// renderNew renders the community-creation form with the given status and data.
// It is shared by New (200) and Create's re-render after a validation error
// (422). The status is written before rendering, so callers pass the final
// status here rather than setting it separately.
//
// [Ja] renderNew は指定したステータスとデータでコミュニティ作成フォームを描画します。
// New (200) と、Create のバリデーションエラー後の再描画 (422) で共有します。ステータスは
// 描画前に書き込むため、呼び出し側は別途設定せずここに最終ステータスを渡します。
func (h *Handler) renderNew(w http.ResponseWriter, r *http.Request, status int, data communitypage.NewPageData) {
	ctx := r.Context()

	meta := viewmodel.DefaultPageMeta(ctx, h.cfg)
	meta.Title = i18n.T(ctx, "community_new_title")
	meta.Description = i18n.T(ctx, "community_new_description")
	meta.NoIndex = true

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := layouts.Default(meta, communitypage.New(data)).Render(ctx, w); err != nil {
		// The status and headers are already sent, so this can only be logged,
		// not turned into a 500.
		//
		// [Ja] ステータスとヘッダーは既に送出済みのため、ここでは 500 に変えられず
		// ログに記録するのみとする。
		slog.ErrorContext(ctx, "コミュニティ作成ページのレンダリングに失敗", "error", err)
	}
}
