package thread_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"golang.org/x/net/html"

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
	"github.com/groobb/groobb/go/internal/validator"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// appURLは本テストでのインスタンスの公開ベースURLであり、ページのcanonicalの
// リンクがその下で組み立てられる値です。
const appURL = "https://groobb.example.com"

// communityNameは本テストでこのインスタンスが運営するコミュニティの名前です。
// ページはこれをリクエストのcontextから読み (本番ではミドルウェアがそこへ置きます)、
// タイトルの末尾に置きます。
const communityName = "ジャズ喫茶"

// fixtureはテスト対象のハンドラーと、各ケースが指すスレッドのidです。
type fixture struct {
	handler *thread.Handler

	// dbはハンドラーが読み取るデータベースで、ケースが投入後の行を変えられるように
	// するためのものです。管理者の印は既にそこにあるスレッドや投稿に付くものであり、
	// それらのケースが対象とするのはその状態です。
	db *database.DB

	// openはページを読むためのスレッドです。3つの投稿を持ち、そのうち1つは退会した
	// アカウントによるもので、先行する投稿を指して戻る返信があります。
	open model.ThreadID

	// englishは言語宣言のテストで開く英語のスレッドです。
	english model.ThreadID

	// otherはアプリがロケールを持たない言語で書かれたスレッドで、どの言語も宣言
	// しないページのテストで開きます。
	other model.ThreadID

	// fullは持てる投稿数に達したスレッドで、ページがその旨の文言を持つ状態です。
	full model.ThreadID
}

// newFixtureは、1つのコミュニティを持つデータベース上にthread Handlerを構築
// します。その "music" カテゴリーは "jazz" 掲示板を並べ、その中に4つのスレッドが
// 立っています。1つは普通のスレッドとして読むもの、1つは英語で書かれたもの、1つは
// アプリがロケールを持たない言語で書かれたもの、もう1つはposts_countが上限に達した
// ものです。
//
// ページ自身の言語でない2つがあるのは、掲示板を言語で分けないためです。読んでいる
// スレッドの傍らの一覧は複数の言語のスレッドを持ちますが、ページ自身の言語のスレッドしか
// 持たない一覧では、行が自身の言語を述べているかどうかを確かめられません。どの表示言語にも
// 解決しない1本は読んでいるスレッドとしても開き、見出しと経路が何も宣言しないことを
// そこで確かめます。
func newFixture(t *testing.T) fixture {
	t.Helper()

	ctx := context.Background()
	db := testutil.SetupDB(t)

	if _, err := db.Writer.ExecContext(ctx, "INSERT INTO communities (id, name) VALUES (1, ?)", communityName); err != nil {
		t.Fatalf("communitiesへのINSERTに失敗: %v", err)
	}

	categoryRepo := repository.NewCategoryRepository(db)
	boardRepo := repository.NewBoardRepository(db)
	threadRepo := repository.NewThreadRepository(db)
	postRepo := repository.NewPostRepository(db)
	postReferenceRepo := repository.NewPostReferenceRepository(db)

	music, err := categoryRepo.Create(ctx, repository.CreateCategoryInput{Slug: "music", Name: "音楽"})
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}
	jazz, err := boardRepo.Create(ctx, repository.CreateBoardInput{CategoryID: &music.ID, Slug: "jazz", Name: "ジャズ・ファンク"})
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}

	author := testutil.NewUserBuilder(t, db).WithAtname("alice").WithEmail("alice@example.com").Build()
	withdrawn := testutil.NewUserBuilder(t, db).
		WithAtname("bob").
		WithEmail("bob@example.com").
		WithDeletedAt(time.Now().Add(-24 * time.Hour)).
		Build()

	open, err := threadRepo.Create(ctx, repository.CreateThreadInput{BoardID: jazz.ID, UserID: &author, Title: "枯葉の名演", Language: model.LocaleJa.ThreadLanguage()})
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}
	first, err := postRepo.Create(ctx, repository.CreatePostInput{
		ThreadID: open.ID,
		UserID:   &author,
		Number:   1,
		Body:     "好きな演奏は? https://example.com/kareha を貼っておく",
	})
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}
	second, err := postRepo.Create(ctx, repository.CreatePostInput{
		ThreadID: open.ID,
		UserID:   &withdrawn,
		Number:   2,
		Body:     ">>1 Bill Evansが好き",
	})
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}
	for _, reference := range []repository.CreatePostReferenceInput{
		{PostID: second.ID, ReferencedPostID: first.ID},
	} {
		if _, err := postReferenceRepo.Create(ctx, reference); err != nil {
			t.Fatalf("Create()のエラー = %v", err)
		}
	}
	if err := threadRepo.UpdateLastPost(ctx, open.ID, repository.UpdateThreadLastPostInput{
		PostsCount:   2,
		LastPostID:   second.ID,
		LastPostedAt: time.Now().Add(-30 * time.Minute),
	}); err != nil {
		t.Fatalf("UpdateLastPost()のエラー = %v", err)
	}

	english, err := threadRepo.Create(ctx, repository.CreateThreadInput{BoardID: jazz.ID, Title: "Records I picked up", Language: model.LocaleEn.ThreadLanguage()})
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}
	englishPost, err := postRepo.Create(ctx, repository.CreatePostInput{ThreadID: english.ID, Number: 1, Body: "What have you been listening to?"})
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}
	if err := threadRepo.UpdateLastPost(ctx, english.ID, repository.UpdateThreadLastPostInput{
		PostsCount:   1,
		LastPostID:   englishPost.ID,
		LastPostedAt: time.Now().Add(-6 * time.Hour),
	}); err != nil {
		t.Fatalf("UpdateLastPost()のエラー = %v", err)
	}

	other, err := threadRepo.Create(ctx, repository.CreateThreadInput{BoardID: jazz.ID, Title: "Mes derniers disques", Language: model.ThreadLanguageOther})
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}
	otherPost, err := postRepo.Create(ctx, repository.CreatePostInput{ThreadID: other.ID, Number: 1, Body: "Bonjour, quels disques écoutez-vous ?"})
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}
	if err := threadRepo.UpdateLastPost(ctx, other.ID, repository.UpdateThreadLastPostInput{
		PostsCount:   1,
		LastPostID:   otherPost.ID,
		LastPostedAt: time.Now().Add(-12 * time.Hour),
	}); err != nil {
		t.Fatalf("UpdateLastPost()のエラー = %v", err)
	}

	full, err := threadRepo.Create(ctx, repository.CreateThreadInput{BoardID: jazz.ID, Title: "埋まったスレッド", Language: model.LocaleJa.ThreadLanguage()})
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}
	fullPost, err := postRepo.Create(ctx, repository.CreatePostInput{ThreadID: full.ID, Number: 1, Body: "最初の投稿"})
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}
	if err := threadRepo.UpdateLastPost(ctx, full.ID, repository.UpdateThreadLastPostInput{
		PostsCount:   model.ThreadPostLimit,
		LastPostID:   fullPost.ID,
		LastPostedAt: time.Now().Add(-48 * time.Hour),
	}); err != nil {
		t.Fatalf("UpdateLastPost()のエラー = %v", err)
	}

	return fixture{handler: newHandlerForDB(db), db: db, open: open.ID, english: english.ID, other: other.ID, full: full.ID}
}

