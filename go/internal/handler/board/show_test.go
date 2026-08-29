package board_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/handler/board"
	"github.com/groobb/groobb/go/internal/httperror"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/templates"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// appURL is the instance's public base URL in these tests, the value the page's
// canonical link is built under.
//
// [Ja] appURL は本テストでのインスタンスの公開ベース URL であり、ページの canonical の
// リンクがその下で組み立てられる値です。
const appURL = "https://groobb.example.com"

// communityName is the name of the community this instance hosts in these
// tests. The pages read it from the request context, where the middleware puts
// it in production, and end their titles with it.
//
// [Ja] communityName は本テストでこのインスタンスが運営するコミュニティの名前です。
// ページはこれをリクエストの context から読み (本番ではミドルウェアがそこへ置きます)、
// タイトルの末尾に置きます。
const communityName = "ジャズ喫茶"

// newHandler builds the board Handler over a database holding one community
// whose "music" category lists two boards: "jazz", which holds two threads
// posted in the reverse of the order the listing puts them in, and "quiet",
// which holds none. Between them the two cover what the page renders — a listing
// ordered by when each thread was last posted in, and the state where nobody has
// started one.
//
// [Ja] newHandler は、1 つのコミュニティを持つデータベース上に board Handler を構築
// します。その "music" カテゴリーは 2 つの掲示板を並べます。"jazz" は一覧が並べるのとは
// 逆の順序で投稿された 2 つのスレッドを持ち、"quiet" は 1 つも持ちません。この 2 つで、
// このページが描画するもの — 各スレッドが最後に投稿された時刻による並び順と、誰もまだ
// スレッドを立てていない状態 — を覆えます。
func newHandler(t *testing.T) *board.Handler {
	t.Helper()

	ctx := context.Background()
	db := testutil.SetupDB(t)

	if _, err := db.Writer.ExecContext(ctx, "INSERT INTO communities (id, name) VALUES (1, ?)", communityName); err != nil {
		t.Fatalf("communities への INSERT に失敗: %v", err)
	}

	categoryRepo := repository.NewCategoryRepository(db)
	boardRepo := repository.NewBoardRepository(db)
	threadRepo := repository.NewThreadRepository(db)
	postRepo := repository.NewPostRepository(db)

	music, err := categoryRepo.Create(ctx, repository.CreateCategoryInput{Slug: "music", Name: "音楽", Position: 1})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	jazz, err := boardRepo.Create(ctx, repository.CreateBoardInput{
		CategoryID:  &music.ID,
		Slug:        "jazz",
		Name:        "ジャズ・ファンク",
		Description: "ジャズの話をする板",
		Position:    1,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := boardRepo.Create(ctx, repository.CreateBoardInput{
		CategoryID: &music.ID,
		Slug:       "quiet",
		Name:       "準備中の板",
		Position:   2,
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	createThread := func(title string, postsCount int, lastPostedAt time.Time) {
		t.Helper()
		thread, err := threadRepo.Create(ctx, repository.CreateThreadInput{BoardID: jazz.ID, Title: title})
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
	createThread("枯葉の名演", 3, time.Now().Add(-48*time.Hour))
	createThread("最近買ったレコード", 42, time.Now().Add(-30*time.Minute))

	return newHandlerForDB(db)
}

// newHandlerForDB builds the board Handler over the supplied application
// database.
//
// [Ja] newHandlerForDB は、渡されたアプリケーションデータベース上に board Handler を
// 構築します。
func newHandlerForDB(db *database.DB) *board.Handler {
	return newHandlerForDatabases(db, db, db)
}

// newHandlerForDatabases builds the board Handler with a separate database
// behind each of its three reads: resolving the board, the community navigation,
// and the thread listing. Production passes the same database for all three;
// tests can break one read path without preventing the others from reaching the
// branch under test.
//
// [Ja] newHandlerForDatabases は、3 つの読み取り (掲示板の解決・コミュニティの
// ナビゲーション・スレッドの一覧) それぞれの背後に別々のデータベースを置いて board
// Handler を構築します。本番は 3 つとも同じデータベースを渡しますが、テストでは 1 つの
// 読み取りだけを壊し、残りが対象分岐へ到達できます。
func newHandlerForDatabases(boardDB, navigationDB, threadDB *database.DB) *board.Handler {
	getCommunityNavigationUC := usecase.NewGetCommunityNavigationUsecase(
		repository.NewCommunityRepository(navigationDB),
		repository.NewBoardRepository(navigationDB),
	)
	getBoardUC := usecase.NewGetBoardUsecase(repository.NewBoardRepository(boardDB), repository.NewCategoryRepository(boardDB))
	getBoardThreadsUC := usecase.NewGetBoardThreadsUsecase(repository.NewThreadRepository(threadDB))

	cfg := &config.Config{Env: "dev", AppURL: appURL}
	return board.NewHandler(cfg, httperror.NewRenderer(cfg), getCommunityNavigationUC, getBoardUC, getBoardThreadsUC)
}

// newRequest builds a GET /b/{slug} request as the router would hand it to the
// handler: the slug in chi's route context, and the locale, the current path and
// the viewer in the request context, placed there directly the way i18n's,
// templates' and the auth middleware would. A nil user is an anonymous visitor.
//
// [Ja] newRequest は、ルーターがハンドラーへ渡すのと同じ形で GET /b/{slug} の
// リクエストを組み立てます。slug は chi のルート context に、ロケール・現在のパス・
// 閲覧者はリクエスト context に、i18n・templates・認証の各ミドルウェアがするのと同じ
// ように直接置きます。user が nil のときは匿名の訪問者です。
func newRequest(t *testing.T, slug string, locale model.Locale, user *model.User) *http.Request {
	t.Helper()

	path := templates.BoardPath(slug).String()
	req := httptest.NewRequest(http.MethodGet, path, nil)

	ctx := i18n.SetLocale(req.Context(), locale)
	ctx = templates.SetCurrentPath(ctx, path)
	ctx = viewmodel.SetSiteName(ctx, communityName)
	if user != nil {
		ctx = middleware.SetUserToContext(ctx, user)
	}

	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("slug", slug)
	ctx = context.WithValue(ctx, chi.RouteCtxKey, routeCtx)

	return req.WithContext(ctx)
}

// TestShow verifies that GET /b/{slug} returns HTTP 200 with an HTML body that
// renders, for each supported locale, the board page inside the community shell:
// the board's name as the <h1> naming the <main> landmark, the breadcrumb naming
// the category that lists it, the threads with their post counts and last-post
// times ordered by when each was last posted in, the sidebar and its account
// controls, and the complementary column standing in for a thread that has not
// been opened. The page carries no noindex, since a community's boards are
// public.
//
// [Ja] TestShow は GET /b/{slug} が HTTP 200 と、サポートする各ロケールについて
// コミュニティのシェルの中に掲示板ページを描画した HTML ボディを返すことを検証します。
// <main> ランドマークを名付ける <h1> としての掲示板名、それを並べるカテゴリーを名指す
// パンくず、最後に投稿された順に並ぶスレッドとその投稿数・最終投稿時刻、サイドバーと
// そのアカウント操作、そしてまだ開かれていないスレッドの代わりを務める補助カラムです。
// コミュニティの掲示板は公開であるため、このページは noindex を持ちません。
func TestShow(t *testing.T) {
	t.Parallel()

	handler := newHandler(t)

	tests := []struct {
		name            string
		locale          model.Locale
		wantPostsCount  string
		wantLastPosted  string
		wantRegionLabel string
		wantPrompt      string
	}{
		{
			name:            "Japanese",
			locale:          model.LocaleJa,
			wantPostsCount:  "42 件の投稿",
			wantLastPosted:  "30 分前",
			wantRegionLabel: "スレッドの閲覧",
			wantPrompt:      "スレッドを選ぶと、その投稿が表示されます。",
		},
		{
			name:            "English",
			locale:          model.LocaleEn,
			wantPostsCount:  "42 posts",
			wantLastPosted:  "30 minutes ago",
			wantRegionLabel: "Reading a thread",
			wantPrompt:      "Choose a thread to see the posts in it.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			handler.Show(rec, newRequest(t, "jazz", tt.locale, &model.User{Atname: "alice"}))

			if rec.Code != http.StatusOK {
				t.Errorf("status code = %d, want %d", rec.Code, http.StatusOK)
			}
			if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
				t.Errorf("Content-Type = %q, want prefix %q", got, "text/html")
			}

			body := rec.Body.String()
			wants := []string{
				"<title>ジャズ・ファンク - " + communityName + "</title>",
				`content="ジャズの話をする板"`,
				tt.wantPrompt,
				tt.wantPostsCount,
				tt.wantLastPosted,
				"最近買ったレコード",
				"枯葉の名演",
				"ジャズ喫茶",
				`href="/settings"`,
				`action="/user_session"`,
				`lang="` + string(tt.locale) + `"`,
			}
			for _, want := range wants {
				if !strings.Contains(body, want) {
					t.Errorf("response body does not contain %q", want)
				}
			}

			// A thread's title is the link to it, so the listing is how the board's
			// conversations are reached. The heading is taken by the markup only a
			// thread row produces — an h2 opening straight into a link to /t/ —
			// because the page's first h2 is not necessarily this one: a flash and a
			// form's error summary render an h2 as well. The same link keeps the
			// minimum touch target used by the page's other compact links.
			//
			// [Ja] スレッドのタイトルはそれへのリンクであり、一覧が掲示板の会話へ辿り着く
			// 手立てとなる。見出しは、スレッドの行だけが生むマークアップ (h2 が直ちに /t/ への
			// リンクを開く形) で取り出す。ページの最初の h2 がこれであるとは限らないためで、
			// フラッシュとフォームのエラー要約も h2 を描画する。同じリンクは、ページ内の他の
			// 小さなリンクと共通の最小タッチ領域も保つ。
			threadHeading := testutil.Element(t, body, `<h2><a href="/t/`, "</h2>")
			if !strings.Contains(threadHeading, "最近買ったレコード") {
				t.Errorf("スレッドの見出し = %s, want the title as a link to the thread", threadHeading)
			}
			threadLink := testutil.OpeningTag(t, threadHeading, `href="/t/`)
			for _, want := range []string{"inline-flex", "min-h-6", "min-w-6"} {
				if !strings.Contains(threadLink, want) {
					t.Errorf("スレッドのタイトルリンクに %q が無い: %s", want, threadLink)
				}
			}

			if strings.Contains(body, "noindex") {
				t.Error("公開ページのレスポンスに noindex が含まれている")
			}

			main := testutil.OpeningTag(t, body, `id="main"`)
			if !strings.HasPrefix(main, "<main ") || !strings.Contains(main, `aria-labelledby="board-show-heading"`) {
				t.Errorf("main landmark = %s, want the page heading as its accessible name", main)
			}
			heading := testutil.OpeningTag(t, body, `id="board-show-heading"`)
			if !strings.HasPrefix(heading, "<h1 ") {
				t.Errorf("main landmark を名付ける要素 = %s, want h1", heading)
			}
			aside := testutil.OpeningTag(t, body, `aria-label="`+tt.wantRegionLabel+`"`)
			if !strings.HasPrefix(aside, "<aside ") {
				t.Errorf("スレッド領域の要素 = %s, want aside", aside)
			}

			// The exact instant sits in the datetime attribute beside the text
			// saying how long ago it was, so the moment survives the rounding the
			// relative form does. The element carrying the attribute is looked up
			// by the attribute itself, so the assertion also says it is a <time>
			// rather than some other element that happens to hold a date.
			//
			// [Ja] 正確な時点は、どれだけ前かを述べるテキストの傍らの datetime 属性に
			// 置かれ、相対表現が行う丸めを越えて残る。属性そのものを手がかりに要素を
			// 引くことで、日付を持つ別の要素ではなく <time> であることも併せて検証する。
			if timeTag := testutil.OpeningTag(t, body, "datetime="); !strings.HasPrefix(timeTag, "<time ") {
				t.Errorf("datetime 属性を持つ要素 = %s, want time", timeTag)
			}

			// Both threads appear in the list column, so comparing their positions
			// checks that the most recently posted-in one comes first.
			//
			// [Ja] どちらのスレッドも一覧カラムに現れるため、その位置を比べることで、
			// 最後に投稿されたものが先に来ることを確かめられる。
			if got, want := strings.Index(body, "最近買ったレコード"), strings.Index(body, "枯葉の名演"); got > want {
				t.Error("スレッドが最終投稿の新しい順に並んでいない")
			}
		})
	}
}

// TestShow_Breadcrumb verifies that the page says where the board sits: a
// breadcrumb naming the category that lists it, linked, followed by the board
// itself marked as the current step and left unlinked. /b/{slug} carries nothing
// about the category, so without this the visitor has no way to tell.
//
// [Ja] TestShow_Breadcrumb は、ページが掲示板の在り処を述べることを検証します。それを
// 並べるカテゴリーを名指すリンク付きのパンくずと、続く掲示板自身であり、後者は現在地の
// 印を付けてリンクにしません。/b/{slug} はカテゴリーについて何も運ばないため、これが
// 無いと訪問者には知る手立てがありません。
func TestShow_Breadcrumb(t *testing.T) {
	t.Parallel()

	handler := newHandler(t)
	rec := httptest.NewRecorder()
	handler.Show(rec, newRequest(t, "jazz", model.LocaleJa, &model.User{Atname: "alice"}))

	body := rec.Body.String()

	nav := testutil.OpeningTag(t, body, `aria-label="パンくず"`)
	if !strings.HasPrefix(nav, "<nav ") || !strings.Contains(nav, `class="breadcrumb `) {
		t.Errorf("パンくずの要素 = %s, want nav.breadcrumb", nav)
	}
	trail := body[strings.Index(body, `aria-label="パンくず"`):]
	trail = trail[:strings.Index(trail, "</nav>")]
	if !strings.Contains(trail, `href="/c/music"`) {
		t.Error("パンくずにカテゴリーページへのリンクが無い")
	}

	// Only the classes the touch-target rule rests on are asserted, so a change
	// to how the link looks does not fail a test about the response. A category
	// named with one character would otherwise be narrower than the minimum a
	// finger can reliably hit.
	//
	// [Ja] タッチターゲットの要件が拠って立つクラスだけを検証する。リンクの見た目の
	// 変更が、応答についてのテストを落とさないようにするため。1 文字の名前を持つ
	// カテゴリーでは、これが無いと指で確実に押せる最小の幅を下回る。
	link := testutil.OpeningTag(t, trail, `href="/c/music"`)
	for _, want := range []string{"inline-flex", "min-h-6", "min-w-6"} {
		if !strings.Contains(link, want) {
			t.Errorf("パンくずのリンクに %q が無い: %s", want, link)
		}
	}
	if got, want := strings.Count(trail, `<li aria-hidden="true">`), 1; got != want {
		t.Errorf("パンくずの非表示区切り数 = %d, want %d", got, want)
	}
	if !strings.Contains(trail, "data-rtl-flip") {
		t.Error("パンくずの区切りに RTL 反転の印が無い")
	}
	current := testutil.OpeningTag(t, trail, `aria-current="page"`)
	if !strings.HasPrefix(current, "<span ") {
		t.Errorf("パンくずの現在地の要素 = %s, want span (リンクにしない)", current)
	}
}

// TestShow_DeclaresItsCanonicalURLAndPublishesItsTrail verifies that the page
// declares its own address as the one it is to be known by, and publishes the
// trail it draws as BreadcrumbList structured data naming each linked step
// absolutely. A search result can then show where the board sits instead of its
// bare URL, and the same page reached with a campaign parameter appended is not
// counted as a second one.
//
// The structured data is asserted here rather than only in the component's own
// test because this is where the base URL reaches it: a handler that stopped
// passing it would leave the component correct and the page silent.
//
// [Ja] TestShow_DeclaresItsCanonicalURLAndPublishesItsTrail は、ページが自身を知られる
// べきアドレスとして自身のアドレスを宣言すること、そして描いた経路を、リンクを持つ各段を
// 絶対 URL で名指す BreadcrumbList の構造化データとして公開することを検証します。これに
// より検索結果は、素の URL ではなく掲示板の在り処を示せます。またキャンペーンのパラメータ
// を付けて到達した同じページが、2 つ目のページとして数えられません。
//
// 構造化データをコンポーネント自身のテストだけでなくここでも検証するのは、ベース URL が
// そこへ届くのがこの経路だからです。ハンドラーがそれを渡さなくなっても、コンポーネントは
// 正しいままページだけが黙ります。
func TestShow_DeclaresItsCanonicalURLAndPublishesItsTrail(t *testing.T) {
	t.Parallel()

	handler := newHandler(t)
	rec := httptest.NewRecorder()
	handler.Show(rec, newRequest(t, "jazz", model.LocaleJa, nil))

	body := rec.Body.String()

	canonical := testutil.OpeningTag(t, body, `rel="canonical"`)
	if want := `href="` + appURL + "/b/jazz" + `"`; !strings.Contains(canonical, want) {
		t.Errorf("canonical のリンク = %s, want %s を含む", canonical, want)
	}

	testutil.AssertBreadcrumbList(t, body,
		[]string{"音楽", "ジャズ・ファンク"},
		[]string{appURL + "/c/music", ""},
	)
}

// TestShow_BreadcrumbWithoutACategory verifies that a board sitting in no
// category renders no breadcrumb at all. There is no place above it to name
// (ADR 0011), and a trail holding only the page being rendered would repeat the
// heading without telling the visitor anything about where they are.
//
// [Ja] TestShow_BreadcrumbWithoutACategory は、どのカテゴリーにも属さない掲示板の
// ページがパンくずを一切描画しないことを検証します。上位として名指す場所が無く
// (ADR 0011)、今描画しているページだけの経路は、訪問者の居場所について何も伝えずに
// 見出しを繰り返すだけになるためです。
func TestShow_BreadcrumbWithoutACategory(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testutil.SetupDB(t)
	if _, err := repository.NewBoardRepository(db).Create(ctx, repository.CreateBoardInput{
		Slug: "jazz",
		Name: "ジャズ・ファンク",
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	rec := httptest.NewRecorder()
	newHandlerForDB(db).Show(rec, newRequest(t, "jazz", model.LocaleJa, &model.User{Atname: "alice"}))

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	if strings.Contains(body, `aria-label="パンくず"`) {
		t.Error("カテゴリーを持たない掲示板のページにパンくずが描画されている")
	}
	if strings.Contains(body, "ld+json") {
		t.Error("経路を持たない掲示板のページに BreadcrumbList の構造化データが描画されている")
	}
	if !strings.Contains(body, "ジャズ・ファンク") {
		t.Error("掲示板の名前がページに含まれていない")
	}
}

// TestShow_RedirectsCaseVariantToCanonicalSlug verifies that the NOCASE lookup
// does not turn a differently cased path into a second HTTP 200 URL. The stored
// lowercase slug is the canonical address, and a permanent redirect moves both
// visitors and crawlers there before the page is rendered.
//
// The Cache-Control of the redirect is asserted alongside the status, because a
// permanent redirect can be held by the visitor's browser, while the CSRF cookie
// a safe request may mint must keep it out of shared caches.
//
// [Ja] TestShow_RedirectsCaseVariantToCanonicalSlug は、NOCASE 検索によって大小だけが
// 異なるパスが 2 つ目の HTTP 200 URL にならないことを検証します。保存済みの小文字 slug
// が正規アドレスであり、ページを描画する前に恒久リダイレクトで訪問者とクローラーをそこへ
// 移します。
//
// リダイレクトの Cache-Control をステータスと併せて検証するのは、恒久リダイレクトを
// 訪問者のブラウザには保持させながら、安全なリクエストが発行しうる CSRF Cookie を
// 共有キャッシュには保存させないためです。
func TestShow_RedirectsCaseVariantToCanonicalSlug(t *testing.T) {
	t.Parallel()

	handler := newHandler(t)
	rec := httptest.NewRecorder()
	req := newRequest(t, "JAZZ", model.LocaleJa, &model.User{Atname: "alice"})
	req.URL.RawQuery = "utm_source=newsletter"

	handler.Show(rec, req)

	if rec.Code != http.StatusPermanentRedirect {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusPermanentRedirect)
	}
	if got, want := rec.Header().Get("Location"), templates.BoardPath("jazz").String()+"?utm_source=newsletter"; got != want {
		t.Errorf("Location = %q, want %q", got, want)
	}
	if got, want := rec.Header().Get("Cache-Control"), "private, max-age=3600"; got != want {
		t.Errorf("Cache-Control = %q, want %q", got, want)
	}
}

// TestShow_AnonymousVisitor verifies that a signed-out visitor is served the
// board and its threads, that the sidebar's account controls are left out, and
// that the way into an account stands in their place. The community's pages are
// readable without an account, so the page must not depend on there being one —
// and this board page is one of the places a visitor decides to join from, so
// the sign-in link carries the board back to them once they have.
//
// [Ja] TestShow_AnonymousVisitor は、サインアウト状態の訪問者にも掲示板とそのスレッドが
// 届くこと、サイドバーのアカウント操作が描画されないこと、そしてその位置にアカウントを
// 持つための導線が立つことを検証します。コミュニティのページはアカウント無しで読めるため、
// ページがアカウントの存在に依存してはなりません。そしてこの掲示板のページは訪問者が参加を
// 決める場所の 1 つであるため、サインインのリンクは参加した訪問者をこの掲示板へ連れ戻します。
func TestShow_AnonymousVisitor(t *testing.T) {
	t.Parallel()

	handler := newHandler(t)
	rec := httptest.NewRecorder()
	handler.Show(rec, newRequest(t, "jazz", model.LocaleJa, nil))

	if rec.Code != http.StatusOK {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "最近買ったレコード") {
		t.Error("匿名の訪問者のレスポンスにスレッドのタイトルが含まれていない")
	}
	for _, unwanted := range []string{`href="/settings"`, `action="/user_session"`, `name="csrf_token"`} {
		if strings.Contains(body, unwanted) {
			t.Errorf("匿名の訪問者のレスポンスに %q が含まれている", unwanted)
		}
	}
	signInHref := templates.SignInPath().WithReturnTo(templates.BoardPath("jazz").String()).String()
	for _, want := range []string{`href="` + signInHref + `"`, `href="` + templates.SignUpPath().String() + `"`} {
		if !strings.Contains(body, want) {
			t.Errorf("匿名の訪問者のレスポンスに %q が含まれていない", want)
		}
	}
}

// TestShow_EmptyBoard verifies that a board nobody has posted in says so, rather
// than rendering a heading above nothing, and that the reading column stops
// inviting the visitor to choose from a list that has nothing in it. A board
// without a description of its own still gets a meta description naming it,
// instead of falling back to the site-wide default every board would share.
//
// [Ja] TestShow_EmptyBoard は、まだ誰も書き込んでいない掲示板が、見出しの下に何も無い
// 状態ではなくその旨を伝えること、そして読むためのカラムが、何も入っていない一覧から
// 選ぶよう促すのをやめることを検証します。自身の説明を持たない掲示板も、どの掲示板でも
// 同じになるサイト全体の既定値ではなく、それを名指す meta description を得ます。
func TestShow_EmptyBoard(t *testing.T) {
	t.Parallel()

	handler := newHandler(t)
	rec := httptest.NewRecorder()
	handler.Show(rec, newRequest(t, "quiet", model.LocaleJa, &model.User{Atname: "alice"}))

	if rec.Code != http.StatusOK {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "この掲示板にはまだスレッドがありません。") {
		t.Error("スレッドを持たない掲示板のレスポンスに空状態の文言が含まれていない")
	}
	if !strings.Contains(body, "スレッドが立つと、その投稿がここに表示されます。") {
		t.Error("スレッドを持たない掲示板の読むためのカラムに、空状態の文言が含まれていない")
	}
	if strings.Contains(body, "スレッドを選ぶと、その投稿が表示されます。") {
		t.Error("スレッドを持たない掲示板の読むためのカラムが、選べないスレッドを選ぶよう促している")
	}
	if !strings.Contains(body, `content="準備中の板 のスレッドの一覧です。"`) {
		t.Error("説明を持たない掲示板のレスポンスに、掲示板を名指す meta description が含まれていない")
	}
}

// TestShow_UnknownSlug verifies that a slug naming no board is answered with
// HTTP 404 and the shared not-found page. The status is asserted alongside the
// body because a page that reads as "not found" while answering 200 is a soft
// 404, and this route is reachable by crawlers following a link to a board that
// has since been removed.
//
// [Ja] TestShow_UnknownSlug は、どの掲示板も指さない slug が HTTP 404 と共通の
// not-found ページで応答されることを検証します。ステータスをボディと併せて検証するのは、
// 「見つからない」と読めるページが 200 で応答する状態がソフト 404 だからです。そして
// このルートには、削除済みの掲示板へのリンクを辿るクローラーが到達しえます。
func TestShow_UnknownSlug(t *testing.T) {
	t.Parallel()

	handler := newHandler(t)
	rec := httptest.NewRecorder()
	handler.Show(rec, newRequest(t, "no-such-board", model.LocaleJa, &model.User{Atname: "alice"}))

	if rec.Code != http.StatusNotFound {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if body := rec.Body.String(); !strings.Contains(body, "ページが見つかりません") {
		t.Error("未知の slug のレスポンスに 404 ページの見出しが含まれていない")
	}
}

// TestShow_LookupFailure verifies that a failure to read the board is returned
// as an internal server error rather than as a 404: a database that cannot be
// reached does not mean the board is gone, and answering 404 would tell a
// crawler to drop a page that still exists.
//
// [Ja] TestShow_LookupFailure は、掲示板の読み取りの失敗が 404 ではなく Internal
// Server Error として返ることを検証します。到達できないデータベースは掲示板が無く
// なったことを意味せず、404 で応答すればまだ存在するページを落とすようクローラーに
// 伝えてしまいます。
func TestShow_LookupFailure(t *testing.T) {
	t.Parallel()

	handler := newHandler(t)
	req := newRequest(t, "jazz", model.LocaleJa, &model.User{Atname: "alice"})
	ctx, cancel := context.WithCancel(req.Context())
	cancel()
	rec := httptest.NewRecorder()

	handler.Show(rec, req.WithContext(ctx))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(rec.Body.String(), "Internal Server Error") {
		t.Error("response body does not contain Internal Server Error")
	}
}

// TestShow_NavigationLookupFailure verifies the second database failure branch:
// board lookup succeeds, then navigation lookup fails and returns an internal
// server error. Keeping the databases separate prevents the first lookup from
// consuming the intended failure.
//
// [Ja] TestShow_NavigationLookupFailure は 2 つ目の DB 失敗分岐を検証します。掲示板の
// 取得には成功し、その後のナビゲーション取得が失敗して Internal Server Error を返します。
// DB を分けることで、最初の取得が対象の失敗を先に消費しないようにします。
func TestShow_NavigationLookupFailure(t *testing.T) {
	t.Parallel()

	boardDB := testutil.SetupDB(t)
	createJazzBoard(t, boardDB)

	navigationDB := testutil.SetupDB(t)
	if err := navigationDB.Reader.Close(); err != nil {
		t.Fatalf("navigation Reader の Close() error = %v", err)
	}

	handler := newHandlerForDatabases(boardDB, navigationDB, boardDB)
	rec := httptest.NewRecorder()
	handler.Show(rec, newRequest(t, "jazz", model.LocaleJa, &model.User{Atname: "alice"}))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(rec.Body.String(), "Internal Server Error") {
		t.Error("response body does not contain Internal Server Error")
	}
}

// TestShow_ThreadListingFailure verifies the third database failure branch:
// resolving the board and reading the navigation both succeed, then the thread
// listing fails and returns an internal server error. Splitting the listing out
// of the board lookup gave this failure its own branch, so it needs a case of
// its own to stay covered.
//
// [Ja] TestShow_ThreadListingFailure は 3 つ目の DB 失敗分岐を検証します。掲示板の解決と
// ナビゲーションの読み取りには成功し、その後のスレッド一覧の取得が失敗して Internal
// Server Error を返します。一覧を掲示板の解決から切り離したことでこの失敗が独立した
// 分岐になったため、覆い続けるには独立したケースが要ります。
func TestShow_ThreadListingFailure(t *testing.T) {
	t.Parallel()

	boardDB := testutil.SetupDB(t)
	createJazzBoard(t, boardDB)

	threadDB := testutil.SetupDB(t)
	if err := threadDB.Reader.Close(); err != nil {
		t.Fatalf("thread Reader の Close() error = %v", err)
	}

	handler := newHandlerForDatabases(boardDB, boardDB, threadDB)
	rec := httptest.NewRecorder()
	handler.Show(rec, newRequest(t, "jazz", model.LocaleJa, &model.User{Atname: "alice"}))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(rec.Body.String(), "Internal Server Error") {
		t.Error("response body does not contain Internal Server Error")
	}
}

// createJazzBoard creates the board the failure cases resolve, together with the
// category it has to belong to, so that a case about a later read reaches it.
//
// [Ja] createJazzBoard は、失敗のケースが解決する掲示板を、それが属さなければならない
// カテゴリーと一緒に作ります。後続の読み取りについてのケースがそこへ到達できるように
// するためです。
func createJazzBoard(t *testing.T, db *database.DB) {
	t.Helper()

	ctx := context.Background()
	category, err := repository.NewCategoryRepository(db).Create(ctx, repository.CreateCategoryInput{Slug: "music", Name: "音楽"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := repository.NewBoardRepository(db).Create(ctx, repository.CreateBoardInput{
		CategoryID: &category.ID,
		Slug:       "jazz",
		Name:       "ジャズ・ファンク",
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
}
