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

// appURLは本テストでのインスタンスの公開ベースURLであり、ページのcanonicalの
// リンクがその下で組み立てられる値です。
const appURL = "https://groobb.example.com"

// communityNameは本テストでこのインスタンスが運営するコミュニティの名前です。
// ページはこれをリクエストのcontextから読み (本番ではミドルウェアがそこへ置きます)、
// タイトルの末尾に置きます。
const communityName = "ジャズ喫茶"

// newHandlerは、1つのコミュニティを持つデータベース上にboard Handlerを構築
// します。その "music" カテゴリーは2つの掲示板を並べます。"jazz" は一覧が並べるのとは
// 逆の順序で作られた4つのスレッドを持ち、"quiet" は1つも持ちません。この2つで、
// このページが描画するもの — 各スレッドが最後に投稿された時刻による並び順と、誰もまだ
// スレッドを立てていない状態 — を覆えます。
//
// 言語は3つ現れます。掲示板は言語で分けないためで、ページ自身の言語のスレッドしか
// 持たない一覧では、行が自身の言語を述べているかどうかを確かめられません。アプリが
// ロケールを持たない言語で書かれた1本は、どの言語も宣言しない行を確かめる先です。
func newHandler(t *testing.T) *board.Handler {
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

	music, err := categoryRepo.Create(ctx, repository.CreateCategoryInput{Slug: "music", Name: "音楽", Position: 1})
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}

	jazz, err := boardRepo.Create(ctx, repository.CreateBoardInput{
		CategoryID:  &music.ID,
		Slug:        "jazz",
		Name:        "ジャズ・ファンク",
		Description: "ジャズの話をする板",
		Position:    1,
	})
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}
	if _, err := boardRepo.Create(ctx, repository.CreateBoardInput{
		CategoryID: &music.ID,
		Slug:       "quiet",
		Name:       "準備中の板",
		Position:   2,
	}); err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}

	createThread := func(title string, language model.ThreadLanguage, postsCount int, lastPostedAt time.Time) {
		t.Helper()
		thread, err := threadRepo.Create(ctx, repository.CreateThreadInput{BoardID: jazz.ID, Title: title, Language: language})
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
	createThread("枯葉の名演", model.LocaleJa.ThreadLanguage(), 3, time.Now().Add(-48*time.Hour))
	createThread("Mes derniers disques", model.ThreadLanguageOther, 2, time.Now().Add(-12*time.Hour))
	createThread("Records I picked up", model.LocaleEn.ThreadLanguage(), 5, time.Now().Add(-6*time.Hour))
	createThread("最近買ったレコード", model.LocaleJa.ThreadLanguage(), 42, time.Now().Add(-30*time.Minute))

	return newHandlerForDB(db)
}

// newHandlerForDBは、渡されたアプリケーションデータベース上にboard Handlerを
// 構築します。
func newHandlerForDB(db *database.DB) *board.Handler {
	return newHandlerForDatabases(db, db, db)
}

// newHandlerForDatabasesは、3つの読み取り (掲示板の解決・コミュニティの
// ナビゲーション・スレッドの一覧) それぞれの背後に別々のデータベースを置いてboard
// Handlerを構築します。本番は3つとも同じデータベースを渡しますが、テストでは1つの
// 読み取りだけを壊し、残りが対象分岐へ到達できます。
func newHandlerForDatabases(boardDB, navigationDB, threadDB *database.DB) *board.Handler {
	getCommunityNavigationUC := usecase.NewGetCommunityNavigationUsecase(
		repository.NewCommunityRepository(navigationDB),
		repository.NewBoardRepository(navigationDB),
		repository.NewRoleRepository(navigationDB),
	)
	getBoardUC := usecase.NewGetBoardUsecase(repository.NewBoardRepository(boardDB), repository.NewCategoryRepository(boardDB))
	getBoardThreadsUC := usecase.NewGetBoardThreadsUsecase(repository.NewThreadRepository(threadDB))

	cfg := &config.Config{Env: "dev", AppURL: appURL}
	return board.NewHandler(cfg, httperror.NewRenderer(cfg), getCommunityNavigationUC, getBoardUC, getBoardThreadsUC)
}

// newRequestは、ルーターがハンドラーへ渡すのと同じ形でGET /b/{slug} の
// リクエストを組み立てます。slugはchiのルートcontextに、ロケール・現在のパス・
// 閲覧者はリクエストcontextに、i18n・templates・認証の各ミドルウェアがするのと同じ
// ように直接置きます。userがnilのときは匿名の訪問者です。
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

