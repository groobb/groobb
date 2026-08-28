package home

import (
	"log/slog"
	"net/http"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/templates/layouts"
	homepage "github.com/groobb/groobb/go/internal/templates/pages/home"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// Show GET /home - renders the community's top page for a signed-in visitor: the
// sidebar listing this community's boards, and beside it every board with the
// few threads that moved in it most recently. It is registered behind
// RequireAuth, which guarantees a signed-in user, so the user from the context
// is non-nil and the handler does not nil-check it. The page is per-user and
// behind authentication, so it is marked noindex to keep it out of search
// indexes; the community's public pages carry no such mark. The sidebar's
// sign-out form takes the CSRF token from the context for the double-submit-
// cookie check.
//
// The page hands the layout one content column. Home is about the boards
// themselves rather than about anything opened inside one, so there is nothing
// for a second column to hold.
//
// [Ja] Show GET /home - サインイン済みの訪問者にコミュニティのトップページを描画します。
// このコミュニティの掲示板を並べるサイドバーと、その隣にすべての掲示板を、それぞれ直近で
// 動いた数件のスレッドとともに描きます。RequireAuth の背後に登録され、サインイン済み
// ユーザーが保証されるため、context のユーザーは非 nil であり、ハンドラーは nil チェックを
// 持ちません。このページはユーザー固有かつ認証の背後にあるため、検索インデックスから
// 除外するよう noindex を付けます。コミュニティの公開ページはこの印を持ちません。
// サイドバーのサインアウトフォームは double-submit cookie 検証のため context の CSRF
// トークンを載せます。
//
// このページはレイアウトへコンテンツカラムを 1 つだけ渡します。ホームが扱っているのは
// 掲示板そのものであって、その中で開いた何かではないため、2 つ目のカラムが持つものが
// ありません。
func (h *Handler) Show(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)

	nav, err := h.getCommunityNavigationUC.Execute(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "コミュニティのナビゲーションの取得に失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	home, err := h.getCommunityHomeUC.Execute(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "コミュニティのホームの取得に失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	meta := viewmodel.DefaultPageMeta(ctx, h.cfg)
	meta.Title = i18n.T(ctx, "home_show_title")
	meta.NoIndex = true

	returnTo := middleware.SanitizeReturnTo(r.URL.RequestURI())
	sidebar := viewmodel.NewSidebar(nav, user, middleware.CSRFTokenFromContext(ctx), returnTo)

	boards := make([]homepage.ShowBoard, len(home.Boards))
	for i, homeBoard := range home.Boards {
		threads := make([]homepage.ShowThread, len(homeBoard.Threads))
		for j, thread := range homeBoard.Threads {
			threads[j] = homepage.ShowThread{
				ID:           viewmodel.ThreadID(thread.ID),
				Title:        thread.Title,
				PostsCount:   thread.PostsCount,
				LastPostedAt: thread.LastPostedAt,
			}
		}
		boards[i] = homepage.ShowBoard{
			Slug:    homeBoard.Board.Slug,
			Name:    homeBoard.Board.Name,
			Threads: threads,
		}
	}

	columns := layouts.CommunityColumns{
		Center:         homepage.ShowCenter(homepage.ShowPageData{Boards: boards}),
		MainLabelledBy: homepage.ShowHeadingID,
		Main:           layouts.CommunityCenterColumn,
	}
	layoutData := layouts.CommunityLayoutData{Meta: meta, Sidebar: sidebar, Columns: columns}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := layouts.Community(layoutData).Render(ctx, w); err != nil {
		slog.ErrorContext(ctx, "ホームページのレンダリングに失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