// newHandlerForDBは、渡されたアプリケーションデータベース上にthread Handlerを
// 構築します。
func newHandlerForDB(db *database.DB) *thread.Handler {
	return newHandlerForDatabases(db, db, db, db)
}

// newHandlerForDatabasesは、4つの読み取り (スレッド・作成フォームが開かれる掲示板・
// コミュニティのナビゲーション・掲示板のスレッド一覧) それぞれの背後に別々のデータベースを
// 置いてthread Handlerを構築します。本番は4つとも同じデータベースを渡しますが、テストでは
// 1つの読み取りだけを壊し、残りが対象分岐へ到達できます。
//
// スレッドを立てる書き込みの先はthreadDBであり、作られたスレッドがその後読まれる
// データベースでもあります。書き込みを壊すテストは、専用のデータベースを渡すのではなく
// そのデータベースのWriterを閉じます。再描画されたフォームが、その傍らのReaderを通じて
// 掲示板へ到達できるようにするためです。
func newHandlerForDatabases(threadDB, boardDB, navigationDB, listingDB *database.DB) *thread.Handler {
	getCommunityNavigationUC := usecase.NewGetCommunityNavigationUsecase(
		repository.NewCommunityRepository(navigationDB),
		repository.NewBoardRepository(navigationDB),
		repository.NewRoleRepository(navigationDB),
	)
	getBoardUC := usecase.NewGetBoardUsecase(
		repository.NewBoardRepository(boardDB),
		repository.NewCategoryRepository(boardDB),
	)
	getThreadUC := usecase.NewGetThreadUsecase(
		repository.NewThreadRepository(threadDB),
		repository.NewBoardRepository(threadDB),
		repository.NewCategoryRepository(threadDB),
		repository.NewPostRepository(threadDB),
		repository.NewPostReferenceRepository(threadDB),
		repository.NewUserRepository(threadDB),
		repository.NewRoleRepository(threadDB),
	)
	getBoardThreadsUC := usecase.NewGetBoardThreadsUsecase(repository.NewThreadRepository(listingDB))
	createThreadUC := usecase.NewCreateThreadUsecase(
		threadDB.Writer,
		validator.NewThreadCreateValidator(),
		repository.NewBoardRepository(threadDB),
		repository.NewThreadRepository(threadDB),
		repository.NewPostRepository(threadDB),
		repository.NewUserRepository(threadDB),
	)

	cfg := &config.Config{Env: "dev", AppURL: appURL}
	return thread.NewHandler(cfg, httperror.NewRenderer(cfg), getCommunityNavigationUC, getBoardUC, getThreadUC, getBoardThreadsUC, createThreadUC)
}

// newRequestは、ルーターがハンドラーへ渡すのと同じ形でGET /t/{id} のリクエストを
// 組み立てます。idはchiのルートcontextに、ロケール・現在のパス・閲覧者はリクエスト
// contextに、i18n・templates・認証の各ミドルウェアがするのと同じように直接置きます。
// userがnilのときは匿名の訪問者です。
//
// idは数ではなく書かれたままの形で渡すため、ケースは正規でない綴りでスレッドを指せます。
func newRequest(t *testing.T, id string, locale model.Locale, user *model.User) *http.Request {
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

// TestShow_BreadcrumbWithoutACategoryは、どのカテゴリーにも属さない掲示板の
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
		t.Fatalf("Create()のエラー = %v", err)
	}
	threadRepo := repository.NewThreadRepository(db)
	created, err := threadRepo.Create(ctx, repository.CreateThreadInput{BoardID: board.ID, Title: "枯葉の名演", Language: model.LocaleJa.ThreadLanguage()})
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}
	post, err := repository.NewPostRepository(db).Create(ctx, repository.CreatePostInput{
		ThreadID: created.ID,
		Number:   1,
		Body:     "好きな演奏は?",
	})
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}
	if err := threadRepo.UpdateLastPost(ctx, created.ID, repository.UpdateThreadLastPostInput{
		PostsCount:   1,
		LastPostID:   post.ID,
		LastPostedAt: time.Now().Add(-30 * time.Minute),
	}); err != nil {
		t.Fatalf("UpdateLastPost()のエラー = %v", err)
	}

	rec := httptest.NewRecorder()
	newHandlerForDB(db).Show(rec, newRequest(t, created.ID.String(), model.LocaleJa, &model.User{Atname: "alice"}))

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
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
		t.Errorf("パンくずの非表示区切り数 = %d、期待値 = %d (掲示板とスレッドの2段)", got, want)
	}
	testutil.AssertBreadcrumbList(t, body,
		[]string{"ジャズ・ファンク", "枯葉の名演"},
		[]string{appURL + "/b/jazz", ""},
	)
}

