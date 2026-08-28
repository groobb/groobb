package thread_test

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
	"github.com/groobb/groobb/go/internal/handler/thread"
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

// fixture is the handler under test together with the ids of the threads the
// cases address.
//
// [Ja] fixture はテスト対象のハンドラーと、各ケースが指すスレッドの id です。
type fixture struct {
	handler *thread.Handler

	// open is the thread the page is read on: three posts, one of them by an
	// account that has withdrawn, and replies pointing back at the earlier ones.
	//
	// [Ja] open はページを読むためのスレッドです。3 つの投稿を持ち、そのうち 1 つは退会した
	// アカウントによるもので、先行する投稿を指して戻る返信があります。
	open model.ThreadID

	// full is the thread that has reached the number of posts it can hold, which
	// is the state the page has a notice for.
	//
	// [Ja] full は持てる投稿数に達したスレッドで、ページがその旨の文言を持つ状態です。
	full model.ThreadID
}

// newFixture builds the thread Handler over a database holding one community
// whose "music" category lists a "jazz" board with two threads: one that is read
// as an ordinary thread, and one whose posts_count has reached the cap.
//
// [Ja] newFixture は、1 つのコミュニティを持つデータベース上に thread Handler を構築
// します。その "music" カテゴリーは "jazz" 掲示板を並べ、その中に 2 つのスレッドが
// 立っています。1 つは普通のスレッドとして読むもので、もう 1 つは posts_count が上限に
// 達したものです。
func newFixture(t *testing.T) fixture {
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
	postReferenceRepo := repository.NewPostReferenceRepository(db)

	music, err := categoryRepo.Create(ctx, repository.CreateCategoryInput{Slug: "music", Name: "音楽"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	jazz, err := boardRepo.Create(ctx, repository.CreateBoardInput{CategoryID: &music.ID, Slug: "jazz", Name: "ジャズ・ファンク"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	author := testutil.NewUserBuilder(t, db).WithAtname("alice").WithEmail("alice@example.com").Build()
	withdrawn := testutil.NewUserBuilder(t, db).
		WithAtname("bob").
		WithEmail("bob@example.com").
		WithDeletedAt(time.Now().Add(-24 * time.Hour)).
		Build()

	open, err := threadRepo.Create(ctx, repository.CreateThreadInput{BoardID: jazz.ID, UserID: &author, Title: "枯葉の名演"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	first, err := postRepo.Create(ctx, repository.CreatePostInput{
		ThreadID: open.ID,
		UserID:   &author,
		Number:   1,
		Body:     "好きな演奏は? https://example.com/kareha を貼っておく",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	second, err := postRepo.Create(ctx, repository.CreatePostInput{
		ThreadID: open.ID,
		UserID:   &withdrawn,
		Number:   2,
		Body:     ">>1 Bill Evans が好き",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	for _, reference := range []repository.CreatePostReferenceInput{
		{PostID: second.ID, ReferencedPostID: first.ID},
	} {
		if _, err := postReferenceRepo.Create(ctx, reference); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
	}
	if err := threadRepo.UpdateLastPost(ctx, open.ID, repository.UpdateThreadLastPostInput{
		PostsCount:   2,
		LastPostID:   second.ID,
		LastPostedAt: time.Now().Add(-30 * time.Minute),
	}); err != nil {
		t.Fatalf("UpdateLastPost() error = %v", err)
	}

	full, err := threadRepo.Create(ctx, repository.CreateThreadInput{BoardID: jazz.ID, Title: "埋まったスレッド"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	fullPost, err := postRepo.Create(ctx, repository.CreatePostInput{ThreadID: full.ID, Number: 1, Body: "最初の投稿"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := threadRepo.UpdateLastPost(ctx, full.ID, repository.UpdateThreadLastPostInput{
		PostsCount:   model.ThreadPostLimit,
		LastPostID:   fullPost.ID,
		LastPostedAt: time.Now().Add(-48 * time.Hour),
	}); err != nil {
		t.Fatalf("UpdateLastPost() error = %v", err)
	}

	return fixture{handler: newHandlerForDB(db), open: open.ID, full: full.ID}
}

// newHandlerForDB builds the thread Handler over the supplied application
// database.
//
// [Ja] newHandlerForDB は、渡されたアプリケーションデータベース上に thread Handler を
// 構築します。
func newHandlerForDB(db *database.DB) *thread.Handler {
	return newHandlerForDatabases(db, db, db)
}

// newHandlerForDatabases builds the thread Handler with a separate database
// behind each of its three reads: the thread, the community navigation, and the
// board's thread listing. Production passes the same database for all three;
// tests can break one read path without preventing the others from reaching the
// branch under test.
//
// [Ja] newHandlerForDatabases は、3 つの読み取り (スレッド・コミュニティのナビゲーション・
// 掲示板のスレッド一覧) それぞれの背後に別々のデータベースを置いて thread Handler を
// 構築します。本番は 3 つとも同じデータベースを渡しますが、テストでは 1 つの読み取りだけを
// 壊し、残りが対象分岐へ到達できます。
func newHandlerForDatabases(threadDB, navigationDB, listingDB *database.DB) *thread.Handler {
	getCommunityNavigationUC := usecase.NewGetCommunityNavigationUsecase(
		repository.NewCommunityRepository(navigationDB),
		repository.NewBoardRepository(navigationDB),
	)
	getThreadUC := usecase.NewGetThreadUsecase(
		repository.NewThreadRepository(threadDB),
		repository.NewBoardRepository(threadDB),
		repository.NewCategoryRepository(threadDB),
		repository.NewPostRepository(threadDB),
		repository.NewPostReferenceRepository(threadDB),
		repository.NewUserRepository(threadDB),
	)
	getBoardThreadsUC := usecase.NewGetBoardThreadsUsecase(repository.NewThreadRepository(listingDB))

	cfg := &config.Config{Env: "dev", AppURL: appURL}
	return thread.NewHandler(cfg, httperror.NewRenderer(cfg), getCommunityNavigationUC, getThreadUC, getBoardThreadsUC)
}

// newRequest builds a GET /t/{id} request as the router would hand it to the
// handler: the id in chi's route context, and the locale, the current path and
// the viewer in the request context, placed there directly the way i18n's,
// templates' and the auth middleware would. A nil user is an anonymous visitor.
//
// The id is passed as written rather than as a number, so a case can address the
// thread by a spelling other than the canonical one.
//
// [Ja] newRequest は、ルーターがハンドラーへ渡すのと同じ形で GET /t/{id} のリクエストを
// 組み立てます。id は chi のルート context に、ロケール・現在のパス・閲覧者はリクエスト
// context に、i18n・templates・認証の各ミドルウェアがするのと同じように直接置きます。
// user が nil のときは匿名の訪問者です。
//
// id は数ではなく書かれたままの形で渡すため、ケースは正規でない綴りでスレッドを指せます。
func newRequest(t *testing.T, id, locale string, user *model.User) *http.Request {
	t.Helper()

	path := "/t/" + id
	req := httptest.NewRequest(http.MethodGet, path, nil)

	ctx := i18n.SetLocale(req.Context(), locale)
	ctx = templates.SetCurrentPath(ctx, path)
	ctx = viewmodel.SetSiteName(ctx, communityName)
	if user != nil {
		ctx = middleware.SetUserToContext(ctx, user)
	}

	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", id)
	ctx = context.WithValue(ctx, chi.RouteCtxKey, routeCtx)

	return req.WithContext(ctx)
}

// TestShow_BreadcrumbWithoutACategory verifies that a thread in a board sitting
// in no category still says where it is, with a trail that starts at the board
// instead of at a category (ADR 0011). Unlike a board's own page the trail is
// not dropped: the board above the thread is still a place to name and to lead
// back to.
//
// [Ja] TestShow_BreadcrumbWithoutACategory は、どのカテゴリーにも属さない掲示板の
// スレッドも自身の在り処を述べること、そしてその経路がカテゴリーではなく掲示板から
// 始まることを検証します (ADR 0011)。掲示板自身のページと違って経路を落とさないのは、
// スレッドの上位にある掲示板が、名指してそこへ戻す価値のある場所であるためです。
func TestShow_BreadcrumbWithoutACategory(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testutil.SetupDB(t)

	board, err := repository.NewBoardRepository(db).Create(ctx, repository.CreateBoardInput{
		Slug: "jazz",
		Name: "ジャズ・ファンク",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	threadRepo := repository.NewThreadRepository(db)
	created, err := threadRepo.Create(ctx, repository.CreateThreadInput{BoardID: board.ID, Title: "枯葉の名演"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	post, err := repository.NewPostRepository(db).Create(ctx, repository.CreatePostInput{
		ThreadID: created.ID,
		Number:   1,
		Body:     "好きな演奏は?",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := threadRepo.UpdateLastPost(ctx, created.ID, repository.UpdateThreadLastPostInput{
		PostsCount:   1,
		LastPostID:   post.ID,
		LastPostedAt: time.Now().Add(-30 * time.Minute),
	}); err != nil {
		t.Fatalf("UpdateLastPost() error = %v", err)
	}

	rec := httptest.NewRecorder()
	newHandlerForDB(db).Show(rec, newRequest(t, created.ID.String(), i18n.LangJa, &model.User{Atname: "alice"}))

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	trail := body[strings.Index(body, `aria-label="パンくず"`):]
	trail = trail[:strings.Index(trail, "</nav>")]
	if strings.Contains(trail, "/c/") {
		t.Errorf("パンくずにカテゴリーへのリンクが含まれている: %s", trail)
	}
	if !strings.Contains(trail, `href="/b/jazz"`) {
		t.Errorf("パンくずが掲示板から始まっていない: %s", trail)
	}
	if got, want := strings.Count(trail, `<li aria-hidden="true">`), 1; got != want {
		t.Errorf("パンくずの非表示区切り数 = %d, want %d (掲示板とスレッドの 2 段)", got, want)
	}
	testutil.AssertBreadcrumbList(t, body,
		[]string{"ジャズ・ファンク", "枯葉の名演"},
		[]string{appURL + "/b/jazz", ""},
	)
}

// TestShow verifies that GET /t/{id} returns HTTP 200 with an HTML body that
// renders, for each supported locale, the thread inside the community shell: its
// title as the <h1> naming the <main> landmark, its localized meta description,
// the breadcrumb naming the category and the board, each post labelled by its
// numbered self-link and author and carrying its time and body, the board's
// threads in the complementary column, and the sidebar. The page carries no
// noindex, since a community's conversations are public.
//
// [Ja] TestShow は GET /t/{id} が HTTP 200 と、サポートする各ロケールについてコミュニティの
// シェルの中にスレッドを描画した HTML ボディを返すことを検証します。<main> ランドマークを
// 名付ける <h1> としてのタイトル、ローカライズされた meta description、カテゴリーと
// 掲示板を名指すパンくず、番号付きの自己リンクと投稿者によってラベル付けされ、時刻と本文を
// 伴う各投稿、補助カラムに並ぶ掲示板のスレッド、そしてサイドバーです。コミュニティの会話は
// 公開であるため、このページは noindex を持ちません。
func TestShow(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t)

	tests := []struct {
		name              string
		locale            string
		wantAuthor        string
		wantDescription   string
		wantPostLinkLabel string
		wantWithdrawn     string
		wantRepliesLabel  string
		wantRegionLabel   string
		wantPostsCount    string
	}{
		{
			name:              "Japanese",
			locale:            i18n.LangJa,
			wantAuthor:        "@alice",
			wantDescription:   "ジャズ・ファンク に立っているスレッド「枯葉の名演」の投稿の一覧です。",
			wantPostLinkLabel: "レス 1",
			wantWithdrawn:     "退会した利用者",
			wantRepliesLabel:  "返信",
			wantRegionLabel:   "この板のスレッド",
			wantPostsCount:    "2 件の投稿",
		},
		{
			name:              "English",
			locale:            i18n.LangEn,
			wantAuthor:        "@alice",
			wantDescription:   "The posts in the 枯葉の名演 thread, in the ジャズ・ファンク board.",
			wantPostLinkLabel: "Post 1",
			wantWithdrawn:     "A withdrawn member",
			wantRepliesLabel:  "Replies",
			wantRegionLabel:   "Threads in this board",
			wantPostsCount:    "2 posts",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			fixture.handler.Show(rec, newRequest(t, fixture.open.String(), tt.locale, &model.User{Atname: "alice"}))

			if rec.Code != http.StatusOK {
				t.Errorf("status code = %d, want %d", rec.Code, http.StatusOK)
			}
			if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
				t.Errorf("Content-Type = %q, want prefix %q", got, "text/html")
			}

			body := rec.Body.String()
			wants := []string{
				"<title>枯葉の名演 - " + communityName + "</title>",
				tt.wantAuthor,
				tt.wantWithdrawn,
				tt.wantRepliesLabel,
				tt.wantPostsCount,
				`id="p1"`,
				`id="p2"`,
				`href="#p1"`,
				`href="https://example.com/kareha"`,
				`href="/b/jazz"`,
				`href="/c/music"`,
				"ジャズ喫茶",
				`href="/settings"`,
				`lang="` + tt.locale + `"`,
			}
			for _, want := range wants {
				if !strings.Contains(body, want) {
					t.Errorf("response body does not contain %q", want)
				}
			}

			description := testutil.OpeningTag(t, body, `name="description"`)
			if !strings.Contains(description, `content="`+tt.wantDescription+`"`) {
				t.Errorf("meta description = %s, want content %q", description, tt.wantDescription)
			}

			postTag := testutil.OpeningTag(t, body, `id="p1"`)
			for _, want := range []string{
				"[contain-intrinsic-size:auto_6.5625rem]",
				"[content-visibility:auto]",
				"print:[content-visibility:visible]",
			} {
				if !strings.Contains(postTag, want) {
					t.Errorf("投稿の要素に %q が無い: %s", want, postTag)
				}
			}
			postWithoutRepliesTag := testutil.OpeningTag(t, body, `id="p2"`)
			if !strings.Contains(postWithoutRepliesTag, "[contain-intrinsic-size:auto_4.5625rem]") {
				t.Errorf("返信フッターの無い投稿に実測推定高が無い: %s", postWithoutRepliesTag)
			}
			post := testutil.Element(t, body, `id="p1"`, "</li>")
			article := testutil.OpeningTag(t, post, `aria-labelledby="p1-label"`)
			if !strings.HasPrefix(article, "<article ") {
				t.Errorf("投稿を表す要素 = %s, want article labelled by the post label", article)
			}
			label := testutil.Element(t, post, `id="p1-label"`, "</span>")
			if !strings.Contains(label, ">1</a>") || !strings.Contains(label, tt.wantAuthor) {
				t.Errorf("投稿のラベル = %s, want reply number 1 and author %q", label, tt.wantAuthor)
			}
			selfLink := testutil.OpeningTag(t, post, `href="#p1"`)
			if !strings.Contains(selfLink, `aria-label="`+tt.wantPostLinkLabel+`"`) {
				t.Errorf("投稿自身へのリンク = %s, want accessible name %q", selfLink, tt.wantPostLinkLabel)
			}

			if strings.Contains(body, "noindex") {
				t.Error("公開ページのレスポンスに noindex が含まれている")
			}

			// The thread is what the page is opened to read, so it is the <main>
			// landmark and the board's listing beside it is complementary. On a
			// narrow viewport that is what decides which column is kept.
			//
			// [Ja] このページを開いて読むものがスレッドであるため、スレッドが <main>
			// ランドマークであり、その傍らの掲示板の一覧が補助となる。狭いビューポートで
			// どちらのカラムを残すかもこれで決まる。
			main := testutil.OpeningTag(t, body, `id="main"`)
			if !strings.HasPrefix(main, "<main ") || !strings.Contains(main, `aria-labelledby="thread-show-heading"`) {
				t.Errorf("main landmark = %s, want the page heading as its accessible name", main)
			}
			heading := testutil.OpeningTag(t, body, `id="thread-show-heading"`)
			if !strings.HasPrefix(heading, "<h1 ") {
				t.Errorf("main landmark を名付ける要素 = %s, want h1", heading)
			}
			aside := testutil.OpeningTag(t, body, `aria-label="`+tt.wantRegionLabel+`"`)
			if !strings.HasPrefix(aside, "<aside ") {
				t.Errorf("掲示板のスレッド一覧の要素 = %s, want aside", aside)
			}

			// The thread being read is the current one in the listing beside it,
			// which is how the listing says where in the board the visitor is.
			//
			// [Ja] 読んでいるスレッドは、傍らの一覧における現在地である。一覧が、訪問者が
			// 掲示板のどこにいるかを述べる手立てがこれである。
			current := testutil.OpeningTag(t, body, `aria-current="page"`)
			if !strings.Contains(current, `href="`+templates.ThreadPath(viewmodel.ThreadID(fixture.open)).String()+`"`) {
				t.Errorf("現在地の印を持つ要素 = %s, want the link to the thread being read", current)
			}
		})
	}
}

// TestShow_PostReferences verifies both halves of a >>N: the body of the post
// that wrote it links forward to the post it names, and the post it names lists
// that reply number back. Without the second half a conversation can only be
// followed in the direction it was written.
//
// [Ja] TestShow_PostReferences は >>N の両側を検証します。それを書いた投稿の本文が、
// 名指した投稿へ前向きにリンクすることと、名指された投稿がそのレス番号を逆向きに並べる
// ことです。後者が無いと、会話は書かれた向きにしか辿れません。
func TestShow_PostReferences(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t)
	rec := httptest.NewRecorder()
	fixture.handler.Show(rec, newRequest(t, fixture.open.String(), i18n.LangJa, nil))

	body := rec.Body.String()

	post := testutil.Element(t, body, `id="p2"`, "</li>")
	if !strings.Contains(post, `>&gt;&gt;1</a>`) {
		t.Error("2 つ目の投稿の本文の >>1 がリンクになっていない")
	}

	referenced := testutil.Element(t, body, `id="p1"`, "</li>")
	if !strings.Contains(referenced, "返信") || !strings.Contains(referenced, `href="#p2"`) {
		t.Error("1 つ目の投稿に、それに答えた投稿への逆参照が無い")
	}
}

// TestShow_FullThread verifies that a thread that has reached the number of
// posts it can hold says so, and that a thread that has not stays free of the
// notice. The cap is what keeps a reply number a permanent address (ADR 0009),
// so the page has to say when it has been reached.
//
// [Ja] TestShow_FullThread は、持てる投稿数に達したスレッドがその旨を述べること、そして
// 達していないスレッドにはその文言が現れないことを検証します。レス番号を永久アドレスに
// 保っているのがこの上限である (ADR 0009) ため、それに達したことをページが述べる必要が
// あります。
func TestShow_FullThread(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t)

	rec := httptest.NewRecorder()
	fixture.handler.Show(rec, newRequest(t, fixture.full.String(), i18n.LangJa, nil))
	if rec.Code != http.StatusOK {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), "このスレッドは投稿数の上限 (1000 件) に達しました。") {
		t.Error("上限に達したスレッドのレスポンスに、書き込めない旨の文言が含まれていない")
	}

	open := httptest.NewRecorder()
	fixture.handler.Show(open, newRequest(t, fixture.open.String(), i18n.LangJa, nil))
	if strings.Contains(open.Body.String(), "これ以上は書き込めません。") {
		t.Error("上限に達していないスレッドのレスポンスに、書き込めない旨の文言が含まれている")
	}
}

// TestShow_AnonymousVisitor verifies that a signed-out visitor is served the
// thread, that the sidebar's account controls are left out, and that the way
// into an account stands in their place. The community's pages are readable
// without an account, so the page must not depend on there being one — and a
// thread is where a visitor most often decides to join, so the sign-in link
// carries the thread back to them once they have.
//
// [Ja] TestShow_AnonymousVisitor は、サインアウト状態の訪問者にもスレッドが届くこと、
// サイドバーのアカウント操作が描画されないこと、そしてその位置にアカウントを持つための
// 導線が立つことを検証します。コミュニティのページはアカウント無しで読めるため、ページが
// アカウントの存在に依存してはなりません。そしてスレッドは訪問者が参加を決めることの最も
// 多い場所であるため、サインインのリンクは参加した訪問者をこのスレッドへ連れ戻します。
func TestShow_AnonymousVisitor(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t)
	rec := httptest.NewRecorder()
	fixture.handler.Show(rec, newRequest(t, fixture.open.String(), i18n.LangJa, nil))

	if rec.Code != http.StatusOK {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "枯葉の名演") {
		t.Error("匿名の訪問者のレスポンスにスレッドのタイトルが含まれていない")
	}
	for _, unwanted := range []string{`href="/settings"`, `action="/user_session"`, `name="csrf_token"`} {
		if strings.Contains(body, unwanted) {
			t.Errorf("匿名の訪問者のレスポンスに %q が含まれている", unwanted)
		}
	}
	threadPath := templates.ThreadPath(viewmodel.ThreadID(fixture.open)).String()
	signInHref := templates.SignInPath().WithReturnTo(threadPath).String()
	for _, want := range []string{`href="` + signInHref + `"`, `href="` + templates.SignUpPath().String() + `"`} {
		if !strings.Contains(body, want) {
			t.Errorf("匿名の訪問者のレスポンスに %q が含まれていない", want)
		}
	}
}

// TestShow_DeclaresItsCanonicalURL verifies that the page declares its own
// address as the one it is to be known by, and that the address is the thread's
// canonical one: the same id written with a leading zero redirects here, so the
// two spellings must not be counted as two pages. It also verifies the page
// publishes its category-to-thread trail with absolute addresses, which is
// where the configured application URL reaches the breadcrumb component.
//
// [Ja] TestShow_DeclaresItsCanonicalURL は、ページが自身を知られるべきアドレスとして
// 自身のアドレスを宣言すること、そしてそのアドレスがスレッドの正規のものであることを
// 検証します。同じ id を先頭のゼロ付きで書いたパスはここへリダイレクトされるため、
// 2 つの綴りが 2 つのページとして数えられてはなりません。また、設定されたアプリケーション
// URL がパンくずコンポーネントへ届くこの経路で、カテゴリーからスレッドまでの経路を絶対
// アドレス付きで公開することも検証します。
func TestShow_DeclaresItsCanonicalURL(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	rec := httptest.NewRecorder()
	f.handler.Show(rec, newRequest(t, f.open.String(), i18n.LangJa, nil))

	canonical := testutil.OpeningTag(t, rec.Body.String(), `rel="canonical"`)
	if want := `href="` + appURL + "/t/" + f.open.String() + `"`; !strings.Contains(canonical, want) {
		t.Errorf("canonical のリンク = %s, want %s を含む", canonical, want)
	}
	testutil.AssertBreadcrumbList(t, rec.Body.String(),
		[]string{"音楽", "ジャズ・ファンク", "枯葉の名演"},
		[]string{appURL + "/c/music", appURL + "/b/jazz", ""},
	)
}

// TestShow_RedirectsNonCanonicalIDToCanonicalPath verifies that a path spelling
// the same id differently does not become a second HTTP 200 URL. strconv accepts
// a leading zero and a plus sign, so without this the same thread would answer
// under an unbounded number of addresses.
//
// The Cache-Control of the redirect is asserted alongside the status, because a
// permanent redirect can be held by the visitor's browser, while the CSRF cookie
// a safe request may mint must keep it out of shared caches.
//
// [Ja] TestShow_RedirectsNonCanonicalIDToCanonicalPath は、同じ id を別の綴りで表す
// パスが 2 つ目の HTTP 200 URL にならないことを検証します。strconv は先頭のゼロやプラス
// 記号を受け付けるため、これが無いと同じスレッドが際限のない数のアドレスで応答します。
//
// リダイレクトの Cache-Control をステータスと併せて検証するのは、恒久リダイレクトを
// 訪問者のブラウザには保持させながら、安全なリクエストが発行しうる CSRF Cookie を共有
// キャッシュには保存させないためです。
func TestShow_RedirectsNonCanonicalIDToCanonicalPath(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t)
	rec := httptest.NewRecorder()
	req := newRequest(t, "0"+fixture.open.String(), i18n.LangJa, nil)
	req.URL.RawQuery = "utm_source=newsletter"

	fixture.handler.Show(rec, req)

	if rec.Code != http.StatusPermanentRedirect {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusPermanentRedirect)
	}
	want := templates.ThreadPath(viewmodel.ThreadID(fixture.open)).String() + "?utm_source=newsletter"
	if got := rec.Header().Get("Location"); got != want {
		t.Errorf("Location = %q, want %q", got, want)
	}
	if got, want := rec.Header().Get("Cache-Control"), "private, max-age=3600"; got != want {
		t.Errorf("Cache-Control = %q, want %q", got, want)
	}
}

// TestShow_UnknownID verifies that a path naming no thread is answered with HTTP
// 404 and the shared not-found page, whether it is a number no thread carries or
// not a number at all. The status is asserted alongside the body because a page
// that reads as "not found" while answering 200 is a soft 404, and this route is
// reachable by crawlers following a link to a thread that has since been
// removed.
//
// [Ja] TestShow_UnknownID は、どのスレッドも指さないパスが HTTP 404 と共通の not-found
// ページで応答されることを検証します。どのスレッドも持たない数であっても、そもそも数で
// なくても同じです。ステータスをボディと併せて検証するのは、「見つからない」と読める
// ページが 200 で応答する状態がソフト 404 だからです。そしてこのルートには、削除済みの
// スレッドへのリンクを辿るクローラーが到達しえます。
func TestShow_UnknownID(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t)

	tests := []struct {
		name string
		id   string
	}{
		{name: "どのスレッドも持たない id", id: "999999"},
		{name: "数ではない id", id: "abc"},
		{name: "負の id", id: "-1"},
		{name: "空の id", id: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			fixture.handler.Show(rec, newRequest(t, tt.id, i18n.LangJa, nil))

			if rec.Code != http.StatusNotFound {
				t.Errorf("status code = %d, want %d", rec.Code, http.StatusNotFound)
			}
			if body := rec.Body.String(); !strings.Contains(body, "ページが見つかりません") {
				t.Error("未知の id のレスポンスに 404 ページの見出しが含まれていない")
			}
		})
	}
}

// TestShow_LookupFailure verifies that a failure to read the thread is returned
// as an internal server error rather than as a 404: a database that cannot be
// reached does not mean the thread is gone, and answering 404 would tell a
// crawler to drop a page that still exists.
//
// [Ja] TestShow_LookupFailure は、スレッドの読み取りの失敗が 404 ではなく Internal
// Server Error として返ることを検証します。到達できないデータベースはスレッドが無く
// なったことを意味せず、404 で応答すればまだ存在するページを落とすようクローラーに
// 伝えてしまいます。
func TestShow_LookupFailure(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t)
	req := newRequest(t, fixture.open.String(), i18n.LangJa, nil)
	ctx, cancel := context.WithCancel(req.Context())
	cancel()
	rec := httptest.NewRecorder()

	fixture.handler.Show(rec, req.WithContext(ctx))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(rec.Body.String(), "Internal Server Error") {
		t.Error("response body does not contain Internal Server Error")
	}
}

// TestShow_NavigationLookupFailure verifies the second database failure branch:
// the thread is read, then the navigation lookup fails and returns an internal
// server error. Keeping the databases separate prevents the first read from
// consuming the intended failure.
//
// [Ja] TestShow_NavigationLookupFailure は 2 つ目の DB 失敗分岐を検証します。スレッドの
// 読み取りには成功し、その後のナビゲーション取得が失敗して Internal Server Error を
// 返します。DB を分けることで、最初の読み取りが対象の失敗を先に消費しないようにします。
func TestShow_NavigationLookupFailure(t *testing.T) {
	t.Parallel()

	threadDB, id := newThreadDB(t)

	navigationDB := testutil.SetupDB(t)
	if err := navigationDB.Reader.Close(); err != nil {
		t.Fatalf("navigation Reader の Close() error = %v", err)
	}

	rec := httptest.NewRecorder()
	newHandlerForDatabases(threadDB, navigationDB, threadDB).Show(rec, newRequest(t, id.String(), i18n.LangJa, nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(rec.Body.String(), "Internal Server Error") {
		t.Error("response body does not contain Internal Server Error")
	}
}

// TestShow_BoardThreadListingFailure verifies the third database failure branch:
// the thread and the navigation are both read, then the board's thread listing
// fails and returns an internal server error. The listing is a read of its own,
// so it needs a case of its own to stay covered.
//
// [Ja] TestShow_BoardThreadListingFailure は 3 つ目の DB 失敗分岐を検証します。スレッドと
// ナビゲーションの読み取りには成功し、その後の掲示板のスレッド一覧の取得が失敗して
// Internal Server Error を返します。一覧は独立した読み取りであるため、覆い続けるには
// 独立したケースが要ります。
func TestShow_BoardThreadListingFailure(t *testing.T) {
	t.Parallel()

	threadDB, id := newThreadDB(t)

	listingDB := testutil.SetupDB(t)
	if err := listingDB.Reader.Close(); err != nil {
		t.Fatalf("listing Reader の Close() error = %v", err)
	}

	rec := httptest.NewRecorder()
	newHandlerForDatabases(threadDB, threadDB, listingDB).Show(rec, newRequest(t, id.String(), i18n.LangJa, nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(rec.Body.String(), "Internal Server Error") {
		t.Error("response body does not contain Internal Server Error")
	}
}

// newThreadDB creates a database holding one thread with one post, so a case
// about a later read reaches it, and returns the thread's id.
//
// [Ja] newThreadDB は、投稿を 1 つ持つスレッド 1 つを収めたデータベースを作り、後続の
// 読み取りについてのケースがそこへ到達できるようにして、そのスレッドの id を返します。
func newThreadDB(t *testing.T) (*database.DB, model.ThreadID) {
	t.Helper()

	ctx := context.Background()
	db := testutil.SetupDB(t)

	category, err := repository.NewCategoryRepository(db).Create(ctx, repository.CreateCategoryInput{Slug: "music", Name: "音楽"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	board, err := repository.NewBoardRepository(db).Create(ctx, repository.CreateBoardInput{
		CategoryID: &category.ID,
		Slug:       "jazz",
		Name:       "ジャズ・ファンク",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	created, err := repository.NewThreadRepository(db).Create(ctx, repository.CreateThreadInput{BoardID: board.ID, Title: "枯葉の名演"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := repository.NewPostRepository(db).Create(ctx, repository.CreatePostInput{
		ThreadID: created.ID,
		Number:   1,
		Body:     "最初の投稿",
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	return db, created.ID
}