// TestShowはGET /b/{slug} がHTTP 200と、サポートする各ロケールについて
// コミュニティのシェルの中に掲示板ページを描画したHTMLボディを返すことを検証します。
// <main> ランドマークを名付ける <h1> としての掲示板名、それを並べるカテゴリーを名指す
// パンくず、最後に投稿された順に並ぶスレッドとその投稿数・最終投稿時刻、サイドバーと
// そのアカウント操作、そしてまだ開かれていないスレッドの代わりを務める補助カラムです。
// コミュニティの掲示板は公開であるため、このページはnoindexを持ちません。
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
		wantNewThread   string
	}{
		{
			name:            "日本語",
			locale:          model.LocaleJa,
			wantPostsCount:  "42 件の投稿",
			wantLastPosted:  "30 分前",
			wantRegionLabel: "スレッドの閲覧",
			wantPrompt:      "スレッドを選ぶと、その投稿が表示されます。",
			wantNewThread:   "スレッドを立てる",
		},
		{
			name:            "英語",
			locale:          model.LocaleEn,
			wantPostsCount:  "42 posts",
			wantLastPosted:  "30 minutes ago",
			wantRegionLabel: "Reading a thread",
			wantPrompt:      "Choose a thread to see the posts in it.",
			wantNewThread:   "Start a thread",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			handler.Show(rec, newRequest(t, "jazz", tt.locale, &model.User{Atname: "alice"}))

			if rec.Code != http.StatusOK {
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
			}
			if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
				t.Errorf("Content-Type = %q、期待値の接頭辞 = %q", got, "text/html")
			}

			body := rec.Body.String()
			wants := []string{
				"<title>ジャズ・ファンク - " + communityName + "</title>",
				`content="ジャズの話をする板"`,
				tt.wantPrompt,
				tt.wantNewThread,
				tt.wantPostsCount,
				tt.wantLastPosted,
				"最近買ったレコード",
				"枯葉の名演",
				"ジャズ喫茶",
				`href="/settings"`,
				`action="/user_session"`,
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

			// スレッドのタイトルはそれへのリンクであり、一覧が掲示板の会話へ辿り着く
			// 手立てとなる。見出しは、スレッドの行だけが生むマークアップ (h2が直ちに /t/ への
			// リンクを開く形) で取り出す。ページの最初のh2がこれであるとは限らないためで、
			// フラッシュとフォームのエラー要約もh2を描画する。同じリンクは、ページ内の他の
			// 小さなリンクと共通の最小タッチ領域も保つ。
			threadHeading := testutil.Element(t, body, `<h2><a href="/t/`, "</h2>")
			if !strings.Contains(threadHeading, "最近買ったレコード") {
				t.Errorf("スレッドの見出し = %s、期待値はスレッドへのリンクになったタイトル", threadHeading)
			}
			threadLink := testutil.OpeningTag(t, threadHeading, `href="/t/`)
			for _, want := range []string{"inline-flex", "min-h-6", "min-w-6"} {
				if !strings.Contains(threadLink, want) {
					t.Errorf("スレッドのタイトルリンクに %q が無い: %s", want, threadLink)
				}
			}

			newThreadLink := testutil.OpeningTag(t, body, `href="`+templates.BoardThreadsNewPath("jazz").String()+`"`)
			if !strings.HasPrefix(newThreadLink, "<a ") {
				t.Errorf("スレッド作成フォームへの導線 = %s、期待値はリンク", newThreadLink)
			}

			if strings.Contains(body, "noindex") {
				t.Error("公開ページのレスポンスにnoindexが含まれている")
			}

			main := testutil.OpeningTag(t, body, `id="main"`)
			if !strings.HasPrefix(main, "<main ") || !strings.Contains(main, `aria-labelledby="board-show-heading"`) {
				t.Errorf("main landmark = %s、ページの見出しをアクセシブルネームに持つことを期待", main)
			}
			heading := testutil.OpeningTag(t, body, `id="board-show-heading"`)
			if !strings.HasPrefix(heading, "<h1 ") {
				t.Errorf("main landmarkを名付ける要素 = %s、期待値 = h1", heading)
			}
			aside := testutil.OpeningTag(t, body, `aria-label="`+tt.wantRegionLabel+`"`)
			if !strings.HasPrefix(aside, "<aside ") {
				t.Errorf("スレッド領域の要素 = %s、期待値 = aside", aside)
			}

			// 正確な時点は、どれだけ前かを述べるテキストの傍らのdatetime属性に
			// 置かれ、相対表現が行う丸めを越えて残る。属性そのものを手がかりに要素を
			// 引くことで、日付を持つ別の要素ではなく <time> であることも併せて検証する。
			if timeTag := testutil.OpeningTag(t, body, "datetime="); !strings.HasPrefix(timeTag, "<time ") {
				t.Errorf("datetime属性を持つ要素 = %s、期待値 = time", timeTag)
			}

			// どちらのスレッドも一覧カラムに現れるため、その位置を比べることで、
			// 最後に投稿されたものが先に来ることを確かめられる。
			if got, want := strings.Index(body, "最近買ったレコード"), strings.Index(body, "枯葉の名演"); got > want {
				t.Error("スレッドが最終投稿の新しい順に並んでいない")
			}
		})
	}
}

