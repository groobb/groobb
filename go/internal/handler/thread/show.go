package thread

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/groobb/groobb/go/internal/httpredirect"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/templates"
	"github.com/groobb/groobb/go/internal/templates/components"
	"github.com/groobb/groobb/go/internal/templates/layouts"
	threadpage "github.com/groobb/groobb/go/internal/templates/pages/thread"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// Show GET /t/{id} - renders a thread: its posts in the reading column, the
// listing of the board it was posted in beside them, and the sidebar the whole
// community shell carries. The page is readable while signed out, so it is
// registered behind SetUser rather than RequireAuth and the user from the
// context may be nil; the sidebar then offers sign-in and sign-up in place of
// the account controls. It carries no noindex, since a community's threads are
// its conversations.
//
// The id is checked before anything is read. A path that is not the decimal
// spelling of a thread's id names no thread, and one that spells the same id
// differently — a leading zero or a plus sign, which strconv accepts — is
// redirected to the canonical form, so the thread answers under one URL. Neither
// answer costs a query: the id alone settles both.
//
// [Ja] Show GET /t/{id} - スレッドを描画します。読むためのカラムにその投稿を、その傍らに
// それが立った掲示板の一覧を、そしてコミュニティのシェルがどこでも運ぶサイドバーを
// 描きます。このページはサインアウト状態でも読めるため、RequireAuth ではなく SetUser の
// 背後に登録され、context のユーザーは nil でありえます。その場合サイドバーはアカウント
// 操作の代わりにサインインと新規登録を差し出します。noindex は付けません。コミュニティの
// スレッドはその会話だからです。
//
// id は何かを読む前に検査します。スレッドの id の 10 進表記でないパスはどのスレッドも
// 名指しません。同じ id を別の綴りで表すもの (strconv が受け付ける先頭のゼロやプラス記号)
// は正規の形へリダイレクトし、スレッドが 1 つの URL で応答するようにします。どちらの応答も
// クエリを要しません。id だけで両方が決まるためです。
func (h *Handler) Show(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	raw := chi.URLParam(r, "id")

	id, ok := parseThreadID(raw)
	if !ok {
		h.errorRenderer.NotFound(w, r)
		return
	}
	if raw != id.String() {
		httpredirect.ToCanonical(w, r, templates.ThreadPath(viewmodel.ThreadID(id)))
		return
	}

	resolved, err := h.getThreadUC.Execute(ctx, usecase.GetThreadInput{ID: id})
	if err != nil {
		var ae *model.AppError
		if errors.As(err, &ae) && ae.Code == model.AppErrCodeResourceNotFound {
			h.errorRenderer.NotFound(w, r)
			return
		}
		slog.ErrorContext(ctx, "スレッドの取得に失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	nav, err := h.getCommunityNavigationUC.Execute(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "コミュニティのナビゲーションの取得に失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	listing, err := h.getBoardThreadsUC.Execute(ctx, usecase.GetBoardThreadsInput{BoardID: resolved.Board.ID})
	if err != nil {
		slog.ErrorContext(ctx, "掲示板のスレッド一覧の取得に失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	meta := viewmodel.DefaultPageMeta(ctx, h.cfg)
	meta.Title = i18n.T(ctx, "thread_show_title", map[string]any{"Title": resolved.Thread.Title})
	meta.Description = i18n.T(ctx, "thread_show_description", map[string]any{
		"Title": resolved.Thread.Title,
		"Board": resolved.Board.Name,
	})
	meta.CanonicalURL = templates.ThreadPath(viewmodel.ThreadID(id)).AbsoluteURL(h.cfg.AppURL)

	returnTo := middleware.SanitizeReturnTo(r.URL.RequestURI())
	sidebar := viewmodel.NewSidebar(nav, middleware.UserFromContext(ctx), middleware.CSRFTokenFromContext(ctx), returnTo)
	language := viewmodel.NewThreadLanguage(resolved.Thread.Language)

	pageData := threadpage.ShowPageData{
		Title:      resolved.Thread.Title,
		Language:   language,
		Breadcrumb: breadcrumb(resolved, language, h.cfg.AppURL),
		PostsCount: resolved.Thread.PostsCount,
		Posts:      showPosts(resolved.Posts),
		Board:      showBoard(resolved.Board, listing.Threads),
		Full:       resolved.Thread.PostsCount >= model.ThreadPostLimit,
		PostLimit:  model.ThreadPostLimit,
	}
	columns := layouts.CommunityColumns{
		Center:             threadpage.ShowCenter(pageData),
		Right:              threadpage.ShowRight(pageData),
		MainLabelledBy:     threadpage.ShowHeadingID,
		ComplementaryLabel: i18n.T(ctx, "thread_show_board_threads_region_label"),
		Main:               layouts.CommunityRightColumn,
	}
	layoutData := layouts.CommunityLayoutData{Meta: meta, Sidebar: sidebar, Columns: columns}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := layouts.Community(layoutData).Render(ctx, w); err != nil {
		slog.ErrorContext(ctx, "スレッドページのレンダリングに失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// parseThreadID reads the id out of the path, reporting whether the path can
// name a thread at all. Anything that is not a positive whole number is rejected
// here rather than looked up, since no thread carries such an id and the lookup
// would answer 404 after a query.
//
// [Ja] parseThreadID はパスから id を読み取り、そのパスがそもそもスレッドを名指しうるか
// どうかを併せて返します。正の整数でないものはルックアップせずここで弾きます。そのような
// id を持つスレッドは無く、ルックアップしてもクエリを 1 回発行した末に 404 になるだけ
// だからです。
func parseThreadID(raw string) (model.ThreadID, bool) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return model.ThreadID(id), true
}

// breadcrumb builds the trail naming where the thread sits: the category that
// lists the board, the board the thread was posted in, then the thread itself as
// the step the visitor is on. /t/{id} says nothing about either, so this is the
// only place the page tells a visitor which part of the community they are in.
//
// A board sitting in no category (ADR 0011) drops the first step, and the trail
// starts at the board. It still names where the visitor is and the place above
// it, so unlike a board's own page it is not left empty.
//
// The trail starts at the category rather than at the community's top page, for
// the reason a board's page starts there: /home is behind authentication and
// this page is not, so a signed-out visitor would be handed a first step that
// turns them away to the sign-in form.
//
// The current step carries the thread's language because its title repeats
// there outside the heading that otherwise declares it.
//
// [Ja] breadcrumb はスレッドの在り処を示す経路を組み立てます。掲示板を並べるカテゴリー、
// スレッドが立った掲示板、続いて訪問者が今いる段としてのスレッド自身です。/t/{id} は
// そのどちらについても何も述べないため、コミュニティのどの部分にいるのかをこのページが
// 訪問者に伝える場所はここだけです。
//
// 経路をコミュニティのトップページではなくカテゴリーから始めるのは、掲示板のページが
// そうしているのと同じ理由です。/home は認証の背後にある一方このページはそうではなく、
// サインアウト状態の訪問者は、辿るとサインインフォームへ追い返される最初の段を渡される
// ことになります。
//
// 現在地の段にはスレッドの言語を持たせます。他では言語を宣言する見出しの外側で、タイトルが
// ここにも繰り返されるためです。
//
// 掲示板がどのカテゴリーにも属さないときは掲示板から始めます (ADR 0011)。そのときも経路は
// 訪問者が今いる場所とその上位の 2 段を持ち、掲示板のページのように空にはなりません。
func breadcrumb(resolved *usecase.GetThreadOutput, language viewmodel.ThreadLanguage, baseURL string) components.BreadcrumbData {
	items := make([]components.BreadcrumbItem, 0, 3)
	if resolved.Category != nil {
		items = append(items, components.BreadcrumbItem{
			Name: resolved.Category.Name,
			Path: templates.CategoryPath(resolved.Category.Slug),
		})
	}
	items = append(items,
		components.BreadcrumbItem{Name: resolved.Board.Name, Path: templates.BoardPath(resolved.Board.Slug)},
		components.BreadcrumbItem{Name: resolved.Thread.Title, Lang: language.Tag},
	)

	return components.BreadcrumbData{Items: items, BaseURL: baseURL}
}

// showPosts converts the thread's posts into what the page renders, splitting
// each body into the pieces the template draws.
//
// The set of reply numbers the thread carries is built once and handed to every
// body, because whether a >>N is a link depends on the thread rather than on the
// body it is written in: a number this thread does not hold leads nowhere and
// stays text.
//
// An author is passed as an atname, and as "" once there is no account to name.
// The two ways that happens — the account has withdrawn, or its row has since
// been purged — read the same to a visitor, so the page does not tell them
// apart.
//
// [Ja] showPosts はスレッドの投稿を、ページが描画する形へ変換し、各本文をテンプレートが
// 描く断片へ分解します。
//
// スレッドが持つレス番号の集合は 1 度だけ組み立て、すべての本文へ渡します。>>N がリンクに
// なるかどうかは、それが書かれた本文ではなくスレッドによって決まるためです。このスレッドが
// 持たない番号はどこへも繋がらず、テキストのまま残ります。
//
// 作者は atname として渡し、名指すアカウントが無くなったときは "" とします。そうなる 2 つの
// 経路 — アカウントが退会した場合と、その行が既にパージされた場合 — は訪問者には同じものと
// して読めるため、ページはそれらを区別しません。
func showPosts(posts []usecase.ThreadPost) []threadpage.ShowPost {
	numbers := make(map[int]bool, len(posts))
	for _, post := range posts {
		numbers[post.Post.Number] = true
	}

	converted := make([]threadpage.ShowPost, len(posts))
	for i, post := range posts {
		author := ""
		if post.Author != nil {
			author = post.Author.Atname
		}
		converted[i] = threadpage.ShowPost{
			Number:       post.Post.Number,
			Author:       author,
			PostedAt:     post.Post.CreatedAt,
			Body:         viewmodel.NewPostBody(post.Post.Body, numbers),
			ReplyNumbers: post.ReplyNumbers,
		}
	}
	return converted
}

// showBoard converts the board and its threads into the listing the page's list
// column renders.
//
// [Ja] showBoard は掲示板とそのスレッドを、ページの一覧カラムが描画する一覧へ変換します。
func showBoard(board *model.Board, threads []*model.Thread) threadpage.ShowBoard {
	showThreads := make([]threadpage.ShowBoardThread, len(threads))
	for i, thread := range threads {
		showThreads[i] = threadpage.ShowBoardThread{
			ID:           viewmodel.ThreadID(thread.ID),
			Title:        thread.Title,
			Language:     viewmodel.NewThreadLanguage(thread.Language),
			PostsCount:   thread.PostsCount,
			LastPostedAt: thread.LastPostedAt,
		}
	}
	return threadpage.ShowBoard{Slug: board.Slug, Name: board.Name, Threads: showThreads}
}
