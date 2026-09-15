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

// appURLは本テストでのインスタンスの公開ベースURLであり、ページのcanonicalの
// リンクがその下で組み立てられる値です。
const appURL = "https://groobb.example.com"

// communityNameは本テストでこのインスタンスが運営するコミュニティの名前です。
// ページはこれをリクエストのcontextから読み (本番ではミドルウェアがそこへ置きます)、
// タイトルの末尾に置きます。
const communityName = "ジャズ喫茶"

// newHandlerは、1つのコミュニティと2つのカテゴリーを持つデータベース上に
// category Handlerを構築します。"music" は作成順とは逆の順序で2つの掲示板を並べ、
// "empty" は1つも並べません。この2つで、このページが描画するもの — コミュニティが
// 各掲示板に与えたposition由来の並び順と、コミュニティがまだ掲示板を置いていない
// 状態 — を覆えます。
func newHandler(t *testing.T) *category.Handler {
	t.Helper()

	ctx := context.Background()
	db := testutil.SetupDB(t)

	if _, err := db.Writer.ExecContext(ctx, "INSERT INTO communities (id, name) VALUES (1, ?)", communityName); err != nil {
		t.Fatalf("communitiesへのINSERTに失敗: %v", err)
	}

	categoryRepo := repository.NewCategoryRepository(db)
	boardRepo := repository.NewBoardRepository(db)

	music, err := categoryRepo.Create(ctx, repository.CreateCategoryInput{Slug: "music", Name: "音楽", Position: 1})
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}
	if _, err := categoryRepo.Create(ctx, repository.CreateCategoryInput{Slug: "empty", Name: "準備中", Position: 2}); err != nil {
		t.Fatalf("Create()のエラー = %v", err)
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
			t.Fatalf("Create()のエラー = %v", err)
		}
	}
	createBoard("rock", "ロック", "ロックの話をする板", 2)
	createBoard("jazz", "ジャズ・ファンク", "ジャズの話をする板", 1)

	return newHandlerForDB(db)
}

// newHandlerForDBは、渡されたアプリケーションデータベース上にcategory Handlerを
// 構築します。
func newHandlerForDB(db *database.DB) *category.Handler {
	return newHandlerForDatabases(db, db, db)
}

// newHandlerForDatabasesは、3つの読み取り (カテゴリーの解決・コミュニティの
// ナビゲーション・掲示板の一覧) それぞれの背後に別々のデータベースを置いてcategory
// Handlerを構築します。本番は3つとも同じデータベースを渡しますが、テストでは1つの
// 読み取りだけを壊し、残りが対象分岐へ到達できます。
func newHandlerForDatabases(categoryDB, navigationDB, boardDB *database.DB) *category.Handler {
	getCommunityNavigationUC := usecase.NewGetCommunityNavigationUsecase(
		repository.NewCommunityRepository(navigationDB),
		repository.NewBoardRepository(navigationDB),
		repository.NewRoleRepository(navigationDB),
	)
	getCategoryUC := usecase.NewGetCategoryUsecase(repository.NewCategoryRepository(categoryDB))
	getCategoryBoardsUC := usecase.NewGetCategoryBoardsUsecase(repository.NewBoardRepository(boardDB))

	cfg := &config.Config{Env: "dev", AppURL: appURL}
	return category.NewHandler(cfg, httperror.NewRenderer(cfg), getCommunityNavigationUC, getCategoryUC, getCategoryBoardsUC)
}

// newRequestは、ルーターがハンドラーへ渡すのと同じ形でGET /c/{slug} の
// リクエストを組み立てます。slugはchiのルートcontextに、ロケール・現在のパス・
// 閲覧者はリクエストcontextに、i18n・templates・認証の各ミドルウェアがするのと同じ
// ように直接置きます。userがnilのときは匿名の訪問者です。
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

