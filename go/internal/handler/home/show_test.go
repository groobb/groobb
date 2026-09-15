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

// newHandlerは、1つのコミュニティと2つの掲示板 — 書き込まれたものと、まだ
// 書き込まれていないもの — を持つデータベース上にhome Handlerを構築します。スレッドを
// 並べる区画と、スレッドが無いことを伝える区画の両方をページが描画する最小の構成です。
// サイドバーもこのページもカテゴリーを描かないにもかかわらず、書き込まれた掲示板に
// カテゴリーを与えているのは、一覧をカテゴリーを持つ掲示板で動かすためです。
//
// その掲示板のスレッドは3つの言語で書かれています。掲示板は言語で分けないためで、
// ページ自身の言語のスレッドしか持たない区画では、行が自身の言語を述べているかどうかを
// 確かめられません。アプリがロケールを持たない言語で書かれた1本は、どの言語も宣言
// しない行を確かめる先です。
func newHandler(t *testing.T) *home.Handler {
	t.Helper()

	ctx := context.Background()
	db := testutil.SetupDB(t)

	if _, err := db.Writer.ExecContext(ctx, "INSERT INTO communities (id, name) VALUES (1, ?)", "ジャズ喫茶"); err != nil {
		t.Fatalf("communitiesへのINSERTに失敗: %v", err)
	}

	categoryRepo := repository.NewCategoryRepository(db)
	boardRepo := repository.NewBoardRepository(db)

	category, err := categoryRepo.Create(ctx, repository.CreateCategoryInput{Slug: "music", Name: "音楽", Position: 1})
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}
	board, err := boardRepo.Create(ctx, repository.CreateBoardInput{
		CategoryID: &category.ID,
		Slug:       "jazz",
		Name:       "ジャズ・ファンク",
		Position:   1,
	})
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}
	if _, err := boardRepo.Create(ctx, repository.CreateBoardInput{
		Slug:     "quiet",
		Name:     "静かな板",
		Position: 2,
	}); err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}

	// 最終投稿の時刻は現在時刻からの相対で与えます。ページがそれを今からの隔たり
	// として述べるためです。
	now := time.Now()
	createThread(t, ctx, db, board.ID, "モードジャズの話", model.LocaleJa.ThreadLanguage(), 7, now.Add(-5*time.Hour))
	createThread(t, ctx, db, board.ID, "Mes derniers disques", model.ThreadLanguageOther, 2, now.Add(-4*time.Hour))
	createThread(t, ctx, db, board.ID, "Records I picked up", model.LocaleEn.ThreadLanguage(), 5, now.Add(-3*time.Hour))
	createThread(t, ctx, db, board.ID, "最近買ったレコード", model.LocaleJa.ThreadLanguage(), 3, now.Add(-2*time.Hour))

	return newHandlerForDB(db)
}

// createThreadは、実際にスレッドが存在する形 — 最初の投稿を伴い、非正規化列が
// スレッドの投稿を表している状態 — でスレッドを挿入します。一覧はその並び順も、示す
// 2つの事実も、この列から読みます。
func createThread(t *testing.T, ctx context.Context, db *database.DB, boardID model.BoardID, title string, language model.ThreadLanguage, postsCount int, lastPostedAt time.Time) {
	t.Helper()

	threadRepo := repository.NewThreadRepository(db)
	postRepo := repository.NewPostRepository(db)

	thread, err := threadRepo.Create(ctx, repository.CreateThreadInput{BoardID: boardID, Title: title, Language: language})
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}
	post, err := postRepo.Create(ctx, repository.CreatePostInput{ThreadID: thread.ID, Number: 1, Body: title + "の1つ目の投稿"})
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}
	if err := threadRepo.UpdateLastPost(ctx, thread.ID, repository.UpdateThreadLastPostInput{
		PostsCount:   postsCount,
		LastPostID:   post.ID,
		LastPostedAt: lastPostedAt,
	}); err != nil {
		t.Fatalf("UpdateLastPost()のエラー = %v", err)
	}
}

// newHandlerForDBは、渡されたアプリケーションデータベース上にhome Handlerを
// 構築します。
func newHandlerForDB(db *database.DB) *home.Handler {
	return newHandlerForDatabases(db, db)
}