// TestShow_Breadcrumbは、ページが掲示板の在り処を述べることを検証します。それを
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
		t.Errorf("パンくずの要素 = %s、期待値 = nav.breadcrumb", nav)
	}
	trail := body[strings.Index(body, `aria-label="パンくず"`):]
	trail = trail[:strings.Index(trail, "</nav>")]
	if !strings.Contains(trail, `href="/c/music"`) {
		t.Error("パンくずにカテゴリーページへのリンクが無い")
	}

	// タッチターゲットの要件が拠って立つクラスだけを検証する。リンクの見た目の
	// 変更が、応答についてのテストを落とさないようにするため。1文字の名前を持つ
	// カテゴリーでは、これが無いと指で確実に押せる最小の幅を下回る。
	link := testutil.OpeningTag(t, trail, `href="/c/music"`)
	for _, want := range []string{"inline-flex", "min-h-6", "min-w-6"} {
		if !strings.Contains(link, want) {
			t.Errorf("パンくずのリンクに %q が無い: %s", want, link)
		}
	}
	if got, want := strings.Count(trail, `<li aria-hidden="true">`), 1; got != want {
		t.Errorf("パンくずの非表示区切り数 = %d、期待値 = %d", got, want)
	}
	if !strings.Contains(trail, "data-rtl-flip") {
		t.Error("パンくずの区切りにRTL反転の印が無い")
	}
	current := testutil.OpeningTag(t, trail, `aria-current="page"`)
	if !strings.HasPrefix(current, "<span ") {
		t.Errorf("パンくずの現在地の要素 = %s、期待値 = span (リンクにしない)", current)
	}
}

// TestShow_ThreadLanguageは、各行が自身のスレッドの言語を述べることを検証します。
// その言語自身の名前を載せたバッジと、その言語として宣言されたタイトルです。掲示板は
// 複数の言語のスレッドを持つため、これが無いと一覧を見渡す訪問者は自分が読めるものを
// 見分けられず、スクリーンリーダーはどのタイトルもページ自身の言語で発音します。
//
// どの表示言語にも解決しない言語のスレッドの行では、その逆を確かめます。バッジは訳語へ
// 退き、タイトルは空のタグやでっち上げたタグではなく、何も宣言しません。
func TestShow_ThreadLanguage(t *testing.T) {
	t.Parallel()

	handler := newHandler(t)
	rec := httptest.NewRecorder()
	handler.Show(rec, newRequest(t, "jazz", model.LocaleJa, &model.User{Atname: "alice"}))

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

// TestShow_DeclaresItsCanonicalURLAndPublishesItsTrailは、ページが自身を知られる
// べきアドレスとして自身のアドレスを宣言すること、そして描いた経路を、リンクを持つ各段を
// 絶対URLで名指すBreadcrumbListの構造化データとして公開することを検証します。これに
// より検索結果は、素のURLではなく掲示板の在り処を示せます。またキャンペーンのパラメータ
// を付けて到達した同じページが、2つ目のページとして数えられません。
//
// 構造化データをコンポーネント自身のテストだけでなくここでも検証するのは、ベースURLが
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
		t.Errorf("canonicalのリンク = %s、%s を含むことを期待", canonical, want)
	}

	testutil.AssertBreadcrumbList(t, body,
		[]string{"音楽", "ジャズ・ファンク"},
		[]string{appURL + "/c/music", ""},
	)
}

