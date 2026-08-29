package category_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/handler/category"
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

// newHandler builds the category Handler over a database holding one community
// with two categories: "music", which lists two boards in the reverse of the
// order they are created in, and "empty", which lists none. Between them the two
// cover what the page renders — a listing whose order comes from the position
// the community gave each board, and the state where the community has yet to
// place a board.
//
// [Ja] newHandler は、1 つのコミュニティと 2 つのカテゴリーを持つデータベース上に
// category Handler を構築します。"music" は作成順とは逆の順序で 2 つの掲示板を並べ、
// "empty" は 1 つも並べません。この 2 つで、このページが描画するもの — コミュニティが
// 各掲示板に与えた position 由来の並び順と、コミュニティがまだ掲示板を置いていない
// 状態 — を覆えます。
func newHandler(t *testing.T) *category.Handler {
	t.Helper()

	ctx := context.Background()
	db := testutil.SetupDB(t)

	if _, err := db.Writer.ExecContext(ctx, "INSERT INTO communities (id, name) VALUES (1, ?)", communityName); err != nil {
		t.Fatalf("communities への INSERT に失敗: %v", err)
	}

	categoryRepo := repository.NewCategoryRepository(db)
	boardRepo := repository.NewBoardRepository(db)

	music, err := categoryRepo.Create(ctx, repository.CreateCategoryInput{Slug: "music", Name: "音楽", Position: 1})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := categoryRepo.Create(ctx, repository.CreateCategoryInput{Slug: "empty", Name: "準備中", Position: 2}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	createBoard := func(slug, name, description string, position int) {
		t.Helper()
		if _, err := boardRepo.Create(ctx, repository.CreateBoardInput{
			CategoryID:  &music.ID,
			Slug:        slug,
			Name:        name,
			Description: description,
			Position:    position,
		}); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
	}
	createBoard("rock", "ロック", "ロックの話をする板", 2)
	createBoard("jazz", "ジャズ・ファンク", "ジャズの話をする板", 1)

	return newHandlerForDB(db)
}

// newHandlerForDB builds the category Handler over the supplied application
// database.
//
// [Ja] newHandlerForDB は、渡されたアプリケーションデータベース上に category Handler を
// 構築します。
func newHandlerForDB(db *database.DB) *category.Handler {
	return newHandlerForDatabases(db, db, db)
}

// newHandlerForDatabases builds the category Handler with a separate database
// behind each of its three reads: resolving the category, the community
// navigation, and the board listing. Production passes the same database for all
// three; tests can break one read path without preventing the others from
// reaching the branch under test.
//
// [Ja] newHandlerForDatabases は、3 つの読み取り (カテゴリーの解決・コミュニティの
// ナビゲーション・掲示板の一覧) それぞれの背後に別々のデータベースを置いて category
// Handler を構築します。本番は 3 つとも同じデータベースを渡しますが、テストでは 1 つの
// 読み取りだけを壊し、残りが対象分岐へ到達できます。
func newHandlerForDatabases(categoryDB, navigationDB, boardDB *database.DB) *category.Handler {
	getCommunityNavigationUC := usecase.NewGetCommunityNavigationUsecase(
		repository.NewCommunityRepository(navigationDB),
		repository.NewBoardRepository(navigationDB),
	)
	getCategoryUC := usecase.NewGetCategoryUsecase(repository.NewCategoryRepository(categoryDB))
	getCategoryBoardsUC := usecase.NewGetCategoryBoardsUsecase(repository.NewBoardRepository(boardDB))

	cfg := &config.Config{Env: "dev", AppURL: appURL}
	return category.NewHandler(cfg, httperror.NewRenderer(cfg), getCommunityNavigationUC, getCategoryUC, getCategoryBoardsUC)
}

// newRequest builds a GET /c/{slug} request as the router would hand it to the
// handler: the slug in chi's route context, and the locale, the current path and
// the viewer in the request context, placed there directly the way i18n's,
// templates' and the auth middleware would. A nil user is an anonymous visitor.
//
// [Ja] newRequest は、ルーターがハンドラーへ渡すのと同じ形で GET /c/{slug} の
// リクエストを組み立てます。slug は chi のルート context に、ロケール・現在のパス・
// 閲覧者はリクエスト context に、i18n・templates・認証の各ミドルウェアがするのと同じ
// ように直接置きます。user が nil のときは匿名の訪問者です。
func newRequest(t *testing.T, slug string, locale model.Locale, user *model.User) *http.Request {
	t.Helper()

	path := templates.CategoryPath(slug).String()
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

// TestShow verifies that GET /c/{slug} returns HTTP 200 with an HTML body that
// renders, for each supported locale, the category page inside the community
// shell: the category's name as the <h1> naming the <main> landmark, the boards
// it lists in the order their positions give them with their descriptions, the
// sidebar and its account controls, and the complementary column standing in for
// a thread that has not been opened. The page carries no noindex, since a
// community's categories are public.
//
// [Ja] TestShow は GET /c/{slug} が HTTP 200 と、サポートする各ロケールについて
// コミュニティのシェルの中にカテゴリーページを描画した HTML ボディを返すことを検証
// します。<main> ランドマークを名付ける <h1> としてのカテゴリー名、position が与える
// 順序で並ぶ掲示板とその説明、サイドバーとそのアカウント操作、そしてまだ開かれていない
// スレッドの代わりを務める補助カラムです。コミュニティのカテゴリーは公開であるため、
// このページは noindex を持ちません。
func TestShow(t *testing.T) {
	t.Parallel()

	handler := newHandler(t)

	tests := []struct {
		name            string
		locale          model.Locale
		wantDescription string
		wantRegionLabel string
		wantPrompt      string
	}{
		{
			name:            "Japanese",
			locale:          model.LocaleJa,
			wantDescription: "音楽 カテゴリーの掲示板の一覧です。",
			wantRegionLabel: "スレッドの閲覧",
			wantPrompt:      "掲示板を選ぶと、その中のスレッドが表示されます。",
		},
		{
			name:            "English",
			locale:          model.LocaleEn,
			wantDescription: "The boards in the 音楽 category.",
			wantRegionLabel: "Reading a thread",
			wantPrompt:      "Choose a board to see the threads in it.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			handler.Show(rec, newRequest(t, "music", tt.locale, &model.User{Atname: "alice"}))

			if rec.Code != http.StatusOK {
				t.Errorf("status code = %d, want %d", rec.Code, http.StatusOK)
			}
			if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
				t.Errorf("Content-Type = %q, want prefix %q", got, "text/html")
			}

			body := rec.Body.String()
			wants := []string{
				"<title>音楽 - " + communityName + "</title>",
				`content="` + tt.wantDescription + `"`,
				tt.wantPrompt,
				`href="/b/jazz"`,
				"ジャズ・ファンク",
				"ジャズの話をする板",
				`href="/b/rock"`,
				"ロック",
				"ロックの話をする板",
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

			// Only the classes the touch-target rule rests on are asserted, so a
			// change to how the link looks does not fail a test about the response.
			// The search starts at the <main> landmark because the sidebar links the
			// same board earlier in the document under its own, narrower styling.
			//
			// [Ja] タッチターゲットの要件が拠って立つクラスだけを検証する。リンクの
			// 見た目の変更が、応答についてのテストを落とさないようにするため。探索を
			// <main> ランドマークから始めるのは、サイドバーが同じ掲示板を、それ自身の
			// より狭いスタイルで文書のより前にリンクしているためである。
			boardLink := testutil.OpeningTag(t, listColumn(t, body), `href="/b/jazz"`)
			for _, want := range []string{"inline-flex", "min-h-6", "min-w-6"} {
				if !strings.Contains(boardLink, want) {
					t.Errorf("板名リンクに %q が無い: %s", want, boardLink)
				}
			}

			if strings.Contains(body, "noindex") {
				t.Error("公開ページのレスポンスに noindex が含まれている")
			}

			main := testutil.OpeningTag(t, body, `id="main"`)
			if !strings.HasPrefix(main, "<main ") || !strings.Contains(main, `aria-labelledby="category-show-heading"`) {
				t.Errorf("main landmark = %s, want the page heading as its accessible name", main)
			}
			heading := testutil.OpeningTag(t, body, `id="category-show-heading"`)
			if !strings.HasPrefix(heading, "<h1 ") {
				t.Errorf("main landmark を名付ける要素 = %s, want h1", heading)
			}
			aside := testutil.OpeningTag(t, body, `aria-label="`+tt.wantRegionLabel+`"`)
			if !strings.HasPrefix(aside, "<aside ") {
				t.Errorf("スレッド領域の要素 = %s, want aside", aside)
			}
			// The descriptions appear only in the list column, so comparing their
			// positions checks the order of that listing rather than the sidebar,
			// where the same two boards are also linked.
			//
			// [Ja] 説明は一覧カラムにしか現れないため、その位置を比べることで、同じ 2 つの
			// 掲示板がリンクされているサイドバーではなく一覧の並び順を確かめられる。
			if got, want := strings.Index(body, "ジャズの話をする板"), strings.Index(body, "ロックの話をする板"); got > want {
				t.Error("掲示板が position の順に並んでいない")
			}
		})
	}
}

// TestShow_LeavesTheSidebarUnmarked verifies that the sidebar marks nothing
// while a category is being rendered. It lists the boards flat and links to no
// category (ADR 0011), so none of its destinations is this page, and marking a
// board that is not open would turn the mark into a decoration.
//
// [Ja] TestShow_LeavesTheSidebarUnmarked は、カテゴリーを描画している間サイドバーが
// 何にも印を付けないことを検証します。サイドバーは掲示板をフラットに並べ、カテゴリーへは
// リンクしないため (ADR 0011)、その行き先のどれもこのページではありません。開いていない
// 掲示板に印を付ければ、それは飾りになってしまいます。
func TestShow_LeavesTheSidebarUnmarked(t *testing.T) {
	t.Parallel()

	handler := newHandler(t)
	rec := httptest.NewRecorder()
	handler.Show(rec, newRequest(t, "music", model.LocaleJa, &model.User{Atname: "alice"}))

	body := rec.Body.String()

	if strings.Contains(body, `href="/c/music"`) {
		t.Error("サイドバーに今開いているカテゴリーへのリンクが含まれている (掲示板はフラットに並べる)")
	}
	if got := strings.Count(body, `aria-current="page"`); got != 0 {
		t.Errorf("aria-current=\"page\" の数 = %d, want 0 (サイドバーの行き先はいずれもこのページではない)", got)
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
	handler.Show(rec, newRequest(t, "MUSIC", model.LocaleJa, &model.User{Atname: "alice"}))

	if rec.Code != http.StatusPermanentRedirect {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusPermanentRedirect)
	}
	if got, want := rec.Header().Get("Location"), templates.CategoryPath("music").String(); got != want {
		t.Errorf("Location = %q, want %q", got, want)
	}
	if got, want := rec.Header().Get("Cache-Control"), "private, max-age=3600"; got != want {
		t.Errorf("Cache-Control = %q, want %q", got, want)
	}
}

// TestShow_RedirectToCanonicalSlugKeepsQuery verifies that the query string
// survives the redirect. Only the spelling of the slug makes the URL
// non-canonical, so dropping the query would take a campaign parameter — or a
// listing parameter a later task adds — away from whoever followed a differently
// cased link.
//
// [Ja] TestShow_RedirectToCanonicalSlugKeepsQuery は、クエリ文字列がリダイレクトを
// 越えて残ることを検証します。URL を非正規にしているのは slug の綴りだけであるため、
// クエリを落とすと、大文字小文字の異なるリンクを辿った人だけが計測用のパラメータや、
// 後続のタスクが一覧へ足すパラメータを失うことになります。
func TestShow_RedirectToCanonicalSlugKeepsQuery(t *testing.T) {
	t.Parallel()

	handler := newHandler(t)
	req := newRequest(t, "MUSIC", model.LocaleJa, &model.User{Atname: "alice"})
	req.URL.RawQuery = "utm_source=newsletter&page=2"
	rec := httptest.NewRecorder()

	handler.Show(rec, req)

	if rec.Code != http.StatusPermanentRedirect {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusPermanentRedirect)
	}
	want := templates.CategoryPath("music").String() + "?utm_source=newsletter&page=2"
	if got := rec.Header().Get("Location"); got != want {
		t.Errorf("Location = %q, want %q", got, want)
	}
}

