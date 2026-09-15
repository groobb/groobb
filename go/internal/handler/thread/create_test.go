package thread_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"golang.org/x/net/html"

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

// newAuthorはdbにアカウントを追加し、セッションが名指す形で返します。投稿が
// 帰属するidと、シェルが表示するatnameです。
func newAuthor(t *testing.T, db *database.DB, atname string) *model.User {
	t.Helper()

	id := testutil.NewUserBuilder(t, db).WithAtname(atname).WithEmail(atname + "@example.com").Build()
	return &model.User{ID: id, Atname: atname}
}

// validSubmissionは検証を通るフォームであり、フォームそのものではなく、整った
// 送信の周りで何が起きるかを問うケースのためのものです。
func validSubmission() url.Values {
	return url.Values{
		"title":    {"枯葉の名演"},
		"language": {string(model.LocaleJa.ThreadLanguage())},
		"body":     {"好きな演奏は?"},
	}
}

// newSubmitRequestは、ルーターがハンドラーへ渡すのと同じ形で
// POST /b/{slug}/threadsのリクエストを組み立てます。フォームはそれが送信されるボディと
// して、slugはchiのルートcontextに、ロケール・現在のパス・書き手はリクエストcontext
// に、i18n・templates・認証の各ミドルウェアがするのと同じように直接置きます。
func newSubmitRequest(t *testing.T, slug string, locale model.Locale, author *model.User, form url.Values) *http.Request {
	t.Helper()

	path := templates.BoardThreadsPath(slug).String()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	ctx := i18n.SetLocale(req.Context(), locale)
	ctx = templates.SetCurrentPath(ctx, path)
	ctx = viewmodel.SetSiteName(ctx, communityName)
	ctx = middleware.SetUserToContext(ctx, author)

	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("slug", slug)
	ctx = context.WithValue(ctx, chi.RouteCtxKey, routeCtx)

	return req.WithContext(ctx)
}

// savedThreadは、データベースが持つ1つのスレッドを、行が述べるとおりに読み戻した
// ものです。検証が、レスポンスが「書かれた」と述べるものではなく、実際に書かれたものに
// ついて語れるようにするためです。
type savedThread struct {
	id       model.ThreadID
	title    string
	language string
	number   int
	body     string
	authorID model.UserID
}

func readSavedThread(t *testing.T, db *database.DB) savedThread {
	t.Helper()

	var saved savedThread
	err := db.Reader.QueryRowContext(context.Background(), `
		SELECT threads.id, threads.title, threads.language, posts.number, posts.body, posts.user_id
		FROM threads JOIN posts ON posts.thread_id = threads.id
	`).Scan(&saved.id, &saved.title, &saved.language, &saved.number, &saved.body, &saved.authorID)
	if err != nil {
		t.Fatalf("保存されたスレッドの読み戻しに失敗: %v", err)
	}

	return saved
}

// boardIDは指定slugの掲示板のidを読みます。テスト対象のハンドラーを介さず、
// ケースが自分で用意する行のためのものです。
func boardID(t *testing.T, db *database.DB, slug string) model.BoardID {
	t.Helper()

	var id model.BoardID
	if err := db.Reader.QueryRowContext(context.Background(), "SELECT id FROM boards WHERE slug = ?", slug).Scan(&id); err != nil {
		t.Fatalf("テスト用掲示板のidの取得に失敗: %v", err)
	}

	return id
}

func countRows(t *testing.T, db *database.DB, table string) int {
	t.Helper()

	var count int
	if err := db.Reader.QueryRowContext(context.Background(), "SELECT count(*) FROM "+table).Scan(&count); err != nil {
		t.Fatalf("%s の件数の取得に失敗: %v", table, err)
	}

	return count
}