// TestShowはGET /c/{slug} がHTTP 200と、サポートする各ロケールについて
// コミュニティのシェルの中にカテゴリーページを描画したHTMLボディを返すことを検証
// します。<main> ランドマークを名付ける <h1> としてのカテゴリー名、positionが与える
// 順序で並ぶ掲示板とその説明、サイドバーとそのアカウント操作、そしてまだ開かれていない
// スレッドの代わりを務める補助カラムです。コミュニティのカテゴリーは公開であるため、
// このページはnoindexを持ちません。
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
			name:            "日本語",
			locale:          model.LocaleJa,
			wantDescription: "音楽 カテゴリーの掲示板の一覧です。",
			wantRegionLabel: "スレッドの閲覧",
			wantPrompt:      "掲示板を選ぶと、その中のスレッドが表示されます。",
		},
		{
			name:            "英語",
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
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
			}
			if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
				t.Errorf("Content-Type = %q、期待値の接頭辞 = %q", got, "text/html")
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
					t.Errorf("レスポンスボディに %q が含まれていない", want)
				}
			}

			// タッチターゲットの要件が拠って立つクラスだけを検証する。リンクの
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
				t.Error("公開ページのレスポンスにnoindexが含まれている")
			}

			main := testutil.OpeningTag(t, body, `id="main"`)
			if !strings.HasPrefix(main, "<main ") || !strings.Contains(main, `aria-labelledby="category-show-heading"`) {
				t.Errorf("main landmark = %s、ページの見出しをアクセシブルネームに持つことを期待", main)
			}
			heading := testutil.OpeningTag(t, body, `id="category-show-heading"`)
			if !strings.HasPrefix(heading, "<h1 ") {
				t.Errorf("main landmarkを名付ける要素 = %s、期待値 = h1", heading)
			}
			aside := testutil.OpeningTag(t, body, `aria-label="`+tt.wantRegionLabel+`"`)
			if !strings.HasPrefix(aside, "<aside ") {
				t.Errorf("スレッド領域の要素 = %s、期待値 = aside", aside)
			}
			// 説明は一覧カラムにしか現れないため、その位置を比べることで、同じ2つの
			// 掲示板がリンクされているサイドバーではなく一覧の並び順を確かめられる。
			if got, want := strings.Index(body, "ジャズの話をする板"), strings.Index(body, "ロックの話をする板"); got > want {
				t.Error("掲示板がpositionの順に並んでいない")
			}
		})
	}
}

// TestShow_LeavesTheSidebarUnmarkedは、カテゴリーを描画している間サイドバーが
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
		t.Errorf("aria-current=\"page\" の数 = %d、期待値 = 0 (サイドバーの行き先はいずれもこのページではない)", got)
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
	handler.Show(rec, newRequest(t, "MUSIC", model.LocaleJa, &model.User{Atname: "alice"}))

	if rec.Code != http.StatusPermanentRedirect {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusPermanentRedirect)
	}
	if got, want := rec.Header().Get("Location"), templates.CategoryPath("music").String(); got != want {
		t.Errorf("Location = %q、期待値 = %q", got, want)
	}
	if got, want := rec.Header().Get("Cache-Control"), "private, max-age=3600"; got != want {
		t.Errorf("Cache-Control = %q、期待値 = %q", got, want)
	}
}

// TestShow_RedirectToCanonicalSlugKeepsQueryは、クエリ文字列がリダイレクトを
// 越えて残ることを検証します。URLを非正規にしているのはslugの綴りだけであるため、
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
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusPermanentRedirect)
	}
	want := templates.CategoryPath("music").String() + "?utm_source=newsletter&page=2"
	if got := rec.Header().Get("Location"); got != want {
		t.Errorf("Location = %q、期待値 = %q", got, want)
	}
}