// TestShowはGET /t/{id} がHTTP 200と、サポートする各ロケールについてコミュニティの
// シェルの中にスレッドを描画したHTMLボディを返すことを検証します。<main> ランドマークを
// 名付ける <h1> としてのタイトル、ローカライズされたmeta description、カテゴリーと
// 掲示板を名指すパンくず、番号付きの自己リンクと投稿者によってラベル付けされ、時刻と本文を
// 伴う各投稿、補助カラムに並ぶ掲示板のスレッド、そしてサイドバーです。コミュニティの会話は
// 公開であるため、このページはnoindexを持ちません。
func TestShow(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t)

	tests := []struct {
		name              string
		locale            model.Locale
		wantAuthor        string
		wantDescription   string
		wantPostLinkLabel string
		wantWithdrawn     string
		wantRepliesLabel  string
		wantRegionLabel   string
		wantPostsCount    string
	}{
		{
			name:              "日本語",
			locale:            model.LocaleJa,
			wantAuthor:        "@alice",
			wantDescription:   "ジャズ・ファンク に立っているスレッド「枯葉の名演」の投稿の一覧です。",
			wantPostLinkLabel: "レス 1",
			wantWithdrawn:     "退会した利用者",
			wantRepliesLabel:  "返信",
			wantRegionLabel:   "この板のスレッド",
			wantPostsCount:    "2 件の投稿",
		},
		{
			name:              "英語",
			locale:            model.LocaleEn,
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
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
			}
			if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
				t.Errorf("Content-Type = %q、期待値の接頭辞 = %q", got, "text/html")
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
				// ページ自身の言語は <html> 要素そのもので検証する。見出しとその傍らの
				// 行が自身のlang属性を持つため、タグをページ全体から探す形では、文書が
				// 何を宣言していてもスレッドのタイトルかそのバッジで満たされてしまう。
				`<html lang="` + string(tt.locale) + `"`,
			}
			for _, want := range wants {
				if !strings.Contains(body, want) {
					t.Errorf("レスポンスボディに %q が含まれていない", want)
				}
			}

			description := testutil.OpeningTag(t, body, `name="description"`)
			if !strings.Contains(description, `content="`+tt.wantDescription+`"`) {
				t.Errorf("meta description = %s、content %q を期待", description, tt.wantDescription)
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
				t.Errorf("投稿を表す要素 = %s、投稿のラベルで名付けられたarticleを期待", article)
			}
			label := testutil.Element(t, post, `id="p1-label"`, "</span>")
			if !strings.Contains(label, ">1</a>") || !strings.Contains(label, tt.wantAuthor) {
				t.Errorf("投稿のラベル = %s、レス番号1と作者 %q を期待", label, tt.wantAuthor)
			}
			selfLink := testutil.OpeningTag(t, post, `href="#p1"`)
			if !strings.Contains(selfLink, `aria-label="`+tt.wantPostLinkLabel+`"`) {
				t.Errorf("投稿自身へのリンク = %s、アクセシブルネーム %q を期待", selfLink, tt.wantPostLinkLabel)
			}

			if strings.Contains(body, "noindex") {
				t.Error("公開ページのレスポンスにnoindexが含まれている")
			}

			// このページを開いて読むものがスレッドであるため、スレッドが <main>
			// ランドマークであり、その傍らの掲示板の一覧が補助となる。狭いビューポートで
			// どちらのカラムを残すかもこれで決まる。
			main := testutil.OpeningTag(t, body, `id="main"`)
			if !strings.HasPrefix(main, "<main ") || !strings.Contains(main, `aria-labelledby="thread-show-heading"`) {
				t.Errorf("main landmark = %s、ページの見出しをアクセシブルネームに持つことを期待", main)
			}
			heading := testutil.OpeningTag(t, body, `id="thread-show-heading"`)
			if !strings.HasPrefix(heading, "<h1 ") {
				t.Errorf("main landmarkを名付ける要素 = %s、期待値 = h1", heading)
			}
			aside := testutil.OpeningTag(t, body, `aria-label="`+tt.wantRegionLabel+`"`)
			if !strings.HasPrefix(aside, "<aside ") {
				t.Errorf("掲示板のスレッド一覧の要素 = %s、期待値 = aside", aside)
			}

			// 読んでいるスレッドは、傍らの一覧における現在地である。一覧が、訪問者が
			// 掲示板のどこにいるかを述べる手立てがこれである。
			current := testutil.OpeningTag(t, body, `aria-current="page"`)
			if !strings.Contains(current, `href="`+templates.ThreadPath(viewmodel.ThreadID(fixture.open)).String()+`"`) {
				t.Errorf("現在地の印を持つ要素 = %s、期待値は読んでいるスレッドへのリンク", current)
			}
		})
	}
}

// TestShow_ThreadLanguageは、読んでいるスレッドの言語をページが述べること
// (見出しの傍らのバッジと、その言語として宣言された見出しおよびパンくず内で繰り返される
// タイトル) と、その傍らの一覧の各行が自身のスレッドについて同じことを述べることを検証
// します。投稿は宣言しません。別の言語での返信も受け付けるため、スレッドの言語は投稿が
// 名乗れるものではないからです。
//
// どの表示言語にも解決しない言語のスレッドの行では、その逆を確かめます。バッジは訳語へ
// 退き、タイトルは空のタグやでっち上げたタグではなく、何も宣言しません。
func TestShow_ThreadLanguage(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	rec := httptest.NewRecorder()
	f.handler.Show(rec, newRequest(t, f.english.String(), model.LocaleJa, &model.User{Atname: "alice"}))

	body := rec.Body.String()

	heading := testutil.OpeningTag(t, body, `id="thread-show-heading"`)
	if !strings.Contains(heading, `lang="en"`) {
		t.Errorf("スレッドの見出し = %s、期待値はlang=\"en\"を持つ要素", heading)
	}

	// バッジは見出しの中ではなく傍らに置く。見出しのテキストがスレッドの
	// タイトルだけであり続けるようにするため。
	if strings.Contains(heading, "badge") {
		t.Errorf("スレッドの見出し = %s、期待値はバッジを見出しの外に置く形", heading)
	}
	if !strings.Contains(body, `<span lang="en">English</span>`) {
		t.Error("読んでいるスレッドにバッジが無い")
	}

	breadcrumb := testutil.Element(t, body, `class="breadcrumb`, "</nav>")
	current := testutil.OpeningTag(t, breadcrumb, `aria-current="page"`)
	if !strings.Contains(current, `lang="en"`) || !strings.Contains(breadcrumb, ">Records I picked up</span>") {
		t.Errorf("パンくずの現在地 = %s、期待値は英語として宣言されたスレッドタイトル", breadcrumb)
	}

	// 一覧の行では、タイトルだけを持つ要素が宣言を担う。行の残りはページの言語で
	// 書かれているためである。
	title := testutil.OpeningTag(t, body, ">枯葉の名演<")
	if !strings.Contains(title, `lang="ja"`) {
		t.Errorf("日本語のスレッドのタイトル = %s、期待値はlang=\"ja\"を持つ要素", title)
	}
	row := testutil.Element(t, body, ">枯葉の名演<", "</a>")
	if !strings.Contains(row, `<span lang="ja">日本語</span>`) {
		t.Errorf("日本語のスレッドの行 = %s、期待値はその言語自身の名前のバッジ", row)
	}

	// 投稿は自身の言語を持たないため、ページ上の宣言はスレッドの言語が置いた
	// ものだけである。
	article := testutil.OpeningTag(t, body, `aria-labelledby="p1-label"`)
	if strings.Contains(article, "lang=") {
		t.Errorf("投稿の要素 = %s、期待値は投稿にlangを付けない形", article)
	}

	// アプリがロケールを持たない言語で書かれたスレッドの行は、どの言語も宣言
	// しない。宣言するタグが無く、でっち上げたタグは、そのタイトルが書かれていない言語の
	// 規則でスクリーンリーダーに発音させることになるため、バッジは代わりに訳語を載せる。
	otherTitle := testutil.OpeningTag(t, body, ">Mes derniers disques<")
	if strings.Contains(otherTitle, "lang=") {
		t.Errorf("otherのスレッドのタイトル = %s、期待値はlang属性なし", otherTitle)
	}
	otherRow := testutil.Element(t, body, ">Mes derniers disques<", "</a>")
	if !strings.Contains(otherRow, "その他") {
		t.Errorf("otherのスレッドの行 = %s、期待値は「その他」の訳語のバッジ", otherRow)
	}
}

