package home_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/handler/home"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/templates"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
)

// newHandler builds the home Handler over a database holding one community with
// two boards: one that has been posted in and one that has not, which is the
// smallest arrangement in which the page renders both a section listing threads
// and a section saying a board has none. The posted-in board is given a category
// even though neither the sidebar nor this page draws one, so that the listing is
// exercised on a board that has one.
//
// That board's threads are written in three languages, because a board is not
// divided by language: a section holding only threads in the page's own language
// would not show whether a row says which language it is in. The one written in
// a language the application has no locale for is what the row that declares no
// language at all is checked on.
//
// [Ja] newHandler は、1 つのコミュニティと 2 つの掲示板 — 書き込まれたものと、まだ
// 書き込まれていないもの — を持つデータベース上に home Handler を構築します。スレッドを
// 並べる区画と、スレッドが無いことを伝える区画の両方をページが描画する最小の構成です。
// サイドバーもこのページもカテゴリーを描かないにもかかわらず、書き込まれた掲示板に
// カテゴリーを与えているのは、一覧をカテゴリーを持つ掲示板で動かすためです。
//
// その掲示板のスレッドは 3 つの言語で書かれています。掲示板は言語で分けないためで、
// ページ自身の言語のスレッドしか持たない区画では、行が自身の言語を述べているかどうかを
// 確かめられません。アプリがロケールを持たない言語で書かれた 1 本は、どの言語も宣言
// しない行を確かめる先です。
func newHandler(t *testing.T) *home.Handler {
	t.Helper()

	ctx := context.Background()
	db := testutil.SetupDB(t)

	if _, err := db.Writer.ExecContext(ctx, "INSERT INTO communities (id, name) VALUES (1, ?)", "ジャズ喫茶"); err != nil {
		t.Fatalf("communities への INSERT に失敗: %v", err)
	}

	categoryRepo := repository.NewCategoryRepository(db)
	boardRepo := repository.NewBoardRepository(db)

	category, err := categoryRepo.Create(ctx, repository.CreateCategoryInput{Slug: "music", Name: "音楽", Position: 1})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	board, err := boardRepo.Create(ctx, repository.CreateBoardInput{
		CategoryID: &category.ID,
		Slug:       "jazz",
		Name:       "ジャズ・ファンク",
		Position:   1,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := boardRepo.Create(ctx, repository.CreateBoardInput{
		Slug:     "quiet",
		Name:     "静かな板",
		Position: 2,
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// The last-post times are set relative to now, since the page states them as
	// the distance from now.
	//
	// [Ja] 最終投稿の時刻は現在時刻からの相対で与えます。ページがそれを今からの隔たり
	// として述べるためです。
	now := time.Now()
	createThread(t, ctx, db, board.ID, "モードジャズの話", model.LocaleJa.ThreadLanguage(), 7, now.Add(-5*time.Hour))
	createThread(t, ctx, db, board.ID, "Mes derniers disques", model.ThreadLanguageOther, 2, now.Add(-4*time.Hour))
	createThread(t, ctx, db, board.ID, "Records I picked up", model.LocaleEn.ThreadLanguage(), 5, now.Add(-3*time.Hour))
	createThread(t, ctx, db, board.ID, "最近買ったレコード", model.LocaleJa.ThreadLanguage(), 3, now.Add(-2*time.Hour))

	return newHandlerForDB(db)
}

// createThread inserts a thread the way one exists in practice: with a first
// post, and with the denormalized columns describing the thread's posts. The
// listing reads its order and both of the facts it shows from those columns.
//
// [Ja] createThread は、実際にスレッドが存在する形 — 最初の投稿を伴い、非正規化列が
// スレッドの投稿を表している状態 — でスレッドを挿入します。一覧はその並び順も、示す
// 2 つの事実も、この列から読みます。
func createThread(t *testing.T, ctx context.Context, db *database.DB, boardID model.BoardID, title string, language model.ThreadLanguage, postsCount int, lastPostedAt time.Time) {
	t.Helper()

	threadRepo := repository.NewThreadRepository(db)
	postRepo := repository.NewPostRepository(db)

	thread, err := threadRepo.Create(ctx, repository.CreateThreadInput{BoardID: boardID, Title: title, Language: language})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	post, err := postRepo.Create(ctx, repository.CreatePostInput{ThreadID: thread.ID, Number: 1, Body: title + "の 1 つ目の投稿"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := threadRepo.UpdateLastPost(ctx, thread.ID, repository.UpdateThreadLastPostInput{
		PostsCount:   postsCount,
		LastPostID:   post.ID,
		LastPostedAt: lastPostedAt,
	}); err != nil {
		t.Fatalf("UpdateLastPost() error = %v", err)
	}
}

// newHandlerForDB builds the home Handler over the supplied application
// database.
//
// [Ja] newHandlerForDB は、渡されたアプリケーションデータベース上に home Handler を
// 構築します。
func newHandlerForDB(db *database.DB) *home.Handler {
	return newHandlerForDatabases(db, db)
}

// newHandlerForDatabases builds the navigation and home UseCases over separate
// application databases, allowing a test to make the second read fail only
// after the first one has succeeded.
//
// [Ja] newHandlerForDatabases はナビゲーションとホームの UseCase を別々の
// アプリケーションデータベース上に構築し、最初の読み取りが成功した後に 2 番目の
// 読み取りだけを失敗させられるようにします。
func newHandlerForDatabases(navigationDB, homeDB *database.DB) *home.Handler {
	getCommunityNavigationUC := usecase.NewGetCommunityNavigationUsecase(
		repository.NewCommunityRepository(navigationDB),
		repository.NewBoardRepository(navigationDB),
	)
	getCommunityHomeUC := usecase.NewGetCommunityHomeUsecase(
		repository.NewBoardRepository(homeDB),
		repository.NewThreadRepository(homeDB),
	)

	return home.NewHandler(&config.Config{Env: "dev"}, getCommunityNavigationUC, getCommunityHomeUC)
}

// TestShow verifies that GET /home returns HTTP 200 with an HTML body that
// renders, for each supported locale, the community shell: the skip link and the
// <main> landmark it jumps to (named by the page heading it holds), the sidebar
// landmark carrying the community name and the board link, the account controls
// (the settings link and the sign-out form reaching DELETE /user_session through
// the _method override, with the CSRF hidden field and a confirmation prompt),
// and the noindex robots meta this behind-auth page carries.
//
// It also verifies the listing this page is: a section per board headed by the
// board's name as the link to it, holding that board's latest threads with the
// most recently posted-in first, each stating its post count and how long ago
// the last post arrived, and a board nobody has posted in saying so.
//
// The user and the current path are placed in the context directly (as
// RequireAuth and CurrentPathMiddleware would), so the handler runs without
// those middlewares.
//
// [Ja] TestShow は GET /home が HTTP 200 と、サポートする各ロケールについてコミュニティの
// シェルを描画した HTML ボディを返すことを検証します。スキップリンクとその飛び先の
// <main> ランドマーク (それが持つページ見出しで名付けられる)、コミュニティ名と掲示板の
// リンクを運ぶサイドバーのランドマーク、アカウント操作 (設定リンクと、_method オーバー
// ライドで DELETE /user_session に到達するサインアウトフォーム。CSRF hidden フィールドと
// 確認文言つき)、そして認証背後のこのページが持つ noindex の robots メタです。
//
// 併せて、このページ自身である一覧も検証します。掲示板ごとの区画が、その掲示板への
// リンクである掲示板名を見出しに持ち、その掲示板の最新スレッドを最後に投稿されたものから
// 順に並べ、各スレッドが投稿数と最終投稿からの隔たりを述べること、そして誰も書き込んで
// いない掲示板がその旨を伝えることです。
//
// ユーザーと現在のパスは (RequireAuth と CurrentPathMiddleware がするように) context に
// 直接載せ、これらのミドルウェアなしでハンドラーを走らせます。
func TestShow(t *testing.T) {
	t.Parallel()

	handler := newHandler(t)

	tests := []struct {
		name             string
		locale           model.Locale
		wantHeading      string
		wantSignOutBtn   string
		wantSettings     string
		wantSkipLink     string
		wantBoardsLabel  string
		wantSidebarLabel string
		wantPostsCount   string
		wantLastPosted   string
		wantRelativeTime string
		wantNoThreads    string
	}{
		{
			name:             "Japanese",
			locale:           model.LocaleJa,
			wantHeading:      "コミュニティのトップ",
			wantSignOutBtn:   "ログアウト",
			wantSettings:     "設定",
			wantSkipLink:     "本文へスキップ",
			wantBoardsLabel:  "このコミュニティの板",
			wantSidebarLabel: "コミュニティ",
			wantPostsCount:   "3 件の投稿",
			wantLastPosted:   "最終投稿",
			wantRelativeTime: "2 時間前",
			wantNoThreads:    "まだスレッドがありません。",
		},
		{
			name:             "English",
			locale:           model.LocaleEn,
			wantHeading:      "Community home",
			wantSignOutBtn:   "Sign out",
			wantSettings:     "Settings",
			wantSkipLink:     "Skip to main content",
			wantBoardsLabel:  "Boards in this community",
			wantSidebarLabel: "Community",
			wantPostsCount:   "3 posts",
			wantLastPosted:   "Last post",
			wantRelativeTime: "2 hours ago",
			wantNoThreads:    "No threads yet.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/home", nil)
			ctx := i18n.SetLocale(req.Context(), tt.locale)
			ctx = middleware.SetUserToContext(ctx, &model.User{Atname: "alice"})
			ctx = templates.SetCurrentPath(ctx, templates.HomePath().String())
			req = req.WithContext(ctx)
			rec := httptest.NewRecorder()

			handler.Show(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("status code = %d, want %d", rec.Code, http.StatusOK)
			}

			if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
				t.Errorf("Content-Type = %q, want prefix %q", got, "text/html")
			}

			body := rec.Body.String()
			wants := []string{
				tt.wantHeading,
				tt.wantSignOutBtn,
				tt.wantSettings,
				tt.wantSkipLink,
				tt.wantBoardsLabel,
				tt.wantPostsCount,
				tt.wantLastPosted,
				tt.wantRelativeTime,
				tt.wantNoThreads,
				`href="#main"`,
				`id="main"`,
				"ジャズ喫茶",
				`href="/b/jazz"`,
				"ジャズ・ファンク",
				`href="/b/quiet"`,
				"静かな板",
				"最近買ったレコード",
				"モードジャズの話",
				"<time datetime=",
				`href="/settings"`,
				"@alice",
				`action="/user_session"`,
				`method="POST"`,
				`name="_method" value="DELETE"`,
				`name="csrf_token"`,
				"data-confirm",
				`<meta name="robots" content="noindex"`,
				// The page's own language is asserted on the <html> element
				// itself. The rows below carry lang attributes of their own, so a
				// page-wide search for the tag is satisfied by a thread's title or
				// its badge whatever the document declares.
				//
				// [Ja] ページ自身の言語は <html> 要素そのもので検証する。以下の行が自身の
				// lang 属性を持つため、タグをページ全体から探す形では、文書が何を宣言して
				// いてもスレッドのタイトルかそのバッジで満たされてしまう。
				`<html lang="` + string(tt.locale) + `"`,
			}
			for _, want := range wants {
				if !strings.Contains(body, want) {
					t.Errorf("response body does not contain %q", want)
				}
			}

			main := body[strings.Index(body, `id="main"`):]
			if strings.Index(main, "最近買ったレコード") > strings.Index(main, "モードジャズの話") {
				t.Error("スレッドが最終投稿の新しい順に並んでいない")
			}

			// The listing's outline is h1 → h2 → h3: a board's name heads its
			// section and a thread's title heads a row inside it, so a visitor
			// moving by heading reaches a board and then its conversations. Each
			// heading is taken by the markup only its own level produces — a
			// heading opening straight into the link that is its text — because
			// the page's first h2 is not necessarily a board's: a flash renders an
			// h2 as well. Both links keep the minimum touch target the page's
			// other compact links use.
			//
			// [Ja] 一覧のアウトラインは h1 → h2 → h3 である。掲示板の名前がその区画を、
			// スレッドのタイトルがその中の 1 行を見出しとして束ねるため、見出しを辿る
			// 訪問者は掲示板へ、そしてその会話へ着く。各見出しは、そのレベルだけが生む
			// マークアップ (見出しが直ちに、その見出しのテキストであるリンクを開く形) で
			// 取り出す。ページの最初の h2 が掲示板のものであるとは限らないためで、
			// フラッシュも h2 を描画する。どちらのリンクも、ページ内の他の小さなリンクと
			// 共通の最小タッチ領域を保つ。
			boardHeading := testutil.Element(t, main, `<h2 class="text-base font-semibold"><a href="/b/jazz"`, "</h2>")
			if !strings.Contains(boardHeading, "ジャズ・ファンク") {
				t.Errorf("掲示板の見出し = %s, want the board name as a link to the board", boardHeading)
			}
			threadHeading := testutil.Element(t, main, `<h3 class="font-medium"><a href="/t/`, "</h3>")
			if !strings.Contains(threadHeading, "最近買ったレコード") {
				t.Errorf("スレッドの見出し = %s, want the title as a link to the thread", threadHeading)
			}
			for _, heading := range []string{boardHeading, threadHeading} {
				link := testutil.OpeningTag(t, heading, "href=")
				for _, want := range []string{"inline-flex", "min-h-6", "min-w-6"} {
					if !strings.Contains(link, want) {
						t.Errorf("見出しのリンクに %q が無い: %s", want, link)
					}
				}
			}

			nav := testutil.OpeningTag(t, body, `aria-labelledby="sidebar-boards-label"`)
			if !strings.HasPrefix(nav, "<nav ") {
				t.Errorf("掲示板一覧のラベルを持つ要素 = %s, want nav", nav)
			}
			if strings.Contains(body, `href="/c/music"`) {
				t.Error("サイドバーにカテゴリーへのリンクが含まれている (掲示板はフラットに並べる)")
			}
			sidebar := testutil.OpeningTag(t, body, `aria-label="`+tt.wantSidebarLabel+`"`)
			if !strings.HasPrefix(sidebar, "<aside ") {
				t.Errorf("サイドバーの要素 = %s, want aside", sidebar)
			}
			mainTag := testutil.OpeningTag(t, body, `id="main"`)
			if !strings.HasPrefix(mainTag, "<main ") || !strings.Contains(mainTag, `aria-labelledby="home-show-heading"`) {
				t.Errorf("main landmark = %s, want the page heading as its accessible name", mainTag)
			}
			heading := testutil.OpeningTag(t, body, `id="home-show-heading"`)
			if !strings.HasPrefix(heading, "<h1 ") {
				t.Errorf("main landmark を名付ける要素 = %s, want h1", heading)
			}
			if got := strings.Count(body, "<aside"); got != 1 {
				t.Errorf("aside の数 = %d, want %d (このページは補足のカラムを持たない)", got, 1)
			}
			if got := strings.Count(body, "<h1"); got != 1 {
				t.Errorf("h1 の数 = %d, want 1", got)
			}
			beforeMain := body[:strings.Index(body, `id="main"`)]
			if strings.Contains(beforeMain, "<h2") || strings.Contains(beforeMain, "<h3") {
				t.Error("サイドバーに本文の見出し階層へ入る h2 または h3 が含まれている")
			}
		})
	}
}

// TestShow_EmptyInstance verifies that an instance without community content
// still renders the board navigation shell without inventing a home link, and
// says both in the listing and in the sidebar that the community holds no board
// yet rather than rendering an empty page beside an empty list.
//
// The sidebar is asserted within its own navigation landmark, because the
// listing's own empty state says the same thing further down the document.
//
// [Ja] TestShow_EmptyInstance は、コミュニティの内容がないインスタンスでも板の
// ナビゲーション枠を描画し、存在しないホームリンクを作らないこと、そして空の一覧の傍らの
// 空のページではなく、一覧とサイドバーの双方がコミュニティのまだ掲示板を持たないことを
// 伝えることを検証します。
//
// サイドバーについてはそのナビゲーションランドマークの中で検証します。一覧自身の空状態が
// 文書のより後ろで同じことを述べているためです。
func TestShow_EmptyInstance(t *testing.T) {
	t.Parallel()

	handler := newHandlerForDB(testutil.SetupDB(t))
	req := httptest.NewRequest(http.MethodGet, "/home", nil)
	ctx := i18n.SetLocale(req.Context(), model.LocaleJa)
	ctx = middleware.SetUserToContext(ctx, &model.User{Atname: "alice"})
	ctx = templates.SetCurrentPath(ctx, templates.HomePath().String())
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.Show(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "<nav ") {
		t.Error("空のインスタンスのレスポンスに nav が含まれていない")
	}
	if strings.Contains(body, `href="/home"`) {
		t.Error("空のインスタンスのレスポンスにホームリンクが含まれている")
	}
	if !strings.Contains(body, "このコミュニティにはまだ掲示板がありません。") {
		t.Error("掲示板を 1 つも持たないコミュニティの空状態が表示されていない")
	}
	boards := testutil.Element(t, body, `id="sidebar-boards-label"`, "</nav>")
	if !strings.Contains(boards, "まだ掲示板がありません。") {
		t.Error("サイドバーの掲示板一覧に空状態が表示されていない")
	}
}

// TestShow_NavigationFailure verifies that failure to load community navigation
// is returned as an internal server error rather than a partial page.
//
// [Ja] TestShow_NavigationFailure は、コミュニティナビゲーションの取得失敗が部分的な
// ページではなく Internal Server Error として返ることを検証します。
func TestShow_NavigationFailure(t *testing.T) {
	t.Parallel()

	handler := newHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/home", nil)
	ctx := middleware.SetUserToContext(req.Context(), &model.User{Atname: "alice"})
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.Show(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(rec.Body.String(), "Internal Server Error") {
		t.Error("response body does not contain Internal Server Error")
	}
}

// TestShow_CommunityHomeFailure verifies that failure to load the community
// home after its navigation was loaded is returned as an internal server error
// rather than as a page containing only the sidebar.
//
// [Ja] TestShow_CommunityHomeFailure は、ナビゲーションの取得成功後にコミュニティの
// ホーム取得が失敗した場合、サイドバーだけを含むページではなく Internal Server Error が
// 返ることを検証します。
func TestShow_CommunityHomeFailure(t *testing.T) {
	t.Parallel()

	navigationDB := testutil.SetupDB(t)
	homeDB := testutil.SetupDB(t)
	if err := homeDB.Reader.Close(); err != nil {
		t.Fatalf("Reader の Close() error = %v", err)
	}

	handler := newHandlerForDatabases(navigationDB, homeDB)
	req := httptest.NewRequest(http.MethodGet, "/home", nil)
	ctx := middleware.SetUserToContext(req.Context(), &model.User{Atname: "alice"})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.Show(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(rec.Body.String(), "Internal Server Error") {
		t.Error("response body does not contain Internal Server Error")
	}
}

// TestShow_MarksOnlyTheCurrentPage verifies that the sidebar marks with
// aria-current only the link pointing at the page being rendered. On home that
// is the community's own name; the board links are not the current page and must
// go unmarked, which is what keeps the mark from being a decoration the sidebar
// applies wherever it appears.
//
// [Ja] TestShow_MarksOnlyTheCurrentPage は、サイドバーが aria-current を付けるのが
// 今描画しているページを指すリンクだけであることを検証します。ホームではそれが
// コミュニティ自身の名前であり、掲示板のリンクは現在のページではないため印は付きません。
// これにより、印がサイドバーの現れる場所すべてに付く飾りにならずに済みます。
func TestShow_MarksOnlyTheCurrentPage(t *testing.T) {
	t.Parallel()

	handler := newHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/home", nil)
	ctx := i18n.SetLocale(req.Context(), model.LocaleJa)
	ctx = middleware.SetUserToContext(ctx, &model.User{Atname: "alice"})
	ctx = templates.SetCurrentPath(ctx, templates.HomePath().String())
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.Show(rec, req)

	body := rec.Body.String()

	if got := strings.Count(body, `aria-current="page"`); got != 1 {
		t.Errorf("aria-current=\"page\" の数 = %d, want %d (ホームを指すリンクのみ)", got, 1)
	}
	if boardLink := testutil.OpeningTag(t, body, `href="/b/jazz"`); strings.Contains(boardLink, "aria-current") {
		t.Errorf("掲示板のリンクに aria-current が付いている: %s", boardLink)
	}
}

// TestShow_ThreadLanguage verifies that each row of a board's section says which
// language its thread is written in: a badge carrying the language's own name,
// and the title declared as that language. Home crosses every board, so it is
// where the languages of the whole community meet, and without this a visitor
// cannot tell which of the listed conversations they can read.
//
// The row for a thread whose language resolves to no display language is checked
// for the opposite: the badge falls back to the translated word, and the title
// declares nothing rather than an empty or invented tag.
//
// [Ja] TestShow_ThreadLanguage は、掲示板の区画の各行が自身のスレッドの言語を述べる
// ことを検証します。その言語自身の名前を載せたバッジと、その言語として宣言された
// タイトルです。ホームはすべての掲示板を横断するため、コミュニティ全体の言語が出会う
// 場所であり、これが無いと訪問者は並んだ会話のうちどれを読めるのかを見分けられません。
//
// どの表示言語にも解決しない言語のスレッドの行では、その逆を確かめます。バッジは訳語へ
// 退き、タイトルは空のタグやでっち上げたタグではなく、何も宣言しません。
func TestShow_ThreadLanguage(t *testing.T) {
	t.Parallel()

	handler := newHandler(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/home", nil)
	ctx := i18n.SetLocale(req.Context(), model.LocaleJa)
	ctx = middleware.SetUserToContext(ctx, &model.User{Atname: "alice"})
	ctx = templates.SetCurrentPath(ctx, templates.HomePath().String())
	handler.Show(rec, req.WithContext(ctx))

	body := rec.Body.String()

	// The title's own element carries the declaration, so the tag covers the
	// title and nothing else on the row.
	//
	// [Ja] 宣言はタイトル自身の要素が持つ。タグが覆うのはタイトルであって、行の他の
	// ものではない。
	link := testutil.OpeningTag(t, body, ">Records I picked up<")
	if !strings.Contains(link, `lang="en"`) {
		t.Errorf("英語のスレッドのタイトル = %s, want lang=\"en\"", link)
	}

	row := testutil.Element(t, body, ">Records I picked up<", "</p>")
	for _, want := range []string{`<span class="sr-only">主言語:</span>`, `<span lang="en">English</span>`} {
		if !strings.Contains(row, want) {
			t.Errorf("英語のスレッドの行 = %s, want %q", row, want)
		}
	}

	// Every row is badged, not only the ones in another language, so that a
	// missing badge never has to be read as "this one is in my language".
	//
	// [Ja] バッジが付くのは別の言語の行だけではなく、どの行にも付く。バッジが無いことを
	// 「これは自分の言語だ」と読む必要が生じないようにするため。
	if !strings.Contains(body, `<span lang="ja">日本語</span>`) {
		t.Error("日本語のスレッドにバッジが無い")
	}

	// The thread written in a language the application has no locale for is the
	// one row that declares none. There is no tag to declare, and an invented one
	// would have a screen reader pronounce the title by the rules of a language
	// it is not written in, so the badge carries the translated word instead.
	//
	// [Ja] アプリがロケールを持たない言語で書かれたスレッドは、どの言語も宣言しない
	// 唯一の行である。宣言するタグが無く、でっち上げたタグは、そのタイトルが書かれて
	// いない言語の規則でスクリーンリーダーに発音させることになるため、バッジは代わりに
	// 訳語を載せる。
	otherTitle := testutil.OpeningTag(t, body, ">Mes derniers disques<")
	if strings.Contains(otherTitle, "lang=") {
		t.Errorf("other のスレッドのタイトル = %s, want lang 属性なし", otherTitle)
	}
	otherRow := testutil.Element(t, body, ">Mes derniers disques<", "</p>")
	if !strings.Contains(otherRow, "その他") {
		t.Errorf("other のスレッドの行 = %s, want 「その他」の訳語のバッジ", otherRow)
	}
}