// TestCreateは、整った送信がスレッドを立て、その書き手を今書いた投稿へ送ることを
// 検証します。HTTP 303で、新しいスレッドのアドレスに最初の投稿のアンカーを付けた先へ送り、
// スレッドとその投稿は送信されたとおりに保存されます。
//
// 投稿はセッションが名指すアカウントに帰属します。フォームは書き手を名指すフィールドを
// 持たず、本ケースはそれが読まれないことを述べるために1つ送ります。誰の投稿かを述べうる
// 送信は、他人の名前で署名できてしまうためです。
func TestCreate(t *testing.T) {
	t.Parallel()

	db := newBoardDB(t)
	author := newAuthor(t, db, "alice")
	other := newAuthor(t, db, "bob")

	form := validSubmission()
	form.Set("user_id", other.ID.String())

	rec := httptest.NewRecorder()
	newHandlerForDB(db).Create(rec, newSubmitRequest(t, "jazz", model.LocaleJa, author, form))

	if rec.Code != http.StatusSeeOther {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusSeeOther)
	}

	saved := readSavedThread(t, db)
	wantLocation := templates.ThreadPostAnchorPath(viewmodel.ThreadID(saved.id), 1).String()
	if got := rec.Header().Get("Location"); got != wantLocation {
		t.Errorf("Location = %q、期待値 = %q", got, wantLocation)
	}
	if saved.title != "枯葉の名演" {
		t.Errorf("保存されたタイトル = %q、期待値 = %q", saved.title, "枯葉の名演")
	}
	if want := string(model.LocaleJa.ThreadLanguage()); saved.language != want {
		t.Errorf("保存された主言語 = %q、期待値 = %q", saved.language, want)
	}
	if saved.number != 1 {
		t.Errorf("保存されたレス番号 = %d、期待値 = 1", saved.number)
	}
	if saved.body != "好きな演奏は?" {
		t.Errorf("保存された本文 = %q、期待値 = %q", saved.body, "好きな演奏は?")
	}
	if saved.authorID != author.ID {
		t.Errorf("投稿の作者 = %v、期待値 = %v (セッションのアカウント)", saved.authorID, author.ID)
	}
}

// TestCreate_IgnoresQueryStringFieldsは、フィールドが送信されたボディからのみ
// 読まれることを検証します。クエリ文字列にそれらを載せたリンクは何も書きません。
// そうでなければ、サインイン済みの訪問者がそのリンクを踏んだだけで、何も書いていないのに
// その人の名前で投稿されることになります。
func TestCreate_IgnoresQueryStringFields(t *testing.T) {
	t.Parallel()

	db := newBoardDB(t)
	author := newAuthor(t, db, "alice")

	req := newSubmitRequest(t, "jazz", model.LocaleJa, author, url.Values{})
	req.URL.RawQuery = validSubmission().Encode()

	rec := httptest.NewRecorder()
	newHandlerForDB(db).Create(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusUnprocessableEntity)
	}
	if got := countRows(t, db, "threads"); got != 0 {
		t.Errorf("保存されたスレッド = %d 件、期待値 = 0件", got)
	}
	if body := rec.Body.String(); !strings.Contains(body, "入力してください") {
		t.Error("クエリ文字列だけの送信が、空のフォームとして扱われていない")
	}
}

// TestCreate_ValidationErrorは、直すところのある送信が、それが書かれたフォームと
// して返ってくることを検証します。HTTP 422と、それぞれのフィールドに紐づくメッセージ、
// そして訪問者が打った値がすべてそのまま残り、何も2度書かずに済みます。
//
// キャレットは文書の先頭ではなく、直すところのある最初のフィールドに落ち、マークアップに
// 見えるテキストは、ページの一部になるのではなく、そのままのテキストとして表示されます。
func TestCreate_ValidationError(t *testing.T) {
	t.Parallel()

	db := newBoardDB(t)
	author := newAuthor(t, db, "alice")

	form := url.Values{
		"title":    {"<script>alert(1)</script>"},
		"language": {string(model.LocaleEn.ThreadLanguage())},
		"body":     {"   "},
	}

	rec := httptest.NewRecorder()
	newHandlerForDB(db).Create(rec, newSubmitRequest(t, "jazz", model.LocaleJa, author, form))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusUnprocessableEntity)
	}
	if got, want := rec.Header().Get("Cache-Control"), "private, no-store"; got != want {
		t.Errorf("Cache-Control = %q、期待値 = %q", got, want)
	}
	if got := countRows(t, db, "threads"); got != 0 {
		t.Errorf("保存されたスレッド = %d 件、期待値 = 0件", got)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "入力してください") {
		t.Error("再描画されたフォームに本文のエラーメッセージが含まれていない")
	}
	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Error("送信されたタイトルがマークアップとして描画されている")
	}
	if !strings.Contains(body, "&lt;script&gt;") {
		t.Error("再描画されたフォームに、送信されたタイトルがテキストとして残っていない")
	}

	// 主言語は、ページが描かれている言語に戻すのではなく、選ばれたとおりに見せて
	// 返す。戻せば訪問者の選択を黙って取り消すことになる。
	selected := testutil.OpeningTag(t, body, `value="`+string(model.LocaleEn.ThreadLanguage())+`"`)
	if !strings.Contains(selected, "selected") {
		t.Errorf("送信された主言語の選択肢 = %s、selectedを期待", selected)
	}

	// 本文はtextareaの中にエコーバックされる。textareaは値を属性ではなく内容と
	// して持つためである。
	if !strings.Contains(testutil.Element(t, body, `id="body"`, "</textarea>"), "   ") {
		t.Error("再描画されたフォームに、送信された本文が残っていない")
	}

	caret := testutil.OpeningTag(t, body, `id="body"`)
	if !strings.Contains(caret, "autofocus") {
		t.Errorf("直すところのある最初の入力欄 = %s、autofocusを期待", caret)
	}
	if title := testutil.OpeningTag(t, body, `id="title"`); strings.Contains(title, "autofocus") {
		t.Errorf("直すところの無い入力欄 = %s、autofocusが無いことを期待", title)
	}
}