// TestShow_AnonymousVisitorは、サインアウト状態の訪問者にもカテゴリーとその
// 掲示板が届くこと、サイドバーのアカウント操作が描画されないこと、そしてその位置に
// アカウントを持つための導線が立つことを検証します。コミュニティのページはアカウント
// 無しで読めるため、ページがアカウントの存在に依存してはなりません。そしてこのカテゴリーの
// ページは訪問者が参加を決める場所の1つであるため、サインインのリンクは参加した訪問者を
// このカテゴリーへ連れ戻します。
func TestShow_AnonymousVisitor(t *testing.T) {
	t.Parallel()

	handler := newHandler(t)
	rec := httptest.NewRecorder()
	handler.Show(rec, newRequest(t, "music", model.LocaleJa, nil))

	if rec.Code != http.StatusOK {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
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

// TestShow_EmptyCategoryは、コミュニティがまだ掲示板を置いていないカテゴリーが、
// 見出しの下に何も無い状態ではなくその旨を伝えること、そして読むためのカラムが、何も
// 入っていない一覧から選ぶよう促すのをやめることを検証します。
func TestShow_EmptyCategory(t *testing.T) {
	t.Parallel()

	handler := newHandler(t)
	rec := httptest.NewRecorder()
	handler.Show(rec, newRequest(t, "empty", model.LocaleJa, &model.User{Atname: "alice"}))

	if rec.Code != http.StatusOK {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	if !strings.Contains(body, `<meta name="robots" content="noindex">`) {
		t.Error("掲示板を持たないカテゴリーのレスポンスにnoindexが含まれていない")
	}
	if strings.Contains(body, `rel="canonical"`) {
		t.Error("インデックスを求めないカテゴリーがcanonicalのリンクを持っている")
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

// TestShow_DeclaresItsCanonicalURLは、掲示板を1つ以上並べるカテゴリーが、自身を
// 知られるべきアドレスとして自身のアドレスを宣言することを検証します。キャンペーンの
// パラメータを付けて到達した同じページが、2つ目のページとして数えられないようにする
// ためです。
func TestShow_DeclaresItsCanonicalURL(t *testing.T) {
	t.Parallel()

	handler := newHandler(t)
	rec := httptest.NewRecorder()
	handler.Show(rec, newRequest(t, "music", model.LocaleJa, nil))

	canonical := testutil.OpeningTag(t, rec.Body.String(), `rel="canonical"`)
	if want := `href="` + appURL + "/c/music" + `"`; !strings.Contains(canonical, want) {
		t.Errorf("canonicalのリンク = %s、%s を含むことを期待", canonical, want)
	}
}

// TestShow_UnknownSlugは、どのカテゴリーも指さないslugがHTTP 404と共通の
// not-foundページで応答されることを検証します。ステータスをボディと併せて検証するのは、
// 「見つからない」と読めるページが200で応答する状態がソフト404だからです。そして
// このルートには、削除済みのカテゴリーへのリンクを辿るクローラーが到達しえます。
func TestShow_UnknownSlug(t *testing.T) {
	t.Parallel()

	handler := newHandler(t)
	rec := httptest.NewRecorder()
	handler.Show(rec, newRequest(t, "no-such-category", model.LocaleJa, &model.User{Atname: "alice"}))

	if rec.Code != http.StatusNotFound {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusNotFound)
	}
	if body := rec.Body.String(); !strings.Contains(body, "ページが見つかりません") {
		t.Error("未知のslugのレスポンスに404ページの見出しが含まれていない")
	}
}

// TestShow_LookupFailureは、カテゴリーの読み取りの失敗が404ではなくInternal
// Server Errorとして返ることを検証します。到達できないデータベースはカテゴリーが
// 無くなったことを意味せず、404で応答すればまだ存在するページを落とすようクローラーに
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
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(rec.Body.String(), "Internal Server Error") {
		t.Error("レスポンスボディにInternal Server Errorが含まれていない")
	}
}

// TestShow_NavigationLookupFailureは2つ目のDB失敗分岐を検証します。カテゴリーの
// 取得には成功し、その後のナビゲーション取得が失敗してInternal Server Errorを返します。
// DBを分けることで、最初の取得が対象の失敗を先に消費しないようにします。
func TestShow_NavigationLookupFailure(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	categoryDB := testutil.SetupDB(t)
	categoryRepo := repository.NewCategoryRepository(categoryDB)
	if _, err := categoryRepo.Create(ctx, repository.CreateCategoryInput{Slug: "music", Name: "音楽"}); err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}

	navigationDB := testutil.SetupDB(t)
	if err := navigationDB.Reader.Close(); err != nil {
		t.Fatalf("navigation ReaderのClose()のエラー = %v", err)
	}

	handler := newHandlerForDatabases(categoryDB, navigationDB, categoryDB)
	rec := httptest.NewRecorder()
	handler.Show(rec, newRequest(t, "music", model.LocaleJa, &model.User{Atname: "alice"}))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(rec.Body.String(), "Internal Server Error") {
		t.Error("レスポンスボディにInternal Server Errorが含まれていない")
	}
}

// TestShow_BoardListingFailureは3つ目のDB失敗分岐を検証します。カテゴリーの
// 解決とナビゲーションの読み取りには成功し、その後の掲示板一覧の取得が失敗してInternal
// Server Errorを返します。一覧をカテゴリーの解決から切り離したことでこの失敗が独立した
// 分岐になったため、覆い続けるには独立したケースが要ります。
func TestShow_BoardListingFailure(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	categoryDB := testutil.SetupDB(t)
	categoryRepo := repository.NewCategoryRepository(categoryDB)
	if _, err := categoryRepo.Create(ctx, repository.CreateCategoryInput{Slug: "music", Name: "音楽"}); err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}

	boardDB := testutil.SetupDB(t)
	if err := boardDB.Reader.Close(); err != nil {
		t.Fatalf("board ReaderのClose()のエラー = %v", err)
	}

	handler := newHandlerForDatabases(categoryDB, categoryDB, boardDB)
	rec := httptest.NewRecorder()
	handler.Show(rec, newRequest(t, "music", model.LocaleJa, &model.User{Atname: "alice"}))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(rec.Body.String(), "Internal Server Error") {
		t.Error("レスポンスボディにInternal Server Errorが含まれていない")
	}
}

// listColumnは文書の <main> ランドマーク以降を返し、掲示板の一覧についての検証
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