// newHandlerForDatabasesはナビゲーションとホームのUseCaseを別々の
// アプリケーションデータベース上に構築し、最初の読み取りが成功した後に2番目の
// 読み取りだけを失敗させられるようにします。
func newHandlerForDatabases(navigationDB, homeDB *database.DB) *home.Handler {
	getCommunityNavigationUC := usecase.NewGetCommunityNavigationUsecase(
		repository.NewCommunityRepository(navigationDB),
		repository.NewBoardRepository(navigationDB),
		repository.NewRoleRepository(navigationDB),
	)
	getCommunityHomeUC := usecase.NewGetCommunityHomeUsecase(
		repository.NewBoardRepository(homeDB),
		repository.NewThreadRepository(homeDB),
	)

	return home.NewHandler(&config.Config{Env: "dev"}, getCommunityNavigationUC, getCommunityHomeUC)
}

// TestShowはGET /homeがHTTP 200と、サポートする各ロケールについてコミュニティの
// シェルを描画したHTMLボディを返すことを検証します。スキップリンクとその飛び先の
// <main> ランドマーク (それが持つページ見出しで名付けられる)、コミュニティ名と掲示板の
// リンクを運ぶサイドバーのランドマーク、アカウント操作 (設定リンクと、_methodオーバー
// ライドでDELETE /user_sessionに到達するサインアウトフォーム。CSRF hiddenフィールドと
// 確認文言つき)、そして認証背後のこのページが持つnoindexのrobotsメタです。
//
// 併せて、このページ自身である一覧も検証します。掲示板ごとの区画が、その掲示板への
// リンクである掲示板名を見出しに持ち、その掲示板の最新スレッドを最後に投稿されたものから
// 順に並べ、各スレッドが投稿数と最終投稿からの隔たりを述べること、そして誰も書き込んで
// いない掲示板がその旨を伝えることです。
//
// ユーザーと現在のパスは (RequireAuthとCurrentPathMiddlewareがするように) contextに
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
			name:             "日本語",
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
			name:             "英語",
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
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
			}

			if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
				t.Errorf("Content-Type = %q、期待値の接頭辞 = %q", got, "text/html")
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
				// ページ自身の言語は <html> 要素そのもので検証する。以下の行が自身の
				// lang属性を持つため、タグをページ全体から探す形では、文書が何を宣言して
				// いてもスレッドのタイトルかそのバッジで満たされてしまう。
				`<html lang="` + string(tt.locale) + `"`,
			}
			for _, want := range wants {
				if !strings.Contains(body, want) {
					t.Errorf("レスポンスボディに %q が含まれていない", want)
				}
			}

			main := body[strings.Index(body, `id="main"`):]
			if strings.Index(main, "最近買ったレコード") > strings.Index(main, "モードジャズの話") {
				t.Error("スレッドが最終投稿の新しい順に並んでいない")
			}

			// 一覧のアウトラインはh1 → h2 → h3である。掲示板の名前がその区画を、
			// スレッドのタイトルがその中の1行を見出しとして束ねるため、見出しを辿る
			// 訪問者は掲示板へ、そしてその会話へ着く。各見出しは、そのレベルだけが生む
			// マークアップ (見出しが直ちに、その見出しのテキストであるリンクを開く形) で
			// 取り出す。ページの最初のh2が掲示板のものであるとは限らないためで、
			// フラッシュもh2を描画する。どちらのリンクも、ページ内の他の小さなリンクと
			// 共通の最小タッチ領域を保つ。
			boardHeading := testutil.Element(t, main, `<h2 class="text-base font-semibold"><a href="/b/jazz"`, "</h2>")
			if !strings.Contains(boardHeading, "ジャズ・ファンク") {
				t.Errorf("掲示板の見出し = %s、期待値は掲示板へのリンクになった掲示板名", boardHeading)
			}
			threadHeading := testutil.Element(t, main, `<h3 class="font-medium"><a href="/t/`, "</h3>")
			if !strings.Contains(threadHeading, "最近買ったレコード") {
				t.Errorf("スレッドの見出し = %s、期待値はスレッドへのリンクになったタイトル", threadHeading)
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
				t.Errorf("掲示板一覧のラベルを持つ要素 = %s、期待値 = nav", nav)
			}
			if strings.Contains(body, `href="/c/music"`) {
				t.Error("サイドバーにカテゴリーへのリンクが含まれている (掲示板はフラットに並べる)")
			}
			sidebar := testutil.OpeningTag(t, body, `aria-label="`+tt.wantSidebarLabel+`"`)
			if !strings.HasPrefix(sidebar, "<aside ") {
				t.Errorf("サイドバーの要素 = %s、期待値 = aside", sidebar)
			}
			mainTag := testutil.OpeningTag(t, body, `id="main"`)
			if !strings.HasPrefix(mainTag, "<main ") || !strings.Contains(mainTag, `aria-labelledby="home-show-heading"`) {
				t.Errorf("main landmark = %s、ページの見出しをアクセシブルネームに持つことを期待", mainTag)
			}
			heading := testutil.OpeningTag(t, body, `id="home-show-heading"`)
			if !strings.HasPrefix(heading, "<h1 ") {
				t.Errorf("main landmarkを名付ける要素 = %s、期待値 = h1", heading)
			}
			if got := strings.Count(body, "<aside"); got != 1 {
				t.Errorf("asideの数 = %d、期待値 = %d (このページは補足のカラムを持たない)", got, 1)
			}
			if got := strings.Count(body, "<h1"); got != 1 {
				t.Errorf("h1の数 = %d、期待値 = 1", got)
			}
			beforeMain := body[:strings.Index(body, `id="main"`)]
			if strings.Contains(beforeMain, "<h2") || strings.Contains(beforeMain, "<h3") {
				t.Error("サイドバーに本文の見出し階層へ入るh2またはh3が含まれている")
			}
		})
	}
}