// TestShow_ThreadLanguageOtherは、どの表示言語にも解決しない言語のスレッドを開いた
// とき、タイトルが一覧の外で繰り返される2箇所 — ページ見出しと経路の現在地 — がどの
// 言語も宣言しないこと、そして見出しの傍らのバッジは訳語でスレッドの言語を述べ続けることを
// 検証します。
//
// スレッドのタイトルを、それが並ぶ行の外で繰り返すのはこのページです。空のタグや
// でっち上げたタグを持つタイトルが、それが書かれていない言語の規則で読み上げられるのは
// まずここになります。
func TestShow_ThreadLanguageOther(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	rec := httptest.NewRecorder()
	f.handler.Show(rec, newRequest(t, f.other.String(), model.LocaleJa, &model.User{Atname: "alice"}))

	body := rec.Body.String()

	heading := testutil.OpeningTag(t, body, `id="thread-show-heading"`)
	if strings.Contains(heading, "lang=") {
		t.Errorf("スレッドの見出し = %s、期待値はlang属性なし", heading)
	}

	breadcrumb := testutil.Element(t, body, `class="breadcrumb`, "</nav>")
	current := testutil.OpeningTag(t, breadcrumb, `aria-current="page"`)
	if strings.Contains(current, "lang=") {
		t.Errorf("パンくずの現在地 = %s、期待値はlang属性なし", current)
	}

	// 見出しの傍らのバッジは変わらず描かれる。宣言できる言語を持たないスレッドが、
	// 言語の抜け落ちたスレッドとして読まれないようにするため。ページ全体ではなく見出し
	// 自身の領域から取り出すのは、ページ全体では同じスレッドの一覧の行だけでも満たされて
	// しまうためである。
	headingRegion := testutil.Element(t, body, `id="thread-show-heading"`, "</div>")
	if !strings.Contains(headingRegion, "その他") {
		t.Errorf("見出しの領域 = %s、期待値は「その他」の訳語のバッジ", headingRegion)
	}
	if strings.Contains(headingRegion, "lang=") {
		t.Errorf("見出しの領域 = %s、期待値はlang属性なし", headingRegion)
	}
}

// TestShow_PostReferencesは >>Nの両側を検証します。それを書いた投稿の本文が、
// 名指した投稿へ前向きにリンクすることと、名指された投稿がそのレス番号を逆向きに並べる
// ことです。後者が無いと、会話は書かれた向きにしか辿れません。
func TestShow_PostReferences(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t)
	rec := httptest.NewRecorder()
	fixture.handler.Show(rec, newRequest(t, fixture.open.String(), model.LocaleJa, nil))

	body := rec.Body.String()

	post := testutil.Element(t, body, `id="p2"`, "</li>")
	if !strings.Contains(post, `>&gt;&gt;1</a>`) {
		t.Error("2つ目の投稿の本文の >>1がリンクになっていない")
	}

	referenced := testutil.Element(t, body, `id="p1"`, "</li>")
	if !strings.Contains(referenced, "返信") || !strings.Contains(referenced, `href="#p2"`) {
		t.Error("1つ目の投稿に、それに答えた投稿への逆参照が無い")
	}
}

// TestShow_LockedThreadは、持てる投稿数に達したスレッドがその旨を述べること、
// そして達していないスレッドにはその文言が現れないことを検証します。レス番号を永久
// アドレスに保っているのがこの上限である (ADR 0009) ため、それに達したことをページが
// 述べる必要があります。
func TestShow_LockedThread(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t)

	rec := httptest.NewRecorder()
	fixture.handler.Show(rec, newRequest(t, fixture.full.String(), model.LocaleJa, nil))
	if rec.Code != http.StatusOK {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), "このスレッドは投稿数の上限 (1000 件) に達しました。") {
		t.Error("上限に達したスレッドのレスポンスに、書き込めない旨の文言が含まれていない")
	}

	open := httptest.NewRecorder()
	fixture.handler.Show(open, newRequest(t, fixture.open.String(), model.LocaleJa, nil))
	if strings.Contains(open.Body.String(), "これ以上は書き込めません。") {
		t.Error("上限に達していないスレッドのレスポンスに、書き込めない旨の文言が含まれている")
	}
}

// TestShow_LockedThreadOffersTheNextThreadは、持てる投稿をすべて持っているスレッドが
// 訪問者に次の道を手渡すことを検証します。上限についての案内には、同じ掲示板でスレッドを
// 立てるページへのリンクが続きます。会話が続くのはそこだからです。まだ投稿を受け付ける
// スレッドは、その末尾でそうしたものを差し出しません。会話を運ぶべき別の場所が無いため
// です。
func TestShow_LockedThreadOffersTheNextThread(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t)

	rec := httptest.NewRecorder()
	fixture.handler.Show(rec, newRequest(t, fixture.full.String(), model.LocaleJa, nil))
	if !strings.Contains(rec.Body.String(), "この掲示板で新しいスレッドを立てる") {
		t.Error("上限に達したスレッドの案内に、次のスレッドを立てる導線が含まれていない")
	}

	open := httptest.NewRecorder()
	fixture.handler.Show(open, newRequest(t, fixture.open.String(), model.LocaleJa, nil))
	if strings.Contains(open.Body.String(), "この掲示板で新しいスレッドを立てる") {
		t.Error("上限に達していないスレッドに、ロックの案内の導線が含まれている")
	}
}

// TestShow_BoardListingOffersANewThreadは、一覧カラムがその掲示板にスレッドを立てる
// 道を持つことを検証します。1つのスレッドを読んで別に述べたいことのある訪問者が、まず
// 掲示板自身のページへ戻らずに済むようにするためです。
func TestShow_BoardListingOffersANewThread(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t)

	for _, tt := range []struct {
		name   string
		locale model.Locale
		want   string
	}{
		{name: "日本語", locale: model.LocaleJa, want: "スレッドを立てる"},
		{name: "英語", locale: model.LocaleEn, want: "Start a thread"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			fixture.handler.Show(rec, newRequest(t, fixture.open.String(), tt.locale, nil))

			body := rec.Body.String()
			if !strings.Contains(body, `href="`+templates.BoardThreadsNewPath("jazz").String()+`"`) {
				t.Error("一覧カラムに、スレッドを立てるページへのリンクが含まれていない")
			}
			if !strings.Contains(body, tt.want) {
				t.Errorf("一覧カラムのリンクの文言 %q が含まれていない", tt.want)
			}
		})
	}
}

// TestShow_LockedThreadOffersNoReplyは、ロック中のスレッドが、サインイン済みの
// 訪問者にもサインアウト状態の訪問者にも同じく理由だけで終わることを検証します。ロックは
// 全員に対して成立します。送信できないフォームも、拒否されるものを書くためのサインインへの
// 誘いも、スレッドがもう受け付けないものを約束することになります。
func TestShow_LockedThreadOffersNoReply(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t)
	postsPath := templates.ThreadPostsPath(viewmodel.ThreadID(fixture.full)).String()

	for _, visitor := range []struct {
		name string
		user *model.User
	}{
		{name: "サインイン済み", user: &model.User{Atname: "alice"}},
		{name: "サインアウト状態", user: nil},
	} {
		t.Run(visitor.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			fixture.handler.Show(rec, newRequest(t, fixture.full.String(), model.LocaleJa, visitor.user))

			body := rec.Body.String()
			for _, unwanted := range []string{
				`action="` + postsPath + `"`,
				"このスレッドに返信するにはサインインしてください。",
			} {
				if strings.Contains(body, unwanted) {
					t.Errorf("ロック中のスレッドのレスポンスに %q が含まれている", unwanted)
				}
			}
		})
	}
}

