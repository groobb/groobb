package category

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/groobb/groobb/go/internal/httpredirect"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/templates"
	"github.com/groobb/groobb/go/internal/templates/layouts"
	categorypage "github.com/groobb/groobb/go/internal/templates/pages/category"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// Show GET /c/{slug} - renders a category: the boards it groups in the list
// column, and the sidebar the whole community shell carries. The page is
// readable while signed out, so it is registered behind SetUser rather than
// RequireAuth and the user from the context may be nil; the sidebar then offers
// sign-in and sign-up in place of the account controls. A category that lists
// at least one board carries no noindex, since a community's categories are what
// its public pages are reached through; one that lists none does, for the reason
// given where it is set.
//
// The category is resolved on its own first, because that one read settles
// whether the page is going to be rendered at all: a slug naming no category is
// answered with the 404 page, and a case variant that resolves through the
// database's NOCASE collation is redirected to the stored lowercase slug,
// keeping one canonical URL for the category. Neither answer runs the sidebar's
// queries or the board listing, which would be read and thrown away.
//
// [Ja] Show GET /c/{slug} - カテゴリーを描画します。一覧カラムにそれがまとめる掲示板を、
// そしてコミュニティのシェルがどこでも運ぶサイドバーを描きます。このページはサインアウト
// 状態でも読めるため、RequireAuth ではなく SetUser の背後に登録され、context のユーザーは
// nil でありえます。その場合サイドバーはアカウント操作の代わりにサインインと新規登録を
// 差し出します。掲示板を 1 つ以上並べるカテゴリーに noindex は付けません。コミュニティの
// カテゴリーは、その公開ページへ辿り着く手立てだからです。1 つも並べないカテゴリーには
// 付けます。理由はそれを設定している箇所に書いています。
//
// はじめにカテゴリーだけを解決するのは、その 1 回の読み取りでそもそもページを描画するか
// どうかが決まるためです。どのカテゴリーも指さない slug には 404 ページで応答し、DB の
// NOCASE 照合で解決できる大文字小文字違いの slug は保存済みの小文字 slug へリダイレクト
// して、カテゴリーの正規 URL を 1 つに保ちます。どちらの応答も、読んで捨てることになる
// サイドバーのクエリと掲示板の一覧を走らせません。
func (h *Handler) Show(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	slug := chi.URLParam(r, "slug")

	resolved, err := h.getCategoryUC.Execute(ctx, usecase.GetCategoryInput{Slug: slug})
	if err != nil {
		var ae *model.AppError
		if errors.As(err, &ae) && ae.Code == model.AppErrCodeResourceNotFound {
			h.errorRenderer.NotFound(w, r)
			return
		}
		slog.ErrorContext(ctx, "カテゴリーの取得に失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	category := resolved.Category
	if slug != category.Slug {
		httpredirect.ToCanonical(w, r, templates.CategoryPath(category.Slug))
		return
	}

	nav, err := h.getCommunityNavigationUC.Execute(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "コミュニティのナビゲーションの取得に失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	listing, err := h.getCategoryBoardsUC.Execute(ctx, usecase.GetCategoryBoardsInput{CategoryID: category.ID})
	if err != nil {
		slog.ErrorContext(ctx, "カテゴリーの掲示板一覧の取得に失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	meta := viewmodel.DefaultPageMeta(ctx, h.cfg)
	meta.Title = i18n.T(ctx, "category_show_title", map[string]any{"Name": category.Name})
	meta.Description = i18n.T(ctx, "category_show_description", map[string]any{"Name": category.Name})
	// A category is the list of its boards, so one that lists none has nothing to
	// offer a searcher and asks to be left out of the index rather than declaring
	// an address to be known by. It is also unreachable from anywhere in the
	// community while it is in that state: the sidebar is flat (ADR 0011) and the
	// breadcrumbs that link a category are rendered by the pages of the boards it
	// holds. Once a board is placed the page lists something, is linked from that
	// board's trail, and becomes indexable under its own address.
	//
	// [Ja] カテゴリーとはその掲示板の一覧であるため、1 つも並べないカテゴリーは検索する
	// 人に差し出すものを持たず、自身を知られるべきアドレスを宣言する代わりに、インデックス
	// から外すよう求めます。その状態のカテゴリーは、コミュニティのどこからも辿り着けても
	// いません。サイドバーはフラットであり (ADR 0011)、カテゴリーへリンクするパンくずは、
	// それが持つ掲示板のページが描画するものだからです。掲示板が 1 つ置かれれば、この
	// ページは何かを並べ、その掲示板の経路からリンクされ、自身のアドレスでインデックス
	// されうるようになります。
	if len(listing.Boards) == 0 {
		meta.NoIndex = true
	} else {
		meta.CanonicalURL = templates.CategoryPath(category.Slug).AbsoluteURL(h.cfg.AppURL)
	}

	returnTo := middleware.SanitizeReturnTo(r.URL.RequestURI())
	sidebar := viewmodel.NewSidebar(nav, middleware.UserFromContext(ctx), middleware.CSRFTokenFromContext(ctx), returnTo)

	boards := make([]categorypage.ShowBoard, len(listing.Boards))
	for i, board := range listing.Boards {
		boards[i] = categorypage.ShowBoard{Slug: board.Slug, Name: board.Name, Description: board.Description}
	}

	pageData := categorypage.ShowPageData{Name: category.Name, Boards: boards}
	columns := layouts.CommunityColumns{
		Center:             categorypage.ShowCenter(pageData),
		Right:              categorypage.ShowRight(pageData),
		MainLabelledBy:     categorypage.ShowHeadingID,
		ComplementaryLabel: i18n.T(ctx, "category_show_board_prompt_region_label"),
		Main:               layouts.CommunityCenterColumn,
	}
	layoutData := layouts.CommunityLayoutData{Meta: meta, Sidebar: sidebar, Columns: columns}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := layouts.Community(layoutData).Render(ctx, w); err != nil {
		slog.ErrorContext(ctx, "カテゴリーページのレンダリングに失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
