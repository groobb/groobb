package community

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/templates"
	"github.com/groobb/groobb/go/internal/templates/layouts"
	communitypage "github.com/groobb/groobb/go/internal/templates/pages/community"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// Show GET /c/{identifier} - renders the page of the community the identifier
// addresses, which for now carries its name. An identifier nobody has claimed is
// answered with 404 rather than a redirect elsewhere, so a shared link to a
// community that never existed says so instead of landing the visitor on an
// unrelated page. It is registered behind RequireAuth, so the visitor is signed
// in; the page shows the same content to every one of them, since membership does
// not gate the view yet. Being behind authentication, it is marked noindex.
//
// [Ja] Show GET /c/{identifier} - 識別子が指すコミュニティの画面を描画します。現時点では
// コミュニティ名を載せます。誰も取得していない識別子は、別の場所へのリダイレクトではなく
// 404 で応答し、存在しなかったコミュニティへの共有リンクが訪問者を無関係なページに着地
// させず、その旨を伝えるようにします。RequireAuth の背後に登録されるため訪問者はサイン
// イン済みであり、閲覧はまだ所属で制限しないため、そのすべてに同じ内容を見せます。認証の
// 背後にあるため noindex を付けます。
func (h *Handler) Show(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	identifier := chi.URLParam(r, "identifier")

	output, err := h.getCommunityUC.Execute(ctx, usecase.GetCommunityInput{Identifier: identifier})
	if err != nil {
		var ae *model.AppError
		if errors.As(err, &ae) {
			if ae.Code == model.AppErrCodeResourceNotFound {
				http.Error(w, ae.UserMsg, http.StatusNotFound)
				return
			}

			slog.ErrorContext(ctx, ae.LogString())
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		slog.ErrorContext(ctx, "コミュニティの取得に失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	community := output.Community

	// The identifier column is citext, so /c/Groobb and /c/groobb open the same
	// community. Serving both would give one community several URLs, splitting
	// how a shared link is bookmarked, linked, and measured. Send every other
	// spelling to the one the community was created with, permanently: the
	// canonical spelling of an identifier does not change while the community
	// holds it. The query rides along, so the campaign parameters a shared link
	// carries reach the canonical URL instead of being dropped on the way.
	//
	// [Ja] identifier 列は citext のため、/c/Groobb と /c/groobb は同じコミュニティを
	// 開く。両方をそのまま配信すると 1 つのコミュニティが複数の URL を持ち、共有リンクの
	// ブックマーク・被リンク・計測が分散する。それ以外の表記は、コミュニティが作成された
	// ときの表記へ恒久的に送る。識別子の正規の表記は、そのコミュニティが保持している間は
	// 変わらないためである。クエリは一緒に運び、共有リンクが載せてきた計測パラメータが
	// 途中で落ちずに正規の URL へ届くようにする。
	if community.Identifier != identifier {
		target := templates.CommunityPath(community.Identifier).String()
		if r.URL.RawQuery != "" {
			target += "?" + r.URL.RawQuery
		}

		http.Redirect(w, r, target, http.StatusMovedPermanently)
		return
	}

	meta := viewmodel.DefaultPageMeta(ctx, h.cfg)
	meta.Title = community.Name
	meta.Description = i18n.T(ctx, "community_show_description", map[string]any{"Name": community.Name})
	meta.NoIndex = true

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if err := layouts.Default(meta, communitypage.Show(communitypage.ShowPageData{
		Name: community.Name,
	})).Render(ctx, w); err != nil {
		// The status and headers are already sent, so this can only be logged,
		// not turned into a 500.
		//
		// [Ja] ステータスとヘッダーは既に送出済みのため、ここでは 500 に変えられず
		// ログに記録するのみとする。
		slog.ErrorContext(ctx, "コミュニティ画面のレンダリングに失敗", "error", err)
	}
}