// TestShow_ReplyFormは、サインイン済みの訪問者が、開いているスレッドの末尾で、
// それに答えるためのフォームに辿り着くこと、そしてそのフォームがそのスレッド自身の投稿へ
// 送信することを検証します。返信は会話が読まれた場所で書かれるため、フォームは別のどこかへの
// リンクの背後ではなく最後の投稿の後ろに立ちます。
func TestShow_ReplyForm(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t)
	rec := httptest.NewRecorder()
	fixture.handler.Show(rec, newRequest(t, fixture.open.String(), model.LocaleJa, &model.User{Atname: "alice"}))

	body := rec.Body.String()
	for _, want := range []string{
		`id="thread-show-reply-heading"`,
		"返信する",
		`action="` + templates.ThreadPostsPath(viewmodel.ThreadID(fixture.open)).String() + `"`,
		`name="body"`,
		"10,000文字以内で入力してください",
		"続けて投稿するときは10秒の間隔が必要です。",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("サインイン済みの訪問者のレスポンスに %q が含まれていない", want)
		}
	}
}

// TestShow_ReplySignInPromptは、サインアウト状態の訪問者が、開いているスレッドの
// 末尾でアカウントへの導線を差し出され、それがこのスレッドへ連れ戻すこと、そして送信でき
// ないフォームは見せられないことを検証します。訪問者が参加を決めることの最も多い場所が
// スレッドであり、その判断は言いたいことができた時点で下されます。
func TestShow_ReplySignInPrompt(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t)
	rec := httptest.NewRecorder()
	fixture.handler.Show(rec, newRequest(t, fixture.open.String(), model.LocaleJa, nil))

	body := rec.Body.String()
	threadPath := templates.ThreadPath(viewmodel.ThreadID(fixture.open)).String()
	for _, want := range []string{
		"このスレッドに返信するにはサインインしてください。",
		`href="` + templates.SignInPath().WithReturnTo(threadPath).String() + `"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("匿名の訪問者のレスポンスに %q が含まれていない", want)
		}
	}

	postsPath := templates.ThreadPostsPath(viewmodel.ThreadID(fixture.open)).String()
	if strings.Contains(body, `action="`+postsPath+`"`) {
		t.Error("匿名の訪問者のレスポンスに返信フォームが含まれている")
	}
}

// TestShow_AnonymousVisitorは、サインアウト状態の訪問者にもスレッドが届くこと、
// サイドバーのアカウント操作が描画されないこと、そしてその位置にアカウントを持つための
// 導線が立つことを検証します。コミュニティのページはアカウント無しで読めるため、ページが
// アカウントの存在に依存してはなりません。そしてスレッドは訪問者が参加を決めることの最も
// 多い場所であるため、サインインのリンクは参加した訪問者をこのスレッドへ連れ戻します。
func TestShow_AnonymousVisitor(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t)
	rec := httptest.NewRecorder()
	fixture.handler.Show(rec, newRequest(t, fixture.open.String(), model.LocaleJa, nil))

	if rec.Code != http.StatusOK {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
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

// TestShow_DeclaresItsCanonicalURLは、ページが自身を知られるべきアドレスとして
// 自身のアドレスを宣言すること、そしてそのアドレスがスレッドの正規のものであることを
// 検証します。同じidを先頭のゼロ付きで書いたパスはここへリダイレクトされるため、
// 2つの綴りが2つのページとして数えられてはなりません。また、設定されたアプリケーション
// URLがパンくずコンポーネントへ届くこの経路で、カテゴリーからスレッドまでの経路を絶対
// アドレス付きで公開することも検証します。
func TestShow_DeclaresItsCanonicalURL(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	rec := httptest.NewRecorder()
	f.handler.Show(rec, newRequest(t, f.open.String(), model.LocaleJa, nil))

	canonical := testutil.OpeningTag(t, rec.Body.String(), `rel="canonical"`)
	if want := `href="` + appURL + "/t/" + f.open.String() + `"`; !strings.Contains(canonical, want) {
		t.Errorf("canonicalのリンク = %s、%s を含むことを期待", canonical, want)
	}
	testutil.AssertBreadcrumbList(t, rec.Body.String(),
		[]string{"音楽", "ジャズ・ファンク", "枯葉の名演"},
		[]string{appURL + "/c/music", appURL + "/b/jazz", ""},
	)
}

// TestShow_RedirectsNonCanonicalIDToCanonicalPathは、同じidを別の綴りで表す
// パスが2つ目のHTTP 200 URLにならないことを検証します。strconvは先頭のゼロやプラス
// 記号を受け付けるため、これが無いと同じスレッドが際限のない数のアドレスで応答します。
//
// リダイレクトのCache-Controlをステータスと併せて検証するのは、恒久リダイレクトを
// 訪問者のブラウザには保持させながら、安全なリクエストが発行しうるCSRF Cookieを共有
// キャッシュには保存させないためです。
func TestShow_RedirectsNonCanonicalIDToCanonicalPath(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t)
	rec := httptest.NewRecorder()
	req := newRequest(t, "0"+fixture.open.String(), model.LocaleJa, nil)
	req.URL.RawQuery = "utm_source=newsletter"

	fixture.handler.Show(rec, req)

	if rec.Code != http.StatusPermanentRedirect {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusPermanentRedirect)
	}
	want := templates.ThreadPath(viewmodel.ThreadID(fixture.open)).String() + "?utm_source=newsletter"
	if got := rec.Header().Get("Location"); got != want {
		t.Errorf("Location = %q、期待値 = %q", got, want)
	}
	if got, want := rec.Header().Get("Cache-Control"), "private, max-age=3600"; got != want {
		t.Errorf("Cache-Control = %q、期待値 = %q", got, want)
	}
}

// TestShow_UnknownIDは、どのスレッドも指さないパスがHTTP 404と共通のnot-found
// ページで応答されることを検証します。どのスレッドも持たない数であっても、そもそも数で
// なくても同じです。ステータスをボディと併せて検証するのは、「見つからない」と読める
// ページが200で応答する状態がソフト404だからです。そしてこのルートには、削除済みの
// スレッドへのリンクを辿るクローラーが到達しえます。
func TestShow_UnknownID(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t)

	tests := []struct {
		name string
		id   string
	}{
		{name: "どのスレッドも持たないid", id: "999999"},
		{name: "数ではないid", id: "abc"},
		{name: "負のid", id: "-1"},
		{name: "空のid", id: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			fixture.handler.Show(rec, newRequest(t, tt.id, model.LocaleJa, nil))

			if rec.Code != http.StatusNotFound {
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusNotFound)
			}
			if body := rec.Body.String(); !strings.Contains(body, "ページが見つかりません") {
				t.Error("未知のidのレスポンスに404ページの見出しが含まれていない")
			}
		})
	}
}

// TestShow_LookupFailureは、スレッドの読み取りの失敗が404ではなくInternal
// Server Errorとして返ることを検証します。到達できないデータベースはスレッドが無く
// なったことを意味せず、404で応答すればまだ存在するページを落とすようクローラーに
// 伝えてしまいます。
func TestShow_LookupFailure(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t)
	req := newRequest(t, fixture.open.String(), model.LocaleJa, nil)
	ctx, cancel := context.WithCancel(req.Context())
	cancel()
	rec := httptest.NewRecorder()

	fixture.handler.Show(rec, req.WithContext(ctx))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(rec.Body.String(), "Internal Server Error") {
		t.Error("レスポンスボディにInternal Server Errorが含まれていない")
	}
}

// TestShow_NavigationLookupFailureは2つ目のDB失敗分岐を検証します。スレッドの
// 読み取りには成功し、その後のナビゲーション取得が失敗してInternal Server Errorを
// 返します。DBを分けることで、最初の読み取りが対象の失敗を先に消費しないようにします。
func TestShow_NavigationLookupFailure(t *testing.T) {
	t.Parallel()

	threadDB, id := newThreadDB(t)

	navigationDB := testutil.SetupDB(t)
	if err := navigationDB.Reader.Close(); err != nil {
		t.Fatalf("navigation ReaderのClose()のエラー = %v", err)
	}

	rec := httptest.NewRecorder()
	newHandlerForDatabases(threadDB, threadDB, navigationDB, threadDB).Show(rec, newRequest(t, id.String(), model.LocaleJa, nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(rec.Body.String(), "Internal Server Error") {
		t.Error("レスポンスボディにInternal Server Errorが含まれていない")
	}
}

// TestShow_BoardThreadListingFailureは3つ目のDB失敗分岐を検証します。スレッドと
// ナビゲーションの読み取りには成功し、その後の掲示板のスレッド一覧の取得が失敗して
// Internal Server Errorを返します。一覧は独立した読み取りであるため、覆い続けるには
// 独立したケースが要ります。
func TestShow_BoardThreadListingFailure(t *testing.T) {
	t.Parallel()

	threadDB, id := newThreadDB(t)

	listingDB := testutil.SetupDB(t)
	if err := listingDB.Reader.Close(); err != nil {
		t.Fatalf("listing ReaderのClose()のエラー = %v", err)
	}

	rec := httptest.NewRecorder()
	newHandlerForDatabases(threadDB, threadDB, threadDB, listingDB).Show(rec, newRequest(t, id.String(), model.LocaleJa, nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(rec.Body.String(), "Internal Server Error") {
		t.Error("レスポンスボディにInternal Server Errorが含まれていない")
	}
}

// newThreadDBは、投稿を1つ持つスレッド1つを収めたデータベースを作り、後続の
// 読み取りについてのケースがそこへ到達できるようにして、そのスレッドのidを返します。
func newThreadDB(t *testing.T) (*database.DB, model.ThreadID) {
	t.Helper()

	ctx := context.Background()
	db := testutil.SetupDB(t)

	category, err := repository.NewCategoryRepository(db).Create(ctx, repository.CreateCategoryInput{Slug: "music", Name: "音楽"})
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}
	board, err := repository.NewBoardRepository(db).Create(ctx, repository.CreateBoardInput{
		CategoryID: &category.ID,
		Slug:       "jazz",
		Name:       "ジャズ・ファンク",
	})
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}
	created, err := repository.NewThreadRepository(db).Create(ctx, repository.CreateThreadInput{BoardID: board.ID, Title: "枯葉の名演", Language: model.LocaleJa.ThreadLanguage()})
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}
	if _, err := repository.NewPostRepository(db).Create(ctx, repository.CreatePostInput{
		ThreadID: created.ID,
		Number:   1,
		Body:     "最初の投稿",
	}); err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}

	return db, created.ID
}

// TestShow_UnpublishedThreadは、管理者が見えない場所へ移したスレッドが、スレッド
// 自身でも、何も名指さないアドレスが受け取る404でもなく、その旨を述べるページを伴う404で
// 応答することを検証します。クローラーが従うのはステータスで、まだ応答していた頃に共有された
// リンクを手にした訪問者に伝えるのがこのページです。タイトルも投稿もそこにはありません。
func TestShow_UnpublishedThread(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t)

	if err := repository.NewThreadRepository(fixture.db).Unpublish(context.Background(), fixture.open); err != nil {
		t.Fatalf("Unpublish()のエラー = %v", err)
	}

	rec := httptest.NewRecorder()
	fixture.handler.Show(rec, newRequest(t, fixture.open.String(), model.LocaleJa, nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusNotFound)
	}
	if got := rec.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Errorf("Cache-Control = %q、期待値 = %q", got, "private, no-store")
	}

	body := rec.Body.String()
	for _, want := range []string{
		"このページは管理者により非公開にされました。",
		`<meta name="robots" content="noindex"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("非公開のスレッドのレスポンスに %q が含まれていない", want)
		}
	}
	for _, unwanted := range []string{"枯葉の名演", "Bill Evans", "ページが見つかりません"} {
		if strings.Contains(body, unwanted) {
			t.Errorf("非公開のスレッドのレスポンスに %q が含まれている", unwanted)
		}
	}
}