// TestCreate_TooSoonAfterTheLastPostは、1人の投稿と投稿の間隔が尽きる前に届いた
// 送信がHTTP 429で拒否されることを検証します。待ち時間はRetry-Afterとしてブラウザにも
// ページ上にも述べられ、送信は、待ち時間が明けたら送り直せるように保たれます。
//
// 拒否の理由は送信そのものであってそのどのフィールドでもないため、フォームの上の要約が
// キャレットを取り、どの入力にも落ち度の印は付きません。
func TestCreate_TooSoonAfterTheLastPost(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := newBoardDB(t)
	author := newAuthor(t, db, "alice")

	threadRepo := repository.NewThreadRepository(db)
	postRepo := repository.NewPostRepository(db)
	existing, err := threadRepo.Create(ctx, repository.CreateThreadInput{
		BoardID:  boardID(t, db, "jazz"),
		UserID:   &author.ID,
		Title:    "先に立てたスレッド",
		Language: model.LocaleJa.ThreadLanguage(),
	})
	if err != nil {
		t.Fatalf("テスト用スレッドの作成に失敗: %v", err)
	}
	if _, err := postRepo.Create(ctx, repository.CreatePostInput{
		ThreadID: existing.ID,
		UserID:   &author.ID,
		Number:   1,
		Body:     "たった今書いた投稿",
	}); err != nil {
		t.Fatalf("テスト用投稿の作成に失敗: %v", err)
	}

	rec := httptest.NewRecorder()
	newHandlerForDB(db).Create(rec, newSubmitRequest(t, "jazz", model.LocaleJa, author, validSubmission()))

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusTooManyRequests)
	}
	retryAfter, err := strconv.Atoi(rec.Header().Get("Retry-After"))
	if err != nil {
		t.Fatalf("Retry-After = %q、期待値は整数の秒数", rec.Header().Get("Retry-After"))
	}
	if wait := int(model.PostInterval.Seconds()); retryAfter <= 0 || retryAfter > wait {
		t.Errorf("Retry-After = %d、期待値は1以上 %d 以下", retryAfter, wait)
	}
	if got, want := countRows(t, db, "threads"), 1; got != want {
		t.Errorf("スレッド = %d 件、期待値 = %d 件 (拒否された送信は何も残さない)", got, want)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "秒後にもう一度お試しください。") {
		t.Error("再描画されたフォームに、待ち時間を述べる文言が含まれていない")
	}
	if !strings.Contains(testutil.OpeningTag(t, body, `id="title"`), `value="枯葉の名演"`) {
		t.Error("拒否された送信のタイトルがフォームに残っていない")
	}

	// 要約がキャレットを取る。ページが戻ってきた理由が、訪問者のいる位置より上の
	// どこかではなく、その位置にあるようにするためである。
	summary := testutil.Element(t, body, `tabindex="-1" autofocus`, "</div>")
	if !strings.Contains(summary, "秒後にもう一度お試しください。") {
		t.Errorf("フォーカスを取る要素 = %s、期待値はエラーの要約", summary)
	}
	for _, field := range []string{"title", "language", "body"} {
		control := testutil.OpeningTag(t, body, `id="`+field+`"`)
		if strings.Contains(control, "aria-invalid") {
			t.Errorf("%s の入力欄が不正扱いされている: %s", field, control)
		}
		if strings.Contains(control, "autofocus") {
			t.Errorf("%s の入力欄がフォーカスを取っている: %s", field, control)
		}
	}
}