// TestShow_AnonymousVisitor verifies that a signed-out visitor is served the
// category and its boards, that the sidebar's account controls are left out, and
// that the way into an account stands in their place. The community's pages are
// readable without an account, so the page must not depend on there being one —
// and this category page is one of the places a visitor decides to join from, so
// the sign-in link carries the category back to them once they have.
//
// [Ja] TestShow_AnonymousVisitor は、サインアウト状態の訪問者にもカテゴリーとその
// 掲示板が届くこと、サイドバーのアカウント操作が描画されないこと、そしてその位置に
// アカウントを持つための導線が立つことを検証します。コミュニティのページはアカウント
// 無しで読めるため、ページがアカウントの存在に依存してはなりません。そしてこのカテゴリーの
// ページは訪問者が参加を決める場所の 1 つであるため、サインインのリンクは参加した訪問者を
// このカテゴリーへ連れ戻します。
func TestShow_AnonymousVisitor(t *testing.T) {
	t.Parallel()

	handler := newHandler(t)
	rec := httptest.NewRecorder()
	handler.Show(rec, newRequest(t, "music", model.LocaleJa, nil))

	if rec.Code != http.StatusOK {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	if !strings.Contains(body, `href="/b/jazz"`) {
		t.Error("匿名の訪問者のレスポンスに掲示板のリンクが含まれていない")
	}
	for _, unwanted := range []string{`href="/settings"`, `action="/user_session"`, `name="csrf_token"`} {
		if strings.Contains(body, unwanted) {
			t.Errorf("匿名の訪問者のレスポンスに %q が含まれている", unwanted)
		}
	}
	signInHref := templates.SignInPath().WithReturnTo(templates.CategoryPath("music").String()).String()
	for _, want := range []string{`href="` + signInHref + `"`, `href="` + templates.SignUpPath().String() + `"`} {
		if !strings.Contains(body, want) {
			t.Errorf("匿名の訪問者のレスポンスに %q が含まれていない", want)
		}
	}
}

// TestShow_EmptyCategory verifies that a category the community has placed no
// board in says so, rather than rendering a heading above nothing, and that the
// reading column stops inviting the visitor to choose from a list that has
// nothing in it.
//
// [Ja] TestShow_EmptyCategory は、コミュニティがまだ掲示板を置いていないカテゴリーが、
// 見出しの下に何も無い状態ではなくその旨を伝えること、そして読むためのカラムが、何も
// 入っていない一覧から選ぶよう促すのをやめることを検証します。
func TestShow_EmptyCategory(t *testing.T) {
	t.Parallel()

	handler := newHandler(t)
	rec := httptest.NewRecorder()
	handler.Show(rec, newRequest(t, "empty", model.LocaleJa, &model.User{Atname: "alice"}))

	if rec.Code != http.StatusOK {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	if !strings.Contains(body, `<meta name="robots" content="noindex">`) {
		t.Error("掲示板を持たないカテゴリーのレスポンスに noindex が含まれていない")
	}
	if strings.Contains(body, `rel="canonical"`) {
		t.Error("インデックスを求めないカテゴリーが canonical のリンクを持っている")
	}
	if !strings.Contains(body, "このカテゴリーにはまだ掲示板がありません。") {
		t.Error("掲示板を持たないカテゴリーのレスポンスに空状態の文言が含まれていない")
	}
	if !strings.Contains(body, "掲示板が置かれると、そのスレッドがここに表示されます。") {
		t.Error("掲示板を持たないカテゴリーの読むためのカラムに、空状態の文言が含まれていない")
	}
	if strings.Contains(body, "掲示板を選ぶと、その中のスレッドが表示されます。") {
		t.Error("掲示板を持たないカテゴリーの読むためのカラムが、選べない掲示板を選ぶよう促している")
	}
}

// TestShow_DeclaresItsCanonicalURL verifies that a category listing at least one
// board declares its own address as the one it is to be known by, so that the
// same page reached with a campaign parameter appended is not counted as a
// second one.
//
// [Ja] TestShow_DeclaresItsCanonicalURL は、掲示板を 1 つ以上並べるカテゴリーが、自身を
// 知られるべきアドレスとして自身のアドレスを宣言することを検証します。キャンペーンの
// パラメータを付けて到達した同じページが、2 つ目のページとして数えられないようにする
// ためです。
func TestShow_DeclaresItsCanonicalURL(t *testing.T) {
	t.Parallel()

	handler := newHandler(t)
	rec := httptest.NewRecorder()
	handler.Show(rec, newRequest(t, "music", model.LocaleJa, nil))

	canonical := testutil.OpeningTag(t, rec.Body.String(), `rel="canonical"`)
	if want := `href="` + appURL + "/c/music" + `"`; !strings.Contains(canonical, want) {
		t.Errorf("canonical のリンク = %s, want %s を含む", canonical, want)
	}
}

// TestShow_UnknownSlug verifies that a slug naming no category is answered with
// HTTP 404 and the shared not-found page. The status is asserted alongside the
// body because a page that reads as "not found" while answering 200 is a soft
// 404, and this route is reachable by crawlers following a link to a category
// that has since been removed.
//
// [Ja] TestShow_UnknownSlug は、どのカテゴリーも指さない slug が HTTP 404 と共通の
// not-found ページで応答されることを検証します。ステータスをボディと併せて検証するのは、
// 「見つからない」と読めるページが 200 で応答する状態がソフト 404 だからです。そして
// このルートには、削除済みのカテゴリーへのリンクを辿るクローラーが到達しえます。
func TestShow_UnknownSlug(t *testing.T) {
	t.Parallel()

	handler := newHandler(t)
	rec := httptest.NewRecorder()
	handler.Show(rec, newRequest(t, "no-such-category", model.LocaleJa, &model.User{Atname: "alice"}))

	if rec.Code != http.StatusNotFound {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if body := rec.Body.String(); !strings.Contains(body, "ページが見つかりません") {
		t.Error("未知の slug のレスポンスに 404 ページの見出しが含まれていない")
	}
}

// TestShow_LookupFailure verifies that a failure to read the category is
// returned as an internal server error rather than as a 404: a database that
// cannot be reached does not mean the category is gone, and answering 404 would
// tell a crawler to drop a page that still exists.
//
// [Ja] TestShow_LookupFailure は、カテゴリーの読み取りの失敗が 404 ではなく Internal
// Server Error として返ることを検証します。到達できないデータベースはカテゴリーが
// 無くなったことを意味せず、404 で応答すればまだ存在するページを落とすようクローラーに
// 伝えてしまいます。
func TestShow_LookupFailure(t *testing.T) {
	t.Parallel()

	handler := newHandler(t)
	req := newRequest(t, "music", model.LocaleJa, &model.User{Atname: "alice"})
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
// category lookup succeeds, then navigation lookup fails and returns an internal
// server error. Keeping the databases separate prevents the first lookup from
// consuming the intended failure.
//
// [Ja] TestShow_NavigationLookupFailure は 2 つ目の DB 失敗分岐を検証します。カテゴリーの
// 取得には成功し、その後のナビゲーション取得が失敗して Internal Server Error を返します。
// DB を分けることで、最初の取得が対象の失敗を先に消費しないようにします。
func TestShow_NavigationLookupFailure(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	categoryDB := testutil.SetupDB(t)
	categoryRepo := repository.NewCategoryRepository(categoryDB)
	if _, err := categoryRepo.Create(ctx, repository.CreateCategoryInput{Slug: "music", Name: "音楽"}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	navigationDB := testutil.SetupDB(t)
	if err := navigationDB.Reader.Close(); err != nil {
		t.Fatalf("navigation Reader の Close() error = %v", err)
	}

	handler := newHandlerForDatabases(categoryDB, navigationDB, categoryDB)
	rec := httptest.NewRecorder()
	handler.Show(rec, newRequest(t, "music", model.LocaleJa, &model.User{Atname: "alice"}))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(rec.Body.String(), "Internal Server Error") {
		t.Error("response body does not contain Internal Server Error")
	}
}

// TestShow_BoardListingFailure verifies the third database failure branch:
// resolving the category and reading the navigation both succeed, then the board
// listing fails and returns an internal server error. Splitting the listing out
// of the category lookup gave this failure its own branch, so it needs a case of
// its own to stay covered.
//
// [Ja] TestShow_BoardListingFailure は 3 つ目の DB 失敗分岐を検証します。カテゴリーの
// 解決とナビゲーションの読み取りには成功し、その後の掲示板一覧の取得が失敗して Internal
// Server Error を返します。一覧をカテゴリーの解決から切り離したことでこの失敗が独立した
// 分岐になったため、覆い続けるには独立したケースが要ります。
func TestShow_BoardListingFailure(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	categoryDB := testutil.SetupDB(t)
	categoryRepo := repository.NewCategoryRepository(categoryDB)
	if _, err := categoryRepo.Create(ctx, repository.CreateCategoryInput{Slug: "music", Name: "音楽"}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	boardDB := testutil.SetupDB(t)
	if err := boardDB.Reader.Close(); err != nil {
		t.Fatalf("board Reader の Close() error = %v", err)
	}

	handler := newHandlerForDatabases(categoryDB, categoryDB, boardDB)
	rec := httptest.NewRecorder()
	handler.Show(rec, newRequest(t, "music", model.LocaleJa, &model.User{Atname: "alice"}))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(rec.Body.String(), "Internal Server Error") {
		t.Error("response body does not contain Internal Server Error")
	}
}

// listColumn returns the part of the document from the <main> landmark onwards,
// so an assertion about the board listing is not answered by the sidebar, which
// links the same boards earlier in the document.
//
// [Ja] listColumn は文書の <main> ランドマーク以降を返し、掲示板の一覧についての検証
// が、同じ掲示板を文書のより前でリンクしているサイドバーによって満たされてしまわない
// ようにします。
func listColumn(t *testing.T, body string) string {
	t.Helper()

	at := strings.Index(body, `id="main"`)
	if at < 0 {
		t.Fatalf("レスポンスボディに %q が含まれていない", `id="main"`)
	}
	return body[at:]
}