// TestShow_UnpublishedPostは、管理者が見えない場所へ移した投稿が占位としてスレッドの
// 中に位置を保つことを検証します。番号は変わらずその投稿を名指し、非公開にされた旨の1行が
// 投稿のあった場所に立ち、占位は本文・作者・時刻・返信フッターを含みません。後続の投稿からそれを指す
// 返信はリンクのままであり、スレッドを読むための番号は、書かれたものがまだ示されているか
// どうかによらず保たれます (ADR 0009)。
func TestShow_UnpublishedPost(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	fixture := newFixture(t)

	postRepo := repository.NewPostRepository(fixture.db)
	second, err := postRepo.FindByThreadIDAndNumber(ctx, fixture.open, 2)
	if err != nil {
		t.Fatalf("FindByThreadIDAndNumber()のエラー = %v", err)
	}
	if second == nil {
		t.Fatal("レス2が見つからない")
	}
	third, err := postRepo.Create(ctx, repository.CreatePostInput{
		ThreadID: fixture.open,
		Number:   3,
		Body:     ">>2私も好きです",
	})
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}
	if _, err := repository.NewPostReferenceRepository(fixture.db).Create(ctx, repository.CreatePostReferenceInput{
		PostID:           third.ID,
		ReferencedPostID: second.ID,
	}); err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}
	if err := repository.NewThreadRepository(fixture.db).UpdateLastPost(ctx, fixture.open, repository.UpdateThreadLastPostInput{
		PostsCount:   3,
		LastPostID:   third.ID,
		LastPostedAt: third.CreatedAt,
	}); err != nil {
		t.Fatalf("UpdateLastPost()のエラー = %v", err)
	}
	if err := postRepo.Unpublish(ctx, second.ID); err != nil {
		t.Fatalf("Unpublish()のエラー = %v", err)
	}

	rec := httptest.NewRecorder()
	fixture.handler.Show(rec, newRequest(t, fixture.open.String(), model.LocaleJa, nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	for _, want := range []string{
		"管理者により非公開にされました",
		`id="p2"`,
		`href="#p2"`,
		"好きな演奏は?",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("非公開の投稿を含むスレッドのレスポンスに %q が含まれていない", want)
		}
	}
	if strings.Contains(body, "Bill Evans") {
		t.Error("非公開の投稿の本文がレスポンスに含まれている")
	}

	root, err := html.Parse(strings.NewReader(body))
	if err != nil {
		t.Fatalf("html.Parse()のエラー = %v", err)
	}
	postsByID := make(map[string]*html.Node)
	for node := range root.Descendants() {
		if node.Type != html.ElementNode || node.Data != "li" {
			continue
		}
		for _, attr := range node.Attr {
			if attr.Key == "id" {
				postsByID[attr.Val] = node
			}
		}
	}
	placeholder, publicPost := postsByID["p2"], postsByID["p3"]
	if placeholder == nil || publicPost == nil {
		t.Fatal("非公開投稿の占位または公開投稿が見つからない")
	}

	var publicBody *html.Node
	for node := range publicPost.Descendants() {
		if node.Type == html.ElementNode && node.Data == "div" && node.Parent.Data == "article" {
			publicBody = node
			break
		}
	}
	if publicBody == nil {
		t.Fatal("公開投稿の本文要素が見つからない")
	}
	foundReference := false
	for node := range publicBody.Descendants() {
		if node.Type != html.ElementNode || node.Data != "a" {
			continue
		}
		for _, attr := range node.Attr {
			if attr.Key == "href" && attr.Val == "#p2" && node.FirstChild != nil && node.FirstChild.Data == ">>2" {
				foundReference = true
			}
		}
	}
	if !foundReference {
		t.Error("公開投稿の本文内に、非公開投稿への >>2リンクが含まれていない")
	}

	for node := range placeholder.Descendants() {
		if node.Type == html.TextNode && strings.Contains(node.Data, "退会した利用者") {
			t.Error("非公開投稿の占位に作者が含まれている")
		}
		if node.Type != html.ElementNode {
			continue
		}
		if node.Data == "time" || node.Data == "footer" || (node.Data == "div" && node.Parent.Data == "article") {
			t.Errorf("非公開投稿の占位に時刻・本文・返信フッターの要素 <%s> が含まれている", node.Data)
		}
	}
}