// TestCreate_WithdrawnAccountは、アカウントが去ったセッションがHTTP 403で拒否
// され、フォーム上でその旨を伝えられることを検証します。もう存在しないアカウントに投稿が
// 帰属することはありません。
func TestCreate_WithdrawnAccount(t *testing.T) {
	t.Parallel()

	db := newBoardDB(t)
	withdrawn := &model.User{
		ID:     testutil.NewUserBuilder(t, db).WithAtname("bob").WithDeletedAt(time.Now().Add(-24 * time.Hour)).Build(),
		Atname: "bob",
	}

	rec := httptest.NewRecorder()
	newHandlerForDB(db).Create(rec, newSubmitRequest(t, "jazz", model.LocaleJa, withdrawn, validSubmission()))

	if rec.Code != http.StatusForbidden {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusForbidden)
	}
	if got := countRows(t, db, "posts"); got != 0 {
		t.Errorf("保存された投稿 = %d 件、期待値 = 0件", got)
	}
	if !strings.Contains(rec.Body.String(), "このアカウントでは投稿できません。") {
		t.Error("再描画されたフォームに、投稿できないことを伝える文言が含まれていない")
	}
}

// TestCreate_UnknownBoardは、どの掲示板も指さないslugへの送信が、そこに無い
// 掲示板のフォームではなく共通の404ページで応答されることを検証します。
func TestCreate_UnknownBoard(t *testing.T) {
	t.Parallel()

	db := newBoardDB(t)
	author := newAuthor(t, db, "alice")

	rec := httptest.NewRecorder()
	newHandlerForDB(db).Create(rec, newSubmitRequest(t, "unknown", model.LocaleJa, author, validSubmission()))

	if rec.Code != http.StatusNotFound {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusNotFound)
	}
	if !strings.Contains(rec.Body.String(), "ページが見つかりません") {
		t.Error("404ページの文言が含まれていない")
	}
}

// TestCreate_NonCanonicalSlugは、DBの大文字小文字を無視する照合で掲示板へ到達した
// 送信が、リダイレクトされずに保存されることを検証します。リダイレクトはPOSTをGETに
// 変え、書かれたものを落としてしまいます。正規のアドレスは、保存に続く303で届きます。
func TestCreate_NonCanonicalSlug(t *testing.T) {
	t.Parallel()

	db := newBoardDB(t)
	author := newAuthor(t, db, "alice")

	rec := httptest.NewRecorder()
	newHandlerForDB(db).Create(rec, newSubmitRequest(t, "JAZZ", model.LocaleJa, author, validSubmission()))

	if rec.Code != http.StatusSeeOther {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusSeeOther)
	}

	saved := readSavedThread(t, db)
	if want := templates.ThreadPostAnchorPath(viewmodel.ThreadID(saved.id), 1).String(); rec.Header().Get("Location") != want {
		t.Errorf("Location = %q、期待値 = %q", rec.Header().Get("Location"), want)
	}
}

// TestCreate_SaveFailureは、アプリケーションが保存できなかった送信が、それが
// 書かれたフォームとして (HTTP 500) 返ってくることを検証します。訪問者が書いたものを
// 保つため、打ち直さずにもう一度試せます。
func TestCreate_SaveFailure(t *testing.T) {
	t.Parallel()

	db := newBoardDB(t)
	author := newAuthor(t, db, "alice")
	if err := db.Writer.Close(); err != nil {
		t.Fatalf("WriterのClose()のエラー = %v", err)
	}

	rec := httptest.NewRecorder()
	newHandlerForDB(db).Create(rec, newSubmitRequest(t, "jazz", model.LocaleJa, author, validSubmission()))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusInternalServerError)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "投稿を保存できませんでした。") {
		t.Error("再描画されたフォームに、保存できなかったことを伝える文言が含まれていない")
	}
	if !strings.Contains(testutil.OpeningTag(t, body, `id="title"`), `value="枯葉の名演"`) {
		t.Error("保存できなかった送信のタイトルがフォームに残っていない")
	}
}