// TestShow_EmptyInstanceは、コミュニティの内容がないインスタンスでも板の
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
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "<nav ") {
		t.Error("空のインスタンスのレスポンスにnavが含まれていない")
	}
	if strings.Contains(body, `href="/home"`) {
		t.Error("空のインスタンスのレスポンスにホームリンクが含まれている")
	}
	if !strings.Contains(body, "このコミュニティにはまだ掲示板がありません。") {
		t.Error("掲示板を1つも持たないコミュニティの空状態が表示されていない")
	}
	boards := testutil.Element(t, body, `id="sidebar-boards-label"`, "</nav>")
	if !strings.Contains(boards, "まだ掲示板がありません。") {
		t.Error("サイドバーの掲示板一覧に空状態が表示されていない")
	}
}

// TestShow_NavigationFailureは、コミュニティナビゲーションの取得失敗が部分的な
// ページではなくInternal Server Errorとして返ることを検証します。
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
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(rec.Body.String(), "Internal Server Error") {
		t.Error("レスポンスボディにInternal Server Errorが含まれていない")
	}
}

// TestShow_CommunityHomeFailureは、ナビゲーションの取得成功後にコミュニティの
// ホーム取得が失敗した場合、サイドバーだけを含むページではなくInternal Server Errorが
// 返ることを検証します。
func TestShow_CommunityHomeFailure(t *testing.T) {
	t.Parallel()

	navigationDB := testutil.SetupDB(t)
	homeDB := testutil.SetupDB(t)
	if err := homeDB.Reader.Close(); err != nil {
		t.Fatalf("ReaderのClose()のエラー = %v", err)
	}

	handler := newHandlerForDatabases(navigationDB, homeDB)
	req := httptest.NewRequest(http.MethodGet, "/home", nil)
	ctx := middleware.SetUserToContext(req.Context(), &model.User{Atname: "alice"})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.Show(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(rec.Body.String(), "Internal Server Error") {
		t.Error("レスポンスボディにInternal Server Errorが含まれていない")
	}
}

// TestShow_MarksOnlyTheCurrentPageは、サイドバーがaria-currentを付けるのが
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
		t.Errorf("aria-current=\"page\" の数 = %d、期待値 = %d (ホームを指すリンクのみ)", got, 1)
	}
	if boardLink := testutil.OpeningTag(t, body, `href="/b/jazz"`); strings.Contains(boardLink, "aria-current") {
		t.Errorf("掲示板のリンクにaria-currentが付いている: %s", boardLink)
	}
}

