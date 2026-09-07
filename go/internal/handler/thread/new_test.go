package thread_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/templates"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// newBoardDB creates a database holding one community with a "music" category
// listing the "jazz" board, and a "quiet" board sitting in no category.
//
// Two boards are created because the trail the form draws differs between them:
// a board in a category is named below it, and a board in none (ADR 0011) starts
// the trail at the board itself. The form itself is the same on both.
//
// [Ja] newBoardDB は、1 つのコミュニティを持つデータベースを作ります。"music"
// カテゴリーが "jazz" 掲示板を並べ、"quiet" 掲示板はどのカテゴリーにも属しません。
//
// 掲示板を 2 つ作るのは、フォームが描く経路が両者で異なるためです。カテゴリーに属する
// 掲示板はその下に名指され、どのカテゴリーにも属さない掲示板 (ADR 0011) では経路が掲示板
// 自身から始まります。フォーム自体はどちらでも同じです。
func newBoardDB(t *testing.T) *database.DB {
	t.Helper()

	ctx := context.Background()
	db := testutil.SetupDB(t)

	if _, err := db.Writer.ExecContext(ctx, "INSERT INTO communities (id, name) VALUES (1, ?)", communityName); err != nil {
		t.Fatalf("communities への INSERT に失敗: %v", err)
	}

	categoryRepo := repository.NewCategoryRepository(db)
	boardRepo := repository.NewBoardRepository(db)

	music, err := categoryRepo.Create(ctx, repository.CreateCategoryInput{Slug: "music", Name: "音楽"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := boardRepo.Create(ctx, repository.CreateBoardInput{
		CategoryID: &music.ID,
		Slug:       "jazz",
		Name:       "ジャズ・ファンク",
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := boardRepo.Create(ctx, repository.CreateBoardInput{Slug: "quiet", Name: "雑談"}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	return db
}

// newFormRequest builds a GET /b/{slug}/threads/new request as the router would
// hand it to the handler: the slug in chi's route context, and the locale, the
// current path and the viewer in the request context, placed there directly the
// way i18n's, templates' and the auth middleware would. A nil user is an
// anonymous visitor, which in production RequireAuth turns away before the
// handler runs.
//
// [Ja] newFormRequest は、ルーターがハンドラーへ渡すのと同じ形で
// GET /b/{slug}/threads/new のリクエストを組み立てます。slug は chi のルート context に、
// ロケール・現在のパス・閲覧者はリクエスト context に、i18n・templates・認証の各ミドル
// ウェアがするのと同じように直接置きます。user が nil のときは匿名の訪問者で、本番では
// ハンドラーが走る前に RequireAuth が追い返します。
func newFormRequest(t *testing.T, slug string, locale model.Locale, user *model.User) *http.Request {
	t.Helper()

	path := templates.BoardThreadsNewPath(slug).String()
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

// TestNew verifies that GET /b/{slug}/threads/new returns HTTP 200 with an HTML
// body that renders, for each supported locale, the thread-creation form inside
// the community shell: the trail back to the board it will post to, the board's
// name in the lead, the three labelled fields with the limits they are judged
// by, the interval between one person's posts, and the CSRF token the submission
// carries.
//
// The primary language opens on the one the page is drawn in, so the choice is
// already made for the visitor who writes in the language they are reading.
//
// The response asks not to be indexed and not to be stored, since it is a form
// behind authentication that a re-render fills with what the visitor typed.
//
// [Ja] TestNew は GET /b/{slug}/threads/new が HTTP 200 と、サポートする各ロケールに
// ついてコミュニティのシェルの中にスレッド作成フォームを描画した HTML ボディを返すことを
// 検証します。投稿先の掲示板へ戻る経路、説明文の中の掲示板名、判定に使われる上限を添えた
// ラベル付きの 3 つのフィールド、1 人の投稿と投稿の間隔、そして送信が運ぶ CSRF トークン
// です。
//
// 主言語はページが描かれている言語で開きます。読んでいる言語で書く訪問者にとって、選択は
// 既に済んでいることになります。
//
// レスポンスはインデックスしないこと・保存しないことを求めます。認証の背後にあるフォーム
// であり、再描画は訪問者が打った内容で埋まるためです。
func TestNew(t *testing.T) {
	t.Parallel()

	handler := newHandlerForDB(newBoardDB(t))

	tests := []struct {
		name            string
		locale          model.Locale
		wantHeading     string
		wantLead        string
		wantTitleLabel  string
		wantTitleHint   string
		wantBodyLabel   string
		wantBodyHint    string
		wantInterval    string
		wantOtherOption string
		selectedValue   string
		unselectedValue string
	}{
		{
			name:            "Japanese",
			locale:          model.LocaleJa,
			wantHeading:     "スレッドを立てる",
			wantLead:        "ジャズ・ファンク に新しいスレッドを立てます。",
			wantTitleLabel:  "タイトル",
			wantTitleHint:   "100文字以内で入力してください",
			wantBodyLabel:   "最初の投稿",
			wantBodyHint:    "10,000文字以内で入力してください",
			wantInterval:    "続けて投稿するときは10秒の間隔が必要です。",
			wantOtherOption: "その他",
			selectedValue:   "ja",
			unselectedValue: "en",
		},
		{
			name:            "English",
			locale:          model.LocaleEn,
			wantHeading:     "Start a thread",
			wantLead:        "Starting a new thread in ジャズ・ファンク.",
			wantTitleLabel:  "Title",
			wantTitleHint:   "Up to 100 characters",
			wantBodyLabel:   "First post",
			wantBodyHint:    "Up to 10,000 characters",
			wantInterval:    "Posting again takes an interval of 10 seconds.",
			wantOtherOption: "Other",
			selectedValue:   "en",
			unselectedValue: "ja",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			handler.New(rec, newFormRequest(t, "jazz", tt.locale, &model.User{Atname: "alice"}))

			if rec.Code != http.StatusOK {
				t.Errorf("status code = %d, want %d", rec.Code, http.StatusOK)
			}
			if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
				t.Errorf("Content-Type = %q, want prefix %q", got, "text/html")
			}
			if got, want := rec.Header().Get("Cache-Control"), "private, no-store"; got != want {
				t.Errorf("Cache-Control = %q, want %q", got, want)
			}

			body := rec.Body.String()
			if got := parsedTextareaValue(t, body); got != "" {
				t.Errorf("initial textarea value = %q, want empty", got)
			}
			if strings.Contains(body, `id="thread-new-errors"`) {
				t.Error("initial form should not display an error summary")
			}
			wants := []string{
				"<title>" + tt.wantHeading + " - " + communityName + "</title>",
				tt.wantHeading,
				tt.wantLead,
				tt.wantTitleLabel,
				tt.wantTitleHint,
				tt.wantBodyLabel,
				tt.wantBodyHint,
				tt.wantInterval,
				tt.wantOtherOption,
				`<meta name="robots" content="noindex"`,
				`name="csrf_token"`,
				`href="/c/music"`,
				`href="/b/jazz"`,
				`<html lang="` + string(tt.locale) + `"`,
			}
			for _, want := range wants {
				if !strings.Contains(body, want) {
					t.Errorf("response body does not contain %q", want)
				}
			}

			// The board is part of the address the form posts to rather than a
			// field, so the form can only start a thread in the board it was
			// opened from.
			//
			// [Ja] 掲示板はフィールドではなくフォームの送信先のアドレスの一部であるため、
			// フォームはそれが開かれた掲示板にしかスレッドを立てられない。
			action := `action="` + templates.BoardThreadsPath("jazz").String() + `"`
			formTag := testutil.OpeningTag(t, body, action)
			if !strings.HasPrefix(formTag, "<form ") || !strings.Contains(formTag, `method="POST"`) {
				t.Errorf("フォームの開始タグ = %s, want a POST form to %s", formTag, action)
			}
			if strings.Contains(body, `name="board`) {
				t.Error("フォームに掲示板を選ぶフィールドがある")
			}

			// Each control is labelled, marked required, and pointed at the hint
			// stating the limit its submission is judged by. The limits are not
			// written as maxlength, which counts UTF-16 code units where the
			// server counts code points.
			//
			// [Ja] 各入力欄はラベルを持ち、必須の印が付き、送信が判定される上限を述べる
			// ヒントを指す。上限は maxlength には書かない。この属性が数えるのは UTF-16 の
			// コード単位で、サーバーが数えるのはコードポイントであるため。
			for _, field := range []string{"title", "language", "body"} {
				label := testutil.OpeningTag(t, body, `for="`+field+`"`)
				if !strings.HasPrefix(label, "<label ") {
					t.Errorf("%s のラベル = %s, want label", field, label)
				}
				control := testutil.OpeningTag(t, body, `id="`+field+`"`)
				for _, want := range []string{"required", `aria-describedby="` + field + `-hint"`} {
					if !strings.Contains(control, want) {
						t.Errorf("%s の入力欄に %q が無い: %s", field, want, control)
					}
				}
				if strings.Contains(control, "maxlength") {
					t.Errorf("%s の入力欄に maxlength がある: %s", field, control)
				}

				// A form with nothing to fix opens with the caret in the title, the
				// field it is filled in from.
				//
				// [Ja] 直すところの無いフォームは、それが埋められていく最初の欄である
				// タイトルにキャレットを置いて開く。
				if want := field == "title"; strings.Contains(control, "autofocus") != want {
					t.Errorf("%s の入力欄の autofocus = %v, want %v: %s", field, !want, want, control)
				}
			}

			// The select opens on the language the page is drawn in, and offers
			// only the languages a thread may be written in.
			//
			// [Ja] select はページが描かれている言語で開き、スレッドを書ける言語だけを
			// 差し出す。
			selected := testutil.OpeningTag(t, body, `value="`+tt.selectedValue+`"`)
			if !strings.Contains(selected, "selected") {
				t.Errorf("%q の選択肢 = %s, want selected", tt.selectedValue, selected)
			}
			unselected := testutil.OpeningTag(t, body, `value="`+tt.unselectedValue+`"`)
			if strings.Contains(unselected, "selected") {
				t.Errorf("%q の選択肢 = %s, want not selected", tt.unselectedValue, unselected)
			}
			if got, want := strings.Count(body, "<option "), len(model.ThreadLanguages()); got != want {
				t.Errorf("選択肢の数 = %d, want %d", got, want)
			}

			// The form can be submitted: the route it posts to accepts it, so the
			// button is the way what was written reaches the board.
			//
			// [Ja] フォームは送信できる。送信先のルートがそれを受け付けるため、書かれた
			// ものが掲示板へ届く手立てがこのボタンである。
			submit := testutil.OpeningTag(t, testutil.Element(t, body, action, "</form>"), `type="submit"`)
			if strings.Contains(submit, "disabled") {
				t.Errorf("送信ボタン = %s, want enabled", submit)
			}

			main := testutil.OpeningTag(t, body, `id="main"`)
			if !strings.HasPrefix(main, "<main ") || !strings.Contains(main, `aria-labelledby="thread-new-heading"`) {
				t.Errorf("main landmark = %s, want the page heading as its accessible name", main)
			}
			heading := testutil.OpeningTag(t, body, `id="thread-new-heading"`)
			if !strings.HasPrefix(heading, "<h1 ") {
				t.Errorf("main landmark を名付ける要素 = %s, want h1", heading)
			}

			// A page asking not to be indexed publishes no trail for a search
			// result to show, and declares no canonical address.
			//
			// [Ja] インデックスされないよう求めるページは、検索結果が示す経路を公開せず、
			// 正規アドレスも宣言しない。
			if strings.Contains(body, "application/ld+json") {
				t.Error("noindex のページに構造化データが含まれている")
			}
			if strings.Contains(body, `rel="canonical"`) {
				t.Error("noindex のページに canonical のリンクが含まれている")
			}
		})
	}
}

// TestNew_BoardWithoutCategory verifies that the form opened from a board
// sitting in no category (ADR 0011) starts its trail at the board. Unlike a
// board's own page, the trail is not left empty: the board above the form is
// still a place to name and the way back out of it.
//
// [Ja] TestNew_BoardWithoutCategory は、どのカテゴリーにも属さない掲示板 (ADR 0011)
// から開いたフォームの経路が掲示板から始まることを検証します。掲示板自身のページと違い
// 経路は空になりません。フォームの上位である掲示板は名指す場所であり、そこから出る道でも
// あるためです。
func TestNew_BoardWithoutCategory(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	newHandlerForDB(newBoardDB(t)).New(rec, newFormRequest(t, "quiet", model.LocaleJa, &model.User{Atname: "alice"}))

	if rec.Code != http.StatusOK {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	if !strings.Contains(body, `href="/b/quiet"`) {
		t.Error("パンくずに掲示板へ戻るリンクが無い")
	}
	if strings.Contains(body, `href="/c/`) {
		t.Error("どのカテゴリーにも属さない掲示板のパンくずにカテゴリーの段がある")
	}
}

// TestNew_RedirectsNonCanonicalSlugToCanonicalPath verifies that a slug reaching
// the board through the database's case-insensitive collation is redirected to
// the stored spelling, so the form answers under one address however the visitor
// arrived at it. The board's own page normalizes the same way.
//
// The Cache-Control of the redirect is asserted alongside the status, because a
// permanent redirect can be held by the visitor's browser, while the CSRF cookie
// a safe request may mint must keep it out of shared caches.
//
// [Ja] TestNew_RedirectsNonCanonicalSlugToCanonicalPath は、DB の大文字小文字を無視する
// 照合で掲示板へ到達した slug が、保存されている綴りへリダイレクトされることを検証します。
// これにより、訪問者がどう辿り着いてもフォームは 1 つのアドレスで応答します。掲示板自身の
// ページも同じ正規化を行います。
//
// リダイレクトの Cache-Control をステータスと併せて検証するのは、恒久リダイレクトを訪問者の
// ブラウザには保持させながら、安全なリクエストが発行しうる CSRF Cookie を共有キャッシュには
// 保存させないためです。
func TestNew_RedirectsNonCanonicalSlugToCanonicalPath(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	newHandlerForDB(newBoardDB(t)).New(rec, newFormRequest(t, "JAZZ", model.LocaleJa, &model.User{Atname: "alice"}))

	if rec.Code != http.StatusPermanentRedirect {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusPermanentRedirect)
	}
	if got, want := rec.Header().Get("Location"), templates.BoardThreadsNewPath("jazz").String(); got != want {
		t.Errorf("Location = %q, want %q", got, want)
	}
	if got, want := rec.Header().Get("Cache-Control"), "private, max-age=3600"; got != want {
		t.Errorf("Cache-Control = %q, want %q", got, want)
	}
}

// TestNew_NotFound verifies that a slug naming no board is answered with the
// shared 404 page rather than a form that would post to a board that is not
// there.
//
// [Ja] TestNew_NotFound は、どの掲示板も指さない slug に、そこに無い掲示板へ送信する
// ことになるフォームではなく共通の 404 ページで応答することを検証します。
func TestNew_NotFound(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	newHandlerForDB(newBoardDB(t)).New(rec, newFormRequest(t, "unknown", model.LocaleJa, &model.User{Atname: "alice"}))

	if rec.Code != http.StatusNotFound {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if !strings.Contains(rec.Body.String(), "ページが見つかりません") {
		t.Error("404 ページの文言が含まれていない")
	}
}

// TestNew_RedirectsAnonymousToSignInWithReturnTo verifies that the route is
// guarded and that an anonymous visitor is sent to sign-in carrying this
// address, so following a shared link to the form lands them back on it once
// signed in rather than on the home page.
//
// The request goes through a router registering the route the way serve.go does,
// because what turns the visitor away is the middleware the route is registered
// behind rather than anything in the handler.
//
// [Ja] TestNew_RedirectsAnonymousToSignInWithReturnTo は、このルートが保護されており、
// 匿名の訪問者がこのアドレスを載せてサインインへ送られることを検証します。これにより、
// 共有されたフォームのリンクを踏んだ人は、サインイン後にホームではなくそのフォームへ
// 着地します。
//
// リクエストをルーター経由で流すのは、訪問者を追い返すのがハンドラーの中の何かではなく、
// そのルートが登録されている背後のミドルウェアであるためです。ルートの登録は serve.go と
// 同じ形にしています。
func TestNew_RedirectsAnonymousToSignInWithReturnTo(t *testing.T) {
	t.Parallel()

	db := newBoardDB(t)
	auth := middleware.NewAuth(session.NewManager(repository.NewUserRepository(db), &config.Config{Env: "test"}))

	router := chi.NewRouter()
	router.With(auth.RequireAuth).Get("/b/{slug}/threads/new", newHandlerForDB(db).New)

	path := templates.BoardThreadsNewPath("jazz").String()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req = req.WithContext(i18n.SetLocale(req.Context(), model.LocaleJa))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got, want := rec.Header().Get("Location"), templates.SignInPath().WithReturnTo(path).String(); got != want {
		t.Errorf("Location = %q, want %q", got, want)
	}
}

// TestNew_BoardLookupFailure verifies that a board read that fails is answered
// with an internal server error rather than the 404 page. An unreachable
// database does not mean the board is gone, and answering 404 would tell a
// crawler to drop a form that is still there.
//
// [Ja] TestNew_BoardLookupFailure は、掲示板の読み取りの失敗が 404 ページではなく
// Internal Server Error として返ることを検証します。到達できないデータベースは掲示板が
// 無くなったことを意味せず、404 で応答すればまだ存在するフォームを落とすようクローラーに
// 伝えてしまいます。
func TestNew_BoardLookupFailure(t *testing.T) {
	t.Parallel()

	db := newBoardDB(t)
	boardDB := testutil.SetupDB(t)
	if err := boardDB.Reader.Close(); err != nil {
		t.Fatalf("board Reader の Close() error = %v", err)
	}

	rec := httptest.NewRecorder()
	handler := newHandlerForDatabases(db, boardDB, db, db)
	handler.New(rec, newFormRequest(t, "jazz", model.LocaleJa, &model.User{Atname: "alice"}))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(rec.Body.String(), "Internal Server Error") {
		t.Error("response body does not contain Internal Server Error")
	}
}

// TestNew_NavigationLookupFailure verifies the second database failure branch:
// the board is resolved, then the navigation the shell renders fails and returns
// an internal server error. Keeping the databases separate prevents the first
// read from consuming the intended failure.
//
// [Ja] TestNew_NavigationLookupFailure は 2 つ目の DB 失敗分岐を検証します。掲示板の
// 解決には成功し、その後のシェルが描くナビゲーションの取得が失敗して Internal Server
// Error を返します。DB を分けることで、最初の読み取りが対象の失敗を先に消費しないように
// します。
func TestNew_NavigationLookupFailure(t *testing.T) {
	t.Parallel()

	db := newBoardDB(t)
	navigationDB := testutil.SetupDB(t)
	if err := navigationDB.Reader.Close(); err != nil {
		t.Fatalf("navigation Reader の Close() error = %v", err)
	}

	rec := httptest.NewRecorder()
	handler := newHandlerForDatabases(db, db, navigationDB, db)
	handler.New(rec, newFormRequest(t, "jazz", model.LocaleJa, &model.User{Atname: "alice"}))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(rec.Body.String(), "Internal Server Error") {
		t.Error("response body does not contain Internal Server Error")
	}
}