// TestCreate_ThroughRouterは、配信される形でのルートを検証します。ボディの上限・
// CSRF検証・RequireAuthの背後にあり、書き手はフォームが運ぶ何かではなくセッション
// Cookieから解決されます。
//
// トークンを伴わない送信は、ハンドラーが走る前に拒否され、何も保存しません。他のサイトから
// 送信されたフォームがその形になります。
func TestCreate_ThroughRouter(t *testing.T) {
	t.Parallel()

	const csrfToken = "test-csrf-token"

	tests := []struct {
		name       string
		withCSRF   bool
		wantStatus int
		wantSaved  int
	}{
		{name: "正常系: CSRFトークンを伴う送信は保存される", withCSRF: true, wantStatus: http.StatusSeeOther, wantSaved: 1},
		{name: "異常系: CSRFトークンの無い送信は拒否される", withCSRF: false, wantStatus: http.StatusForbidden, wantSaved: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			db := newBoardDB(t)
			author := newAuthor(t, db, "alice")
			sessionToken := "session-token-" + strconv.Itoa(tt.wantSaved)
			testutil.NewUserSessionBuilder(t, db).WithUserID(author.ID).WithToken(sessionToken).Build()

			cfg := &config.Config{Env: "test"}
			auth := middleware.NewAuth(session.NewManager(repository.NewUserRepository(db), cfg))
			router := chi.NewRouter()
			router.Use(middleware.PostFormLimit)
			router.Use(middleware.NewCSRF(cfg).Middleware)
			router.With(auth.RequireAuth).Post("/b/{slug}/threads", newHandlerForDB(db).Create)

			form := validSubmission()
			if tt.withCSRF {
				form.Set("csrf_token", csrfToken)
			}

			path := templates.BoardThreadsPath("jazz").String()
			req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.AddCookie(&http.Cookie{Name: middleware.CSRFCookieName, Value: csrfToken})
			req.AddCookie(&http.Cookie{Name: session.CookieName, Value: sessionToken})
			req = req.WithContext(i18n.SetLocale(req.Context(), model.LocaleJa))

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, tt.wantStatus)
			}
			if got := countRows(t, db, "threads"); got != tt.wantSaved {
				t.Errorf("保存されたスレッド = %d 件、期待値 = %d 件", got, tt.wantSaved)
			}
			if tt.wantSaved == 0 {
				return
			}

			saved := readSavedThread(t, db)
			if want := templates.ThreadPostAnchorPath(viewmodel.ThreadID(saved.id), 1).String(); rec.Header().Get("Location") != want {
				t.Errorf("Location = %q、期待値 = %q", rec.Header().Get("Location"), want)
			}
			if saved.authorID != author.ID {
				t.Errorf("投稿の作者 = %v、期待値 = %v (セッションのアカウント)", saved.authorID, author.ID)
			}
		})
	}
}