// TestShow_BoardListingExcludesUnpublishedThreadsは、管理者が見えない場所へ移した
// スレッドが、読んでいるスレッドの傍らの一覧から外れることを検証します。一覧は訪問者が
// その掲示板のスレッドを次々と辿るためのものであり、404で応答するページへ導く行は道では
// ありません。
func TestShow_BoardListingExcludesUnpublishedThreads(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t)

	before := httptest.NewRecorder()
	fixture.handler.Show(before, newRequest(t, fixture.open.String(), model.LocaleJa, nil))
	if !strings.Contains(before.Body.String(), "Records I picked up") {
		t.Fatal("一覧カラムに、公開されているスレッドが含まれていない")
	}

	if err := repository.NewThreadRepository(fixture.db).Unpublish(context.Background(), fixture.english); err != nil {
		t.Fatalf("Unpublish()のエラー = %v", err)
	}

	rec := httptest.NewRecorder()
	fixture.handler.Show(rec, newRequest(t, fixture.open.String(), model.LocaleJa, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	if strings.Contains(rec.Body.String(), "Records I picked up") {
		t.Error("一覧カラムに、非公開のスレッドが含まれている")
	}
}

// seedModeratorは、指定されたスコープを持つロールを与えたアカウントを作り、それを
// リクエストの主となる訪問者として返します。ロールを組み込みのadminより狭いものにするのは、
// ページが描くものを決めるのが管理者であることではなくスコープであり、それを示せるのは狭い
// ロールだけであるためです。
func seedModerator(t *testing.T, db *database.DB, atname string, scopes []model.Scope) *model.User {
	t.Helper()

	roleName := model.RoleName("role_" + atname)
	testutil.NewRoleBuilder(t, db).WithName(roleName).WithScopes(scopes).Build()
	userID := testutil.NewUserBuilder(t, db).
		WithAtname(atname).
		WithEmail(atname + "@example.com").
		Build()
	testutil.NewUserRoleBuilder(t, db).WithUserID(userID).WithRoleName(roleName).Build()

	return &model.User{ID: userID, Atname: atname}
}

// TestShow_ModerationActionsは、スレッドへの操作が、それを許された訪問者にだけ描かれ、
// 他の誰にも描かれないことを検証します。匿名の訪問者も、ロールを1つも持たないサインイン済みの
// 訪問者も、操作の無いページを示され、1つのスコープを持つ訪問者にはその操作だけが示されます。
//
// 「管理者であるかどうか」の1ケースではなく操作ごとに検証するのは、それぞれが固有のスコープで
// 与えられるためです。1つを持つ人に3つとも描くページは、辿り着いた先で拒否されるリンクを
// 訪問者に差し出すことになります。
func TestShow_ModerationActions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		atname            string
		scopes            []model.Scope
		signedIn          bool
		wantLockLink      bool
		wantUnpublishLink bool
		wantPostLink      bool
	}{
		{
			name:              "community:adminはすべての操作を示される",
			atname:            "admin",
			scopes:            []model.Scope{model.ScopeCommunityAdmin},
			signedIn:          true,
			wantLockLink:      true,
			wantUnpublishLink: true,
			wantPostLink:      true,
		},
		{
			name:         "thread_lock:writeだけを持つ",
			atname:       "locker",
			scopes:       []model.Scope{model.ScopeThreadLockWrite},
			signedIn:     true,
			wantLockLink: true,
		},
		{
			name:              "thread_unpublication:writeだけを持つ",
			atname:            "threadhider",
			scopes:            []model.Scope{model.ScopeThreadUnpublicationWrite},
			signedIn:          true,
			wantUnpublishLink: true,
		},
		{
			name:         "post_unpublication:writeだけを持つ",
			atname:       "posthider",
			scopes:       []model.Scope{model.ScopePostUnpublicationWrite},
			signedIn:     true,
			wantPostLink: true,
		},
		{
			name:     "ロールを1つも持たない利用者",
			atname:   "reader",
			signedIn: true,
		},
		{
			name: "匿名の訪問者",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fixture := newFixture(t)

			var user *model.User
			switch {
			case len(tt.scopes) > 0:
				user = seedModerator(t, fixture.db, tt.atname, tt.scopes)
			case tt.signedIn:
				userID := testutil.NewUserBuilder(t, fixture.db).
					WithAtname(tt.atname).
					WithEmail(tt.atname + "@example.com").
					Build()
				user = &model.User{ID: userID, Atname: tt.atname}
			}

			rec := httptest.NewRecorder()
			fixture.handler.Show(rec, newRequest(t, fixture.open.String(), model.LocaleJa, user))

			if rec.Code != http.StatusOK {
				t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
			}

			body := rec.Body.String()
			id := viewmodel.ThreadID(fixture.open)
			for _, want := range []struct {
				drawn bool
				path  templates.Path
			}{
				{drawn: tt.wantLockLink, path: templates.ThreadLockNewPath(id)},
				{drawn: tt.wantUnpublishLink, path: templates.ThreadUnpublicationNewPath(id)},
				{drawn: tt.wantPostLink, path: templates.PostUnpublicationNewPath(id, 1)},
			} {
				href := `href="` + want.path.String() + `"`
				if got := strings.Contains(body, href); got != want.drawn {
					t.Errorf("%s の描画 = %t、期待値 = %t", href, got, want.drawn)
				}
			}

		})
	}
}