// TestShow_BreadcrumbWithoutACategoryは、どのカテゴリーにも属さない掲示板の
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
		t.Fatalf("Create()のエラー = %v", err)
	}

	rec := httptest.NewRecorder()
	newHandlerForDB(db).Show(rec, newRequest(t, "jazz", model.LocaleJa, &model.User{Atname: "alice"}))

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	if strings.Contains(body, `aria-label="パンくず"`) {
		t.Error("カテゴリーを持たない掲示板のページにパンくずが描画されている")
	}
	if strings.Contains(body, "ld+json") {
		t.Error("経路を持たない掲示板のページにBreadcrumbListの構造化データが描画されている")
	}
	if !strings.Contains(body, "ジャズ・ファンク") {
		t.Error("掲示板の名前がページに含まれていない")
	}
}

// TestShow_RedirectsCaseVariantToCanonicalSlugは、NOCASE検索によって大小だけが
// 異なるパスが2つ目のHTTP 200 URLにならないことを検証します。保存済みの小文字slug
// が正規アドレスであり、ページを描画する前に恒久リダイレクトで訪問者とクローラーをそこへ
// 移します。
//
// リダイレクトのCache-Controlをステータスと併せて検証するのは、恒久リダイレクトを
// 訪問者のブラウザには保持させながら、安全なリクエストが発行しうるCSRF Cookieを
// 共有キャッシュには保存させないためです。
func TestShow_RedirectsCaseVariantToCanonicalSlug(t *testing.T) {
	t.Parallel()

	handler := newHandler(t)
	rec := httptest.NewRecorder()
	req := newRequest(t, "JAZZ", model.LocaleJa, &model.User{Atname: "alice"})
	req.URL.RawQuery = "utm_source=newsletter"

	handler.Show(rec, req)

	if rec.Code != http.StatusPermanentRedirect {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusPermanentRedirect)
	}
	if got, want := rec.Header().Get("Location"), templates.BoardPath("jazz").String()+"?utm_source=newsletter"; got != want {
		t.Errorf("Location = %q、期待値 = %q", got, want)
	}
	if got, want := rec.Header().Get("Cache-Control"), "private, max-age=3600"; got != want {
		t.Errorf("Cache-Control = %q、期待値 = %q", got, want)
	}
}

