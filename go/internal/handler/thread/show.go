package thread

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
	threadpage "github.com/groobb/groobb/go/internal/templates/pages/thread"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// Show GET /t/{id} - スレッドを描画します。読むためのカラムにその投稿を、その傍らに
// それが立った掲示板の一覧を、そしてコミュニティのシェルがどこでも運ぶサイドバーを
// 描きます。このページはサインアウト状態でも読めるため、RequireAuthではなくSetUserの
// 背後に登録され、contextのユーザーはnilでありえます。その場合サイドバーはアカウント
// 操作の代わりにサインインと新規登録を差し出します。noindexは付けません。コミュニティの
// スレッドはその会話だからです。
//
// idは何かを読む前に検査します。スレッドのidの10進表記でないパスはどのスレッドも
// 名指しません。同じidを別の綴りで表すもの (strconvが受け付ける先頭のゼロやプラス記号)
// は正規の形へリダイレクトし、スレッドが1つのURLで応答するようにします。どちらの応答も
// クエリを要しません。idだけで両方が決まるためです。
//
// 管理者が非公開にしたスレッドには、どのスレッドも指さないidが受け取る404ではなく、取り下げ
// られたスレッドのページで応答します。ステータスはどちらも同じで、違うのは訪問者に伝える
// ことです。このアドレスはスレッドを持っていたため、もう開かないリンクを、打ち間違いのように
// 見せるのではなく説明します。
func (h *Handler) Show(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	raw := chi.URLParam(r, "id")

	id, ok := model.ParseThreadID(raw)
	if !ok {
		h.errorRenderer.NotFound(w, r)
		return
	}
	if raw != id.String() {
		httpredirect.ToCanonical(w, r, templates.ThreadPath(viewmodel.ThreadID(id)))
		return
	}

	resolved, err := h.getThreadUC.Execute(ctx, usecase.GetThreadInput{
		ID:     id,
		UserID: middleware.UserIDFromContext(ctx),
	})
	if err != nil {
		var ae *model.AppError
		if errors.As(err, &ae) {
			switch ae.Code {
			case model.AppErrCodeResourceNotFound:
				h.errorRenderer.NotFound(w, r)
				return
			case model.AppErrCodeResourceUnpublished:
				h.errorRenderer.Unpublished(w, r)
				return
			}
		}
		slog.ErrorContext(ctx, "スレッドの取得に失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
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
		ThreadID:   viewmodel.ThreadID(id),
		Title:      resolved.Thread.Title,
		Language:   language,
		Breadcrumb: breadcrumb(resolved, language, h.cfg.AppURL),
		PostsCount: resolved.Thread.PostsCount,
		Posts:      showPosts(resolved.Posts),
		Board:      showBoard(resolved.Board, listing.Threads),
		Lock:       viewmodel.NewThreadLock(resolved.Thread.LockReasons()),
		PostLimit:  model.ThreadPostLimit,
		Reply:      showReply(ctx, viewmodel.ThreadID(id), returnTo),
		Moderation: showModeration(ctx, resolved),
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

// breadcrumbはスレッドの在り処を示す経路を組み立てます。掲示板を並べるカテゴリー、
// スレッドが立った掲示板、続いて訪問者が今いる段としてのスレッド自身です。/t/{id} は
// そのどちらについても何も述べないため、コミュニティのどの部分にいるのかをこのページが
// 訪問者に伝える場所はここだけです。
//
// 経路をコミュニティのトップページではなくカテゴリーから始めるのは、掲示板のページが
// そうしているのと同じ理由です。/homeは認証の背後にある一方このページはそうではなく、
// サインアウト状態の訪問者は、辿るとサインインフォームへ追い返される最初の段を渡される
// ことになります。
//
// 現在地の段にはスレッドの言語を持たせます。他では言語を宣言する見出しの外側で、タイトルが
// ここにも繰り返されるためです。
//
// 掲示板がどのカテゴリーにも属さないときは掲示板から始めます (ADR 0011)。そのときも経路は
// 訪問者が今いる場所とその上位の2段を持ち、掲示板のページのように空にはなりません。
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

// showPostsはスレッドの投稿を、ページが描画する形へ変換し、各本文をテンプレートが
// 描く断片へ分解します。
//
// スレッドが持つレス番号の集合は1度だけ組み立て、すべての本文へ渡します。>>Nがリンクに
// なるかどうかは、それが書かれた本文ではなくスレッドによって決まるためです。このスレッドが
// 持たない番号はどこへも繋がらず、テキストのまま残ります。
//
// 作者はatnameとして渡し、名指すアカウントが無くなったときは "" とします。そうなる2つの
// 経路 — アカウントが退会した場合と、その行が既にパージされた場合 — は訪問者には同じものと
// して読めるため、ページはそれらを区別しません。
func showPosts(posts []usecase.ThreadPost) []threadpage.ShowPost {
	numbers := make(map[int]bool, len(posts))
	for _, post := range posts {
		numbers[post.Post.Number] = true
	}

	converted := make([]threadpage.ShowPost, len(posts))
	for i, post := range posts {
		// 非公開の投稿はその番号だけを運ぶ。ページはそこに占位を描き、印が見えない
		// 場所へ移したものはそこへ渡らない。渡したうえで描かない本文は、テンプレートの
		// 変更1つでまた表示されうるものになる。
		if post.Post.UnpublishedAt != nil {
			converted[i] = threadpage.ShowPost{Number: post.Post.Number, Unpublished: true}
			continue
		}

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

// showReplyは、この訪問者にとってスレッドの末尾に来るものを組み立てます。サイン
// インしていれば返信フォームを、していなければサインイン後に戻す先を持ちます。
//
// ここではロックを参照しません。スレッドが投稿を受け付けない理由は全員に対して成立する
// ため、案内とこれのどちらを出すかはページが同じデータから決めます。2箇所で別々に決めた
// ものが一致するのを当てにはしません。
//
// フォームの送信先はリクエストではなくスレッドから導くため、訪問者が同じidの別の綴りで
// ページへ辿り着いた場合でも、正規の /t/{id} を名指します。
func showReply(ctx context.Context, id viewmodel.ThreadID, returnTo string) threadpage.ShowReply {
	if middleware.UserFromContext(ctx) == nil {
		return threadpage.ShowReply{ReturnTo: returnTo}
	}

	return threadpage.ShowReply{
		SignedIn: true,
		Form: components.PostFormData{
			CSRFToken:           middleware.CSRFTokenFromContext(ctx),
			Action:              templates.ThreadPostsPath(id),
			PostIntervalSeconds: int(model.PostInterval.Seconds()),
		},
	}
}

// showModerationは、このスレッドに対して働きかけてよい訪問者にページが差し出すものを
// 組み立てます。それ以外の人には何も差し出しません。匿名の訪問者も、ロールを1つも持たない
// サインイン済みの訪問者も、どの権限も偽のままここへ至ります。
//
// 答えをcontextではなく読み取りから得るのは、ページが描くものと、その先の操作が許すものとを、
// 同じスコープから決めるためです。トークンはリクエストから取ります。ここに描かれる操作のうち、
// 手前に確認ページを持たないものは、このページから送信するためです。
func showModeration(ctx context.Context, resolved *usecase.GetThreadOutput) threadpage.ShowModeration {
	return threadpage.ShowModeration{
		CanLockThread:      resolved.CanLockThread,
		CanUnpublishThread: resolved.CanUnpublishThread,
		CanUnpublishPost:   resolved.CanUnpublishPost,
		CSRFToken:          middleware.CSRFTokenFromContext(ctx),
	}
}

// showBoardは掲示板とそのスレッドを、ページの一覧カラムが描画する一覧へ変換します。
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