// TestCreate_ExpiredSessionは、それが書かれたときのセッションが失われた後に行われた
// 送信が、アカウントへ戻る道で応答されること、そしてそれによってスレッドが残らないことを
// 検証します。
//
// 送信が届くアドレスは送信しか受け付けないため、RequireAuthはreturn_toを伴わない
// サインインへ送ります。ここへ連れ戻された訪問者は、POSTしか受け付けないURLにGETで
// 辿り着くことになるためです。タイトルと最初の投稿はそれと共に失われます。拒否された送信を
// 保つのはこのハンドラーが描くフォームであり、ハンドラーの手前で止まった送信には、それを
// 戻して置くフォームがありません。
func TestCreate_ExpiredSession(t *testing.T) {
	t.Parallel()

	const csrfToken = "test-csrf-token"

	db := newBoardDB(t)
	cfg := &config.Config{Env: "test"}
	auth := middleware.NewAuth(session.NewManager(repository.NewUserRepository(db), cfg))
	router := chi.NewRouter()
	router.Use(middleware.PostFormLimit)
	router.Use(middleware.NewCSRF(cfg).Middleware)
	router.With(auth.RequireAuth).Post("/b/{slug}/threads", newHandlerForDB(db).Create)

	form := validSubmission()
	form.Set("csrf_token", csrfToken)

	path := templates.BoardThreadsPath("jazz").String()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: middleware.CSRFCookieName, Value: csrfToken})
	// トークンが名指すのは、データベースがもう持っていないセッションである。書かれた
	// ときのセッションが失効した送信が出会う状態がこれにあたる。
	req.AddCookie(&http.Cookie{Name: session.CookieName, Value: "expired-session-token"})
	req = req.WithContext(i18n.SetLocale(req.Context(), model.LocaleJa))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusSeeOther)
	}
	if got, want := rec.Header().Get("Location"), templates.SignInPath().String(); got != want {
		t.Errorf("Location = %q、期待値 = %q", got, want)
	}
	if got := countRows(t, db, "threads"); got != 0 {
		t.Errorf("保存されたスレッド = %d 件、期待値 = 0件", got)
	}
	if got := countRows(t, db, "posts"); got != 0 {
		t.Errorf("保存された投稿 = %d 件、期待値 = 0件", got)
	}
}

// TestCreate_PreservesBodyはHTML解析後のtextareaの値を検証します。HTMLの
// 部分文字列の確認では、パーサーによる先頭改行の消失を検出できないためです。同じ入力を
// 正常に保存するケースも確認し、再試行が保存されるはずだった本文を保つことを検証します。
func TestCreate_PreservesBody(t *testing.T) {
	t.Parallel()

	bodies := []struct {
		name  string
		input string
		want  string
	}{
		{name: "先頭に改行が無い", input: "  本文\t\n", want: "  本文\t\n"},
		{name: "先頭に改行が1つ", input: "\n本文\n", want: "\n本文\n"},
		{name: "先頭に改行が複数", input: "\n\n本文\n", want: "\n\n本文\n"},
		{name: "CRLFの改行", input: "\r\n本文\r\n", want: "\n本文\n"},
		{name: "改行コードの混在", input: "\r\n\r本文\n次\r", want: "\n\n本文\n次\n"},
		{name: "HTMLに見えるテキスト", input: "\n</textarea><script>alert(1)</script>&", want: "\n</textarea><script>alert(1)</script>&"},
	}
	for _, status := range []int{http.StatusSeeOther, http.StatusUnprocessableEntity, http.StatusTooManyRequests, http.StatusInternalServerError} {
		for _, body := range bodies {
			t.Run(strconv.Itoa(status)+"/"+body.name, func(t *testing.T) {
				t.Parallel()
				db := newBoardDB(t)
				author := newAuthor(t, db, "alice")
				handler := newHandlerForDB(db)
				form := validSubmission()
				form.Set("body", body.input)
				switch status {
				case http.StatusUnprocessableEntity:
					form.Set("title", strings.Repeat("a", 101))
				case http.StatusTooManyRequests:
					first := httptest.NewRecorder()
					handler.Create(first, newSubmitRequest(t, "jazz", model.LocaleJa, author, validSubmission()))
					if first.Code != http.StatusSeeOther {
						t.Fatalf("1回目のステータスコード = %d", first.Code)
					}
				case http.StatusInternalServerError:
					if err := db.Writer.Close(); err != nil {
						t.Fatal(err)
					}
				}
				rec := httptest.NewRecorder()
				handler.Create(rec, newSubmitRequest(t, "jazz", model.LocaleJa, author, form))
				if rec.Code != status {
					t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, status)
				}
				if status == http.StatusSeeOther {
					if got := readSavedThread(t, db).body; got != body.want {
						t.Errorf("保存された本文 = %q、期待値 = %q", got, body.want)
					}
					return
				}
				markup := rec.Body.String()
				if strings.Contains(markup, "\r") {
					t.Error("レスポンスに正規化されていないCRが含まれている")
				}
				if got := parsedTextareaValue(t, markup); got != body.want {
					t.Errorf("textareaの値 = %q、期待値 = %q", got, body.want)
				}
			})
		}
	}
}