// TestShow_AnonymousVisitorは、サインアウト状態の訪問者にも掲示板とそのスレッドが
// 届くこと、サイドバーのアカウント操作が描画されないこと、そしてその位置にアカウントを
// 持つための導線が立つことを検証します。コミュニティのページはアカウント無しで読めるため、
// ページがアカウントの存在に依存してはなりません。そしてこの掲示板のページは訪問者が参加を
// 決める場所の1つであるため、サインインのリンクは参加した訪問者をこの掲示板へ連れ戻します。
func TestShow_AnonymousVisitor(t *testing.T) {
	t.Parallel()

	handler := newHandler(t)
	rec := httptest.NewRecorder()
	handler.Show(rec, newRequest(t, "jazz", model.LocaleJa, nil))

	if rec.Code != http.StatusOK {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
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
	for _, want := range []string{
		`href="` + signInHref + `"`,
		`href="` + templates.SignUpPath().String() + `"`,
		`href="` + templates.BoardThreadsNewPath("jazz").String() + `"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("匿名の訪問者のレスポンスに %q が含まれていない", want)
		}
	}
}

// TestShow_EmptyBoardは、まだ誰も書き込んでいない掲示板が、見出しの下に何も無い
// 状態ではなくその旨を伝えること、そして読むためのカラムが、何も入っていない一覧から
// 選ぶよう促すのをやめることを検証します。自身の説明を持たない掲示板も、どの掲示板でも
// 同じになるサイト全体の既定値ではなく、それを名指すmeta descriptionを得ます。
func TestShow_EmptyBoard(t *testing.T) {
	t.Parallel()

	handler := newHandler(t)
	rec := httptest.NewRecorder()
	handler.Show(rec, newRequest(t, "quiet", model.LocaleJa, &model.User{Atname: "alice"}))

	if rec.Code != http.StatusOK {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
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
		t.Error("説明を持たない掲示板のレスポンスに、掲示板を名指すmeta descriptionが含まれていない")
	}

	// 空状態はスレッドを立てる導線を持ち、ここに何も見つけなかった訪問者に、それに
	// ついてできることを差し出す。ページ上でこれが唯一の導線である。一覧の上のリンクには、
	// その上に立つべき一覧が無い。
	if !strings.Contains(body, "最初のスレッドを立てる") {
		t.Error("スレッドを持たない掲示板の空状態に、スレッドを立てる導線が含まれていない")
	}
	if got, want := strings.Count(body, templates.BoardThreadsNewPath("quiet").String()), 1; got != want {
		t.Errorf("スレッド作成フォームへの導線の数 = %d、期待値 = %d", got, want)
	}
}

// TestShow_UnknownSlugは、どの掲示板も指さないslugがHTTP 404と共通の
// not-foundページで応答されることを検証します。ステータスをボディと併せて検証するのは、
// 「見つからない」と読めるページが200で応答する状態がソフト404だからです。そして
// このルートには、削除済みの掲示板へのリンクを辿るクローラーが到達しえます。
func TestShow_UnknownSlug(t *testing.T) {
	t.Parallel()

	handler := newHandler(t)
	rec := httptest.NewRecorder()
	handler.Show(rec, newRequest(t, "no-such-board", model.LocaleJa, &model.User{Atname: "alice"}))

	if rec.Code != http.StatusNotFound {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusNotFound)
	}
	if body := rec.Body.String(); !strings.Contains(body, "ページが見つかりません") {
		t.Error("未知のslugのレスポンスに404ページの見出しが含まれていない")
	}
}

// TestShow_LookupFailureは、掲示板の読み取りの失敗が404ではなくInternal
// Server Errorとして返ることを検証します。到達できないデータベースは掲示板が無く
// なったことを意味せず、404で応答すればまだ存在するページを落とすようクローラーに
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
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(rec.Body.String(), "Internal Server Error") {
		t.Error("レスポンスボディにInternal Server Errorが含まれていない")
	}
}

// TestShow_NavigationLookupFailureは2つ目のDB失敗分岐を検証します。掲示板の
// 取得には成功し、その後のナビゲーション取得が失敗してInternal Server Errorを返します。
// DBを分けることで、最初の取得が対象の失敗を先に消費しないようにします。
func TestShow_NavigationLookupFailure(t *testing.T) {
	t.Parallel()

	boardDB := testutil.SetupDB(t)
	createJazzBoard(t, boardDB)

	navigationDB := testutil.SetupDB(t)
	if err := navigationDB.Reader.Close(); err != nil {
		t.Fatalf("navigation ReaderのClose()のエラー = %v", err)
	}

	handler := newHandlerForDatabases(boardDB, navigationDB, boardDB)
	rec := httptest.NewRecorder()
	handler.Show(rec, newRequest(t, "jazz", model.LocaleJa, &model.User{Atname: "alice"}))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(rec.Body.String(), "Internal Server Error") {
		t.Error("レスポンスボディにInternal Server Errorが含まれていない")
	}
}

// TestShow_ThreadListingFailureは3つ目のDB失敗分岐を検証します。掲示板の解決と
// ナビゲーションの読み取りには成功し、その後のスレッド一覧の取得が失敗してInternal
// Server Errorを返します。一覧を掲示板の解決から切り離したことでこの失敗が独立した
// 分岐になったため、覆い続けるには独立したケースが要ります。
func TestShow_ThreadListingFailure(t *testing.T) {
	t.Parallel()

	boardDB := testutil.SetupDB(t)
	createJazzBoard(t, boardDB)

	threadDB := testutil.SetupDB(t)
	if err := threadDB.Reader.Close(); err != nil {
		t.Fatalf("thread ReaderのClose()のエラー = %v", err)
	}

	handler := newHandlerForDatabases(boardDB, boardDB, threadDB)
	rec := httptest.NewRecorder()
	handler.Show(rec, newRequest(t, "jazz", model.LocaleJa, &model.User{Atname: "alice"}))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(rec.Body.String(), "Internal Server Error") {
		t.Error("レスポンスボディにInternal Server Errorが含まれていない")
	}
}

// createJazzBoardは、失敗のケースが解決する掲示板を、それが属さなければならない
// カテゴリーと一緒に作ります。後続の読み取りについてのケースがそこへ到達できるように
// するためです。
func createJazzBoard(t *testing.T, db *database.DB) {
	t.Helper()

	ctx := context.Background()
	category, err := repository.NewCategoryRepository(db).Create(ctx, repository.CreateCategoryInput{Slug: "music", Name: "音楽"})
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}
	if _, err := repository.NewBoardRepository(db).Create(ctx, repository.CreateBoardInput{
		CategoryID: &category.ID,
		Slug:       "jazz",
		Name:       "ジャズ・ファンク",
	}); err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}
}