// TestShow_ThreadLanguageは、掲示板の区画の各行が自身のスレッドの言語を述べる
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

	// 宣言はタイトル自身の要素が持つ。タグが覆うのはタイトルであって、行の他の
	// ものではない。
	link := testutil.OpeningTag(t, body, ">Records I picked up<")
	if !strings.Contains(link, `lang="en"`) {
		t.Errorf("英語のスレッドのタイトル = %s、期待値はlang=\"en\"を持つ要素", link)
	}

	row := testutil.Element(t, body, ">Records I picked up<", "</p>")
	for _, want := range []string{`<span class="sr-only">主言語:</span>`, `<span lang="en">English</span>`} {
		if !strings.Contains(row, want) {
			t.Errorf("英語のスレッドの行 = %s、%q を含むことを期待", row, want)
		}
	}

	// バッジが付くのは別の言語の行だけではなく、どの行にも付く。バッジが無いことを
	// 「これは自分の言語だ」と読む必要が生じないようにするため。
	if !strings.Contains(body, `<span lang="ja">日本語</span>`) {
		t.Error("日本語のスレッドにバッジが無い")
	}

	// アプリがロケールを持たない言語で書かれたスレッドは、どの言語も宣言しない
	// 唯一の行である。宣言するタグが無く、でっち上げたタグは、そのタイトルが書かれて
	// いない言語の規則でスクリーンリーダーに発音させることになるため、バッジは代わりに
	// 訳語を載せる。
	otherTitle := testutil.OpeningTag(t, body, ">Mes derniers disques<")
	if strings.Contains(otherTitle, "lang=") {
		t.Errorf("otherのスレッドのタイトル = %s、期待値はlang属性なし", otherTitle)
	}
	otherRow := testutil.Element(t, body, ">Mes derniers disques<", "</p>")
	if !strings.Contains(otherRow, "その他") {
		t.Errorf("otherのスレッドの行 = %s、期待値は「その他」の訳語のバッジ", otherRow)
	}
}

// TestShow_AdminLinkは、サイドバーのローカライズされた管理画面への導線が、それを
// 許されたアカウントにだけ描かれること、そして行き先が管理ハブであることを検証します。
// サイドバーはコミュニティのどのページにも出るため、全員に描くリンクは、コミュニティの
// 大半に拒否で応じる入口になってしまいます。
//
// サイドバーのコンポーネントではなくここで検証するのは、コンポーネントが受け取るものが、
// このアカウントの持つロールを読むUseCaseから来るためです。検証したいのはデータベースの
// 割当からマークアップまでの経路の全体であり、ホームはサインイン済みの訪問者が必ず着く
// コミュニティのページです。
func TestShow_AdminLink(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		locale            model.Locale
		grantAdminRole    bool
		wantAdminLinkText string
	}{
		{
			name:              "adminロールを持つ利用者には日本語の管理リンクが出る",
			locale:            model.LocaleJa,
			grantAdminRole:    true,
			wantAdminLinkText: "管理",
		},
		{
			name:              "adminロールを持つ利用者には英語の管理リンクが出る",
			locale:            model.LocaleEn,
			grantAdminRole:    true,
			wantAdminLinkText: "Admin",
		},
		{
			name:   "ロールを持たない利用者には日本語でも管理リンクが出ない",
			locale: model.LocaleJa,
		},
		{
			name:   "ロールを持たない利用者には英語でも管理リンクが出ない",
			locale: model.LocaleEn,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			db := testutil.SetupDB(t)
			handler := newHandlerForDB(db)
			userID := testutil.NewUserBuilder(t, db).Build()
			if tt.grantAdminRole {
				testutil.NewUserRoleBuilder(t, db).WithUserID(userID).Build()
			}

			req := httptest.NewRequest(http.MethodGet, "/home", nil)
			ctx := i18n.SetLocale(req.Context(), tt.locale)
			ctx = middleware.SetUserToContext(ctx, &model.User{ID: userID, Atname: "alice"})
			ctx = templates.SetCurrentPath(ctx, templates.HomePath().String())
			req = req.WithContext(ctx)
			rec := httptest.NewRecorder()

			handler.Show(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
			}

			body := rec.Body.String()
			// 設定のリンクも併せて検証する。どちらも無いボディ (アカウントのブロックを
			// そもそも描けなかったサイドバー) が、管理のリンクが正しく無い場合として通って
			// しまわないようにするためである。
			if !strings.Contains(body, `href="/settings"`) {
				t.Error("サイドバーに設定へのリンクが無い")
			}
			hasAdminLink := strings.Contains(body, `href="/admin"`)
			wantAdminLink := tt.wantAdminLinkText != ""
			if hasAdminLink != wantAdminLink {
				t.Errorf(`ボディにhref="/admin"が含まれるか = %t、期待値 = %t`, hasAdminLink, wantAdminLink)
			}
			if wantAdminLink {
				adminLink := testutil.Element(t, body, `href="/admin"`, "</a>")
				if !strings.HasSuffix(adminLink, ">"+tt.wantAdminLinkText) {
					t.Errorf("管理リンク = %s、期待値は文言 %q", adminLink, tt.wantAdminLinkText)
				}
			}
		})
	}
}