// parsedTextareaValueはHTML解析後の値を読みます。文字参照の復号とtextareaの
// 先頭改行の規則を適用した結果を検証するためです。
func parsedTextareaValue(t *testing.T, markup string) string {
	t.Helper()
	root, err := html.Parse(strings.NewReader(markup))
	if err != nil {
		t.Fatal(err)
	}
	var value string
	found := false
	var visit func(*html.Node)
	visit = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "textarea" {
			found = true
			for child := n.FirstChild; child != nil; child = child.NextSibling {
				if child.Type == html.TextNode {
					value += child.Data
				}
			}
			return
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(root)
	if !found {
		t.Fatal("textareaが見つからない")
	}
	return value
}

// TestCreate_FieldErrorSummaryは修正箇所をまとめて表示しつつ、フォームの読み順で
// 最初の不正な入力欄へフォーカスを置くことを確認します。
func TestCreate_FieldErrorSummary(t *testing.T) {
	t.Parallel()
	tests := []struct {
		locale  model.Locale
		heading string
		labels  []string
	}{
		{locale: model.LocaleJa, heading: "入力内容を確認してください", labels: []string{"タイトル", "主言語", "最初の投稿"}},
		{locale: model.LocaleEn, heading: "Check your entries", labels: []string{"Title", "Primary language", "First post"}},
	}
	for _, tt := range tests {
		t.Run(string(tt.locale), func(t *testing.T) {
			t.Parallel()
			db := newBoardDB(t)
			author := newAuthor(t, db, "alice")
			form := url.Values{"title": {strings.Repeat("a", 101)}, "language": {"invalid"}, "body": {"   "}}
			rec := httptest.NewRecorder()
			newHandlerForDB(db).Create(rec, newSubmitRequest(t, "jazz", tt.locale, author, form))
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("ステータスコード = %d", rec.Code)
			}
			markup := rec.Body.String()
			summary := testutil.Element(t, markup, `id="thread-new-errors"`, "</section>")
			if !strings.Contains(summary, tt.heading) {
				t.Errorf("要約に %q が無い", tt.heading)
			}

			// 一覧を包むsectionには名前を付けない。名前を持つsectionはregion
			// ランドマークになり、エラーの一覧がmainやnavと並んでページの主要領域の
			// 一覧に載ってしまう。
			if got := testutil.OpeningTag(t, summary, "section"); got != "<section>" {
				t.Errorf("要約のsection = %s、期待値 = <section> (名前を持つsectionはランドマークになる)", got)
			}
			if strings.Contains(testutil.OpeningTag(t, markup, `id="thread-new-errors"`), "autofocus") {
				t.Error("エラーの要約がautofocusを持っている (フォーカスは最初の不正な入力欄に置くべき)")
			}
			previous := -1
			for i, field := range []string{"title", "language", "body"} {
				link := testutil.Element(t, summary, `href="#`+field+`"`, "</a>")
				if !strings.Contains(link, tt.labels[i]+": ") {
					t.Errorf("リンクにフィールドのラベル %q が無い: %s", tt.labels[i], link)
				}
				position := strings.Index(summary, `href="#`+field+`"`)
				if position <= previous {
					t.Error("要約のリンクがフィールドの順に並んでいない")
				}
				previous = position
				fieldError := testutil.Element(t, markup, `id="`+field+`-error-0"`, "</p>")
				message := fieldError[strings.Index(fieldError, ">")+1:]
				if !strings.Contains(link, message) {
					t.Errorf("要約に %s のエラーが無い: %s", field, message)
				}
				control := testutil.OpeningTag(t, markup, `id="`+field+`"`)
				if !strings.Contains(control, `aria-invalid="true"`) || !strings.Contains(control, field+"-error-0") {
					t.Errorf("入力欄がエラーと関連付けられていない: %s", control)
				}
				if strings.Contains(control, "autofocus") != (field == "title") {
					t.Errorf("フォーカスの位置が誤っている: %s", control)
				}
			}
			if strings.Index(markup, `id="thread-new-errors"`) > strings.Index(markup, `action="/b/jazz/threads"`) {
				t.Error("要約がフォームより前に無い")
			}
		})
	}
}