// TestShow_ModerationActionsOnLockedThreadは、管理者がロックしたスレッドが、ロックを
// 掛ける操作の代わりに外す操作を差し出すことを検証します。2つは同じ権限であるため、どちらに
// なるかを決めるのはスレッドが持つロックです。外すことは確認ページを持たないため、リンクでは
// なくここから送信するフォームになります。
//
// リクエストをハンドラー単体ではなくCSRFミドルウェア越しに答えさせるのは、フォームが運ぶ
// トークンを、送信が照合される相手と同じものにするためです。フィールドを名前だけで検証すると、
// 空のトークンを運ぶフォームでも通ります。それを埋めるものが失われたときページが取る形が
// それであり、解除はハンドラーに届く前に拒否されます。
func TestShow_ModerationActionsOnLockedThread(t *testing.T) {
	t.Parallel()

	const csrfToken = "test-csrf-token"

	tests := []struct {
		locale  model.Locale
		button  string
		confirm string
	}{
		{locale: model.LocaleJa, button: "ロックを解除する", confirm: "スレッドのロックを解除しますか？"},
		{locale: model.LocaleEn, button: "Unlock thread", confirm: "Unlock this thread?"},
	}
	for _, tt := range tests {
		t.Run(string(tt.locale), func(t *testing.T) {
			t.Parallel()

			fixture := newFixture(t)
			user := seedModerator(t, fixture.db, "locker", []model.Scope{model.ScopeThreadLockWrite})

			if err := repository.NewThreadRepository(fixture.db).Lock(context.Background(), fixture.open); err != nil {
				t.Fatalf("Lock()のエラー = %v", err)
			}

			req := newRequest(t, fixture.open.String(), tt.locale, user)
			req.AddCookie(&http.Cookie{Name: middleware.CSRFCookieName, Value: csrfToken})

			rec := httptest.NewRecorder()
			csrf := middleware.NewCSRF(&config.Config{Env: "test"})
			csrf.Middleware(http.HandlerFunc(fixture.handler.Show)).ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
			}

			body := rec.Body.String()
			id := viewmodel.ThreadID(fixture.open)
			if strings.Contains(body, `href="`+templates.ThreadLockNewPath(id).String()+`"`) {
				t.Error("ロック中のスレッドに、ロックの確認ページへのリンクが描かれている")
			}

			form := testutil.Element(t, body, `<form action="`+templates.ThreadLockPath(id).String()+`"`, "</form>")
			for _, want := range []string{
				`name="_method" value="DELETE"`,
				`name="csrf_token" value="` + csrfToken + `"`,
				tt.button,
				`data-confirm="` + tt.confirm + `"`,
				`onsubmit="if (!confirm(this.dataset.confirm)) { event.preventDefault(); return false; }"`,
			} {
				if !strings.Contains(form, want) {
					t.Errorf("ロックの解除フォームに %q が含まれていない: %s", want, form)
				}
			}
		})
	}
}

// TestShow_ModerationActionsOnFullThreadは、持てる投稿をすべて持っているスレッドにも、
// 解除ではなくロックが差し出されることを検証します。上限は書き込まれることで到達した状態で
// あって誰かの判断ではないため、そこに外すべき管理者のロックは立っていません。ここへ解除を
// 送れば成功を報告しながら、スレッドは閉じたままです。
//
// 2つのロックを区別できるスレッドはこれだけであり、ページが「ロックされているか」ではなく
// 「管理者がこのスレッドをロックしたか」を問い続けることを保つのがこのテストです。
func TestShow_ModerationActionsOnFullThread(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t)
	user := seedModerator(t, fixture.db, "locker", []model.Scope{model.ScopeThreadLockWrite})

	rec := httptest.NewRecorder()
	fixture.handler.Show(rec, newRequest(t, fixture.full.String(), model.LocaleJa, user))

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	id := viewmodel.ThreadID(fixture.full)
	if !strings.Contains(body, `href="`+templates.ThreadLockNewPath(id).String()+`"`) {
		t.Error("上限に達しただけのスレッドに、ロックの確認ページへのリンクが描かれていない")
	}
	if strings.Contains(body, `<form action="`+templates.ThreadLockPath(id).String()+`"`) {
		t.Error("上限に達しただけのスレッドに、ロックの解除フォームが描かれている")
	}
}

// TestShow_ModerationActionsSkipUnpublishedPostsは、すでに視界の外へ移された投稿が
// それを外すリンクを持たず、その傍らに立つ投稿は持ち続けることを検証します。占位は、その操作が
// すでに行われた投稿の残りであるため、そこにリンクを差し出せば、示せない対象を名指すページへ
// 導くことになります。
//
// 描かれたリンクのアクセシブルネームをアドレスと併せて検証するのは、2つのリンクを区別するもの
// がその名前だけであるためです。可視テキストはどの投稿でも同じ短い文言であり、リンクだけを
// 拾い読みする人に、そのリンクがどの投稿へ働きかけるかを述べるのはレス番号です。
func TestShow_ModerationActionsSkipUnpublishedPosts(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	fixture := newFixture(t)
	user := seedModerator(t, fixture.db, "posthider", []model.Scope{model.ScopePostUnpublicationWrite})

	postRepo := repository.NewPostRepository(fixture.db)
	second, err := postRepo.FindByThreadIDAndNumber(ctx, fixture.open, 2)
	if err != nil {
		t.Fatalf("FindByThreadIDAndNumber()のエラー = %v", err)
	}
	if second == nil {
		t.Fatal("レス2が見つからない")
	}
	if err := postRepo.Unpublish(ctx, second.ID); err != nil {
		t.Fatalf("Unpublish()のエラー = %v", err)
	}

	rec := httptest.NewRecorder()
	fixture.handler.Show(rec, newRequest(t, fixture.open.String(), model.LocaleJa, user))

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	id := viewmodel.ThreadID(fixture.open)
	if !strings.Contains(body, `href="`+templates.PostUnpublicationNewPath(id, 1).String()+`"`) {
		t.Error("公開されている投稿に、非公開の確認ページへのリンクが描かれていない")
	}
	if !strings.Contains(body, `aria-label="レス 1 を非公開にする"`) {
		t.Error("投稿の非公開のリンクに、レス番号を含むアクセシブルネームが描かれていない")
	}
	if strings.Contains(body, `href="`+templates.PostUnpublicationNewPath(id, 2).String()+`"`) {
		t.Error("非公開の投稿の占位に、非公開の確認ページへのリンクが描かれている")
	}
}
