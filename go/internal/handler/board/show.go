package board

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/groobb/groobb/go/internal/httpredirect"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/templates"
	"github.com/groobb/groobb/go/internal/templates/components"
	"github.com/groobb/groobb/go/internal/templates/layouts"
	boardpage "github.com/groobb/groobb/go/internal/templates/pages/board"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// Show GET /b/{slug} - 掲示板を描画します。一覧カラムにそこへ立っているスレッドを、
// そしてコミュニティのシェルがどこでも運ぶサイドバーを描きます。このページはサインアウト
// 状態でも読めるため、RequireAuthではなくSetUserの背後に登録され、contextのユーザーは
// nilでありえます。その場合サイドバーはアカウント操作の代わりにサインインと新規登録を
// 差し出します。noindexは付けません。コミュニティの掲示板は、その会話へ辿り着く手立て
// だからです。
//
// はじめに件数の決まった掲示板解決UseCaseを走らせ、掲示板と、それが存在するときは
// パンくずに必要なカテゴリーを読みます。その結果で、そもそもページを描画するかどうかが
// 決まります。どの掲示板も指さないslugには404ページで応答し、DBのNOCASE照合で
// 解決できる大文字小文字違いのslugは保存済みの小文字slugへリダイレクトして、掲示板の
// 正規URLを1つに保ちます。どちらの応答も、読んで捨てることになるサイドバーの
// クエリと、件数に上限の無いスレッドの一覧を走らせません。
func (h *Handler) Show(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	slug := chi.URLParam(r, "slug")

	resolved, err := h.getBoardUC.Execute(ctx, usecase.GetBoardInput{Slug: slug})
	if err != nil {
		var ae *model.AppError
		if errors.As(err, &ae) && ae.Code == model.AppErrCodeResourceNotFound {
			h.errorRenderer.NotFound(w, r)
			return
		}
		slog.ErrorContext(ctx, "掲示板の取得に失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	board := resolved.Board
	if slug != board.Slug {
		httpredirect.ToCanonical(w, r, templates.BoardPath(board.Slug))
		return
	}

	nav, err := h.getCommunityNavigationUC.Execute(ctx, usecase.GetCommunityNavigationInput{
		UserID: middleware.UserIDFromContext(ctx),
	})
	if err != nil {
		slog.ErrorContext(ctx, "コミュニティのナビゲーションの取得に失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	listing, err := h.getBoardThreadsUC.Execute(ctx, usecase.GetBoardThreadsInput{BoardID: board.ID})
	if err != nil {
		slog.ErrorContext(ctx, "掲示板のスレッド一覧の取得に失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	meta := viewmodel.DefaultPageMeta(ctx, h.cfg)
	meta.Title = i18n.T(ctx, "board_show_title", map[string]any{"Name": board.Name})
	meta.Description = metaDescription(ctx, board)
	meta.CanonicalURL = templates.BoardPath(board.Slug).AbsoluteURL(h.cfg.AppURL)

	returnTo := middleware.SanitizeReturnTo(r.URL.RequestURI())
	sidebar := viewmodel.NewSidebar(nav, middleware.UserFromContext(ctx), middleware.CSRFTokenFromContext(ctx), returnTo)

	threads := make([]boardpage.ShowThread, len(listing.Threads))
	for i, thread := range listing.Threads {
		threads[i] = boardpage.ShowThread{
			ID:           viewmodel.ThreadID(thread.ID),
			Title:        thread.Title,
			Language:     viewmodel.NewThreadLanguage(thread.Language),
			PostsCount:   thread.PostsCount,
			LastPostedAt: thread.LastPostedAt,
		}
	}

	pageData := boardpage.ShowPageData{
		Slug:        board.Slug,
		Name:        board.Name,
		Description: board.Description,
		Breadcrumb:  breadcrumb(resolved.Category, board, h.cfg.AppURL),
		Threads:     threads,
	}
	columns := layouts.CommunityColumns{
		Center:             boardpage.ShowCenter(pageData),
		Right:              boardpage.ShowRight(pageData),
		MainLabelledBy:     boardpage.ShowHeadingID,
		ComplementaryLabel: i18n.T(ctx, "board_show_thread_prompt_region_label"),
		Main:               layouts.CommunityCenterColumn,
	}
	layoutData := layouts.CommunityLayoutData{Meta: meta, Sidebar: sidebar, Columns: columns}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := layouts.Community(layoutData).Render(ctx, w); err != nil {
		slog.ErrorContext(ctx, "掲示板ページのレンダリングに失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// breadcrumbは掲示板の在り処を示す経路を組み立てます。それを並べるカテゴリー、
// 続いて訪問者が今いる段としての掲示板自身です。/b/{slug} はカテゴリーについて何も
// 述べないため、コミュニティのどの部分にいるのかをこのページが訪問者に伝える場所は
// ここだけです。
//
// 経路をコミュニティのトップページではなくカテゴリーから始めるのは、/homeが認証の
// 背後にある一方このページはそうではないためです。サインアウト状態の訪問者は、辿ると
// サインインフォームへ追い返される最初の段を渡されることになります。
//
// どのカテゴリーにも属さない掲示板には上位として名指す場所がないため (ADR 0011)、経路は
// 掲示板1段として描画せず空のままにします。今描画しているページを表す1段だけの経路は、
// 訪問者がその見出しから既に読み取れること以上を何も述べないためです。
func breadcrumb(category *model.Category, board *model.Board, baseURL string) components.BreadcrumbData {
	if category == nil {
		return components.BreadcrumbData{}
	}

	return components.BreadcrumbData{
		Items: []components.BreadcrumbItem{
			{Name: category.Name, Path: templates.CategoryPath(category.Slug)},
			{Name: board.Name},
		},
		BaseURL: baseURL,
	}
}

// metaDescriptionは、このページが検索結果に対してこの掲示板について述べることを
// 返します。コミュニティが説明を書いていればその説明を、書いていなければ掲示板を名指す
// 一文です。
//
// コミュニティの言葉を優先するのは、それがその掲示板が何のためにあるかを述べるのに対し、
// 代替の一文はそのページが何であるかしか述べられないためです。代替を用意するのは、掲示板
// の説明が任意であり、それを持たないページはサイト全体の既定値に落ちて、どの掲示板でも
// 同じ一文になってしまうためです。
func metaDescription(ctx context.Context, board *model.Board) string {
	if board.Description != "" {
		return board.Description
	}

	return i18n.T(ctx, "board_show_description", map[string]any{"Name": board.Name})
}
