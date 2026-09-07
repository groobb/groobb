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

// newAuthor adds an account to db and returns it as the session names it: the
// id the post is attributed to, and the atname the shell shows.
//
// [Ja] newAuthor は db にアカウントを追加し、セッションが名指す形で返します。投稿が
// 帰属する id と、シェルが表示する atname です。
func newAuthor(t *testing.T, db *database.DB, atname string) *model.User {
	t.Helper()

	id := testutil.NewUserBuilder(t, db).WithAtname(atname).WithEmail(atname + "@example.com").Build()
	return &model.User{ID: id, Atname: atname}
}

// validSubmission is a form that passes validation, for the cases whose subject
// is what happens around a well-formed submission rather than the form itself.
//
// [Ja] validSubmission は検証を通るフォームであり、フォームそのものではなく、整った
// 送信の周りで何が起きるかを問うケースのためのものです。
func validSubmission() url.Values {
	return url.Values{
		"title":    {"枯葉の名演"},
		"language": {string(model.LocaleJa.ThreadLanguage())},
		"body":     {"好きな演奏は?"},
	}
}

// newSubmitRequest builds a POST /b/{slug}/threads request as the router would
// hand it to the handler: the form as the body it is submitted in, the slug in
// chi's route context, and the locale, the current path and the author in the
// request context, placed there directly the way i18n's, templates' and the auth
// middleware would.
//
// [Ja] newSubmitRequest は、ルーターがハンドラーへ渡すのと同じ形で
// POST /b/{slug}/threads のリクエストを組み立てます。フォームはそれが送信されるボディと
// して、slug は chi のルート context に、ロケール・現在のパス・書き手はリクエスト context
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

// savedThread is the one thread the database holds, read back as the rows say
// it, so an assertion can speak about what was actually written rather than
// about what the response says was written.
//
// [Ja] savedThread は、データベースが持つ 1 つのスレッドを、行が述べるとおりに読み戻した
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

// boardID reads the id of the board with the given slug, for the rows a case
// arranges directly rather than through the handler under test.
//
// [Ja] boardID は指定 slug の掲示板の id を読みます。テスト対象のハンドラーを介さず、
// ケースが自分で用意する行のためのものです。
func boardID(t *testing.T, db *database.DB, slug string) model.BoardID {
	t.Helper()

	var id model.BoardID
	if err := db.Reader.QueryRowContext(context.Background(), "SELECT id FROM boards WHERE slug = ?", slug).Scan(&id); err != nil {
		t.Fatalf("テスト用掲示板の id の取得に失敗: %v", err)
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

// TestCreate verifies that a well-formed submission starts the thread and sends
// its author to the post they just wrote: HTTP 303 to the new thread's address
// carrying the anchor of the first post, with the thread and that post saved as
// they were submitted.
//
// The post is attributed to the account the session names. The form carries no
// field naming an author, and the case submits one to say that none is read: a
// submission that could say whose post it is could sign it with somebody else's
// name.
//
// [Ja] TestCreate は、整った送信がスレッドを立て、その書き手を今書いた投稿へ送ることを
// 検証します。HTTP 303 で、新しいスレッドのアドレスに最初の投稿のアンカーを付けた先へ送り、
// スレッドとその投稿は送信されたとおりに保存されます。
//
// 投稿はセッションが名指すアカウントに帰属します。フォームは書き手を名指すフィールドを
// 持たず、本ケースはそれが読まれないことを述べるために 1 つ送ります。誰の投稿かを述べうる
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
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusSeeOther)
	}

	saved := readSavedThread(t, db)
	wantLocation := templates.ThreadPath(viewmodel.ThreadID(saved.id)).String() + templates.PostAnchor(1).String()
	if got := rec.Header().Get("Location"); got != wantLocation {
		t.Errorf("Location = %q, want %q", got, wantLocation)
	}
	if saved.title != "枯葉の名演" {
		t.Errorf("保存されたタイトル = %q, want %q", saved.title, "枯葉の名演")
	}
	if want := string(model.LocaleJa.ThreadLanguage()); saved.language != want {
		t.Errorf("保存された主言語 = %q, want %q", saved.language, want)
	}
	if saved.number != 1 {
		t.Errorf("保存されたレス番号 = %d, want 1", saved.number)
	}
	if saved.body != "好きな演奏は?" {
		t.Errorf("保存された本文 = %q, want %q", saved.body, "好きな演奏は?")
	}
	if saved.authorID != author.ID {
		t.Errorf("投稿の作者 = %v, want %v (セッションのアカウント)", saved.authorID, author.ID)
	}
}

// TestCreate_IgnoresQueryStringFields verifies that the fields are read from the
// submitted body alone. A link carrying them in its query string writes nothing:
// followed by a signed-in visitor, such a link would otherwise post in their
// name without them having written anything.
//
// [Ja] TestCreate_IgnoresQueryStringFields は、フィールドが送信されたボディからのみ
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
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	if got := countRows(t, db, "threads"); got != 0 {
		t.Errorf("保存されたスレッド = %d 件, want 0 件", got)
	}
	if body := rec.Body.String(); !strings.Contains(body, "入力してください") {
		t.Error("クエリ文字列だけの送信が、空のフォームとして扱われていない")
	}
}

// TestCreate_ValidationError verifies that a submission with something to fix
// comes back as the form it was written in: HTTP 422, the messages against the
// fields they belong to, and every value the visitor typed still in place, so
// nothing has to be written twice.
//
// The caret lands on the first field with something to fix rather than on the
// top of the document, and text that looks like markup is shown as the text it
// is rather than becoming part of the page.
//
// [Ja] TestCreate_ValidationError は、直すところのある送信が、それが書かれたフォームと
// して返ってくることを検証します。HTTP 422 と、それぞれのフィールドに紐づくメッセージ、
// そして訪問者が打った値がすべてそのまま残り、何も 2 度書かずに済みます。
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
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	if got, want := rec.Header().Get("Cache-Control"), "private, no-store"; got != want {
		t.Errorf("Cache-Control = %q, want %q", got, want)
	}
	if got := countRows(t, db, "threads"); got != 0 {
		t.Errorf("保存されたスレッド = %d 件, want 0 件", got)
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

	// The language is shown back as it was chosen rather than reset to the
	// language the page is drawn in, which would quietly undo the choice.
	//
	// [Ja] 主言語は、ページが描かれている言語に戻すのではなく、選ばれたとおりに見せて
	// 返す。戻せば訪問者の選択を黙って取り消すことになる。
	selected := testutil.OpeningTag(t, body, `value="`+string(model.LocaleEn.ThreadLanguage())+`"`)
	if !strings.Contains(selected, "selected") {
		t.Errorf("送信された主言語の選択肢 = %s, want selected", selected)
	}

	// The body is echoed back inside the textarea, which holds its value as
	// content rather than in an attribute.
	//
	// [Ja] 本文は textarea の中にエコーバックされる。textarea は値を属性ではなく内容と
	// して持つためである。
	if !strings.Contains(testutil.Element(t, body, `id="body"`, "</textarea>"), "   ") {
		t.Error("再描画されたフォームに、送信された本文が残っていない")
	}

	caret := testutil.OpeningTag(t, body, `id="body"`)
	if !strings.Contains(caret, "autofocus") {
		t.Errorf("直すところのある最初の入力欄 = %s, want autofocus", caret)
	}
	if title := testutil.OpeningTag(t, body, `id="title"`); strings.Contains(title, "autofocus") {
		t.Errorf("直すところの無い入力欄 = %s, want no autofocus", title)
	}
}

// TestCreate_TooSoonAfterTheLastPost verifies that a submission arriving before
// the interval between one person's posts has run out is refused with HTTP 429,
// the wait stated both to the browser as Retry-After and on the page, and the
// submission held so it can be sent again once the wait is over.
//
// The refusal is about the submission rather than about any field of it, so the
// summary above the form takes the caret and no input is marked as being at
// fault.
//
// [Ja] TestCreate_TooSoonAfterTheLastPost は、1 人の投稿と投稿の間隔が尽きる前に届いた
// 送信が HTTP 429 で拒否されることを検証します。待ち時間は Retry-After としてブラウザにも
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
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
	retryAfter, err := strconv.Atoi(rec.Header().Get("Retry-After"))
	if err != nil {
		t.Fatalf("Retry-After = %q, want whole seconds", rec.Header().Get("Retry-After"))
	}
	if wait := int(model.PostInterval.Seconds()); retryAfter <= 0 || retryAfter > wait {
		t.Errorf("Retry-After = %d, want 1 以上 %d 以下", retryAfter, wait)
	}
	if got, want := countRows(t, db, "threads"), 1; got != want {
		t.Errorf("スレッド = %d 件, want %d 件 (拒否された送信は何も残さない)", got, want)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "秒後にもう一度お試しください。") {
		t.Error("再描画されたフォームに、待ち時間を述べる文言が含まれていない")
	}
	if !strings.Contains(testutil.OpeningTag(t, body, `id="title"`), `value="枯葉の名演"`) {
		t.Error("拒否された送信のタイトルがフォームに残っていない")
	}

	// The summary takes the caret, so the reason the page came back is where the
	// visitor is rather than somewhere above them.
	//
	// [Ja] 要約がキャレットを取る。ページが戻ってきた理由が、訪問者のいる位置より上の
	// どこかではなく、その位置にあるようにするためである。
	summary := testutil.Element(t, body, `tabindex="-1" autofocus`, "</div>")
	if !strings.Contains(summary, "秒後にもう一度お試しください。") {
		t.Errorf("フォーカスを取る要素 = %s, want the error summary", summary)
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

// TestCreate_WithdrawnAccount verifies that a session whose account has left is
// refused with HTTP 403 and told so on the form, rather than having its post
// attributed to an account that is no longer there.
//
// [Ja] TestCreate_WithdrawnAccount は、アカウントが去ったセッションが HTTP 403 で拒否
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
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if got := countRows(t, db, "posts"); got != 0 {
		t.Errorf("保存された投稿 = %d 件, want 0 件", got)
	}
	if !strings.Contains(rec.Body.String(), "このアカウントでは投稿できません。") {
		t.Error("再描画されたフォームに、投稿できないことを伝える文言が含まれていない")
	}
}

// TestCreate_UnknownBoard verifies that a submission to a slug naming no board
// is answered with the shared 404 page rather than with a form for a board that
// is not there.
//
// [Ja] TestCreate_UnknownBoard は、どの掲示板も指さない slug への送信が、そこに無い
// 掲示板のフォームではなく共通の 404 ページで応答されることを検証します。
func TestCreate_UnknownBoard(t *testing.T) {
	t.Parallel()

	db := newBoardDB(t)
	author := newAuthor(t, db, "alice")

	rec := httptest.NewRecorder()
	newHandlerForDB(db).Create(rec, newSubmitRequest(t, "unknown", model.LocaleJa, author, validSubmission()))

	if rec.Code != http.StatusNotFound {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if !strings.Contains(rec.Body.String(), "ページが見つかりません") {
		t.Error("404 ページの文言が含まれていない")
	}
}

// TestCreate_NonCanonicalSlug verifies that a submission reaching the board
// through the database's case-insensitive collation is saved rather than
// redirected: a redirect would turn the POST into a GET and drop what was
// written. The canonical address arrives with the 303 that follows the save.
//
// [Ja] TestCreate_NonCanonicalSlug は、DB の大文字小文字を無視する照合で掲示板へ到達した
// 送信が、リダイレクトされずに保存されることを検証します。リダイレクトは POST を GET に
// 変え、書かれたものを落としてしまいます。正規のアドレスは、保存に続く 303 で届きます。
func TestCreate_NonCanonicalSlug(t *testing.T) {
	t.Parallel()

	db := newBoardDB(t)
	author := newAuthor(t, db, "alice")

	rec := httptest.NewRecorder()
	newHandlerForDB(db).Create(rec, newSubmitRequest(t, "JAZZ", model.LocaleJa, author, validSubmission()))

	if rec.Code != http.StatusSeeOther {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusSeeOther)
	}

	saved := readSavedThread(t, db)
	if want := templates.ThreadPath(viewmodel.ThreadID(saved.id)).String() + templates.PostAnchor(1).String(); rec.Header().Get("Location") != want {
		t.Errorf("Location = %q, want %q", rec.Header().Get("Location"), want)
	}
}

// TestCreate_SaveFailure verifies that a submission the application could not
// save comes back as the form it was written in (HTTP 500), holding what the
// visitor wrote, so the attempt can be made again without retyping it.
//
// [Ja] TestCreate_SaveFailure は、アプリケーションが保存できなかった送信が、それが
// 書かれたフォームとして (HTTP 500) 返ってくることを検証します。訪問者が書いたものを
// 保つため、打ち直さずにもう一度試せます。
func TestCreate_SaveFailure(t *testing.T) {
	t.Parallel()

	db := newBoardDB(t)
	author := newAuthor(t, db, "alice")
	if err := db.Writer.Close(); err != nil {
		t.Fatalf("Writer の Close() error = %v", err)
	}

	rec := httptest.NewRecorder()
	newHandlerForDB(db).Create(rec, newSubmitRequest(t, "jazz", model.LocaleJa, author, validSubmission()))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusInternalServerError)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "投稿を保存できませんでした。") {
		t.Error("再描画されたフォームに、保存できなかったことを伝える文言が含まれていない")
	}
	if !strings.Contains(testutil.OpeningTag(t, body, `id="title"`), `value="枯葉の名演"`) {
		t.Error("保存できなかった送信のタイトルがフォームに残っていない")
	}
}

// TestCreate_ThroughRouter verifies the route as it is served: behind the body
// limit, the CSRF check and RequireAuth, with the author resolved from the
// session cookie rather than from anything the form carries.
//
// A submission without the token is refused before the handler runs and saves
// nothing, which is what a form posted from another site would be.
//
// [Ja] TestCreate_ThroughRouter は、配信される形でのルートを検証します。ボディの上限・
// CSRF 検証・RequireAuth の背後にあり、書き手はフォームが運ぶ何かではなくセッション
// Cookie から解決されます。
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
		{name: "正常系: CSRF トークンを伴う送信は保存される", withCSRF: true, wantStatus: http.StatusSeeOther, wantSaved: 1},
		{name: "異常系: CSRF トークンの無い送信は拒否される", withCSRF: false, wantStatus: http.StatusForbidden, wantSaved: 0},
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
				t.Errorf("status code = %d, want %d", rec.Code, tt.wantStatus)
			}
			if got := countRows(t, db, "threads"); got != tt.wantSaved {
				t.Errorf("保存されたスレッド = %d 件, want %d 件", got, tt.wantSaved)
			}
			if tt.wantSaved == 0 {
				return
			}

			saved := readSavedThread(t, db)
			if want := templates.ThreadPath(viewmodel.ThreadID(saved.id)).String() + templates.PostAnchor(1).String(); rec.Header().Get("Location") != want {
				t.Errorf("Location = %q, want %q", rec.Header().Get("Location"), want)
			}
			if saved.authorID != author.ID {
				t.Errorf("投稿の作者 = %v, want %v (セッションのアカウント)", saved.authorID, author.ID)
			}
		})
	}
}

// TestCreate_ExpiredSession verifies that a submission made after the session it
// was written under has gone is answered by the way back into an account, and
// that no thread is left behind by it.
//
// The address it arrives at accepts nothing but a submission, so RequireAuth
// sends it to sign-in carrying no return_to: a visitor brought back here would
// meet a POST-only URL with a GET. The title and the first post go with it. What
// holds a refused submission is the form this handler draws, and a submission
// stopped before the handler reaches no form to be drawn back into.
//
// [Ja] TestCreate_ExpiredSession は、それが書かれたときのセッションが失われた後に行われた
// 送信が、アカウントへ戻る道で応答されること、そしてそれによってスレッドが残らないことを
// 検証します。
//
// 送信が届くアドレスは送信しか受け付けないため、RequireAuth は return_to を伴わない
// サインインへ送ります。ここへ連れ戻された訪問者は、POST しか受け付けない URL に GET で
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
	// The token names a session the database no longer holds, which is the state
	// a submission meets when the session it was written under has expired.
	//
	// [Ja] トークンが名指すのは、データベースがもう持っていないセッションである。書かれた
	// ときのセッションが失効した送信が出会う状態がこれにあたる。
	req.AddCookie(&http.Cookie{Name: session.CookieName, Value: "expired-session-token"})
	req = req.WithContext(i18n.SetLocale(req.Context(), model.LocaleJa))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got, want := rec.Header().Get("Location"), templates.SignInPath().String(); got != want {
		t.Errorf("Location = %q, want %q", got, want)
	}
	if got := countRows(t, db, "threads"); got != 0 {
		t.Errorf("保存されたスレッド = %d 件, want 0 件", got)
	}
	if got := countRows(t, db, "posts"); got != 0 {
		t.Errorf("保存された投稿 = %d 件, want 0 件", got)
	}
}

// TestCreate_PreservesBody checks the parsed textarea value, since a raw HTML
// substring assertion cannot detect the parser consuming the first newline.
// The same inputs are saved successfully to check that retries preserve the
// exact text that would have been stored.
//
// [Ja] TestCreate_PreservesBody は HTML 解析後の textarea の値を検証します。HTML の
// 部分文字列の確認では、パーサーによる先頭改行の消失を検出できないためです。同じ入力を
// 正常に保存するケースも確認し、再試行が保存されるはずだった本文を保つことを検証します。
func TestCreate_PreservesBody(t *testing.T) {
	t.Parallel()

	bodies := []struct {
		name  string
		input string
		want  string
	}{
		{name: "no leading newline", input: "  本文\t\n", want: "  本文\t\n"},
		{name: "one leading newline", input: "\n本文\n", want: "\n本文\n"},
		{name: "multiple leading newlines", input: "\n\n本文\n", want: "\n\n本文\n"},
		{name: "CRLF", input: "\r\n本文\r\n", want: "\n本文\n"},
		{name: "mixed line endings", input: "\r\n\r本文\n次\r", want: "\n\n本文\n次\n"},
		{name: "HTML-like text", input: "\n</textarea><script>alert(1)</script>&", want: "\n</textarea><script>alert(1)</script>&"},
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
						t.Fatalf("first status = %d", first.Code)
					}
				case http.StatusInternalServerError:
					if err := db.Writer.Close(); err != nil {
						t.Fatal(err)
					}
				}
				rec := httptest.NewRecorder()
				handler.Create(rec, newSubmitRequest(t, "jazz", model.LocaleJa, author, form))
				if rec.Code != status {
					t.Fatalf("status = %d, want %d", rec.Code, status)
				}
				if status == http.StatusSeeOther {
					if got := readSavedThread(t, db).body; got != body.want {
						t.Errorf("saved body = %q, want %q", got, body.want)
					}
					return
				}
				markup := rec.Body.String()
				if strings.Contains(markup, "\r") {
					t.Error("response contains unnormalized CR")
				}
				if got := parsedTextareaValue(t, markup); got != body.want {
					t.Errorf("textarea value = %q, want %q", got, body.want)
				}
			})
		}
	}
}

// parsedTextareaValue reads the value after HTML parsing, including entity
// decoding and the textarea's initial-newline rule.
//
// [Ja] parsedTextareaValue は HTML 解析後の値を読みます。文字参照の復号と textarea の
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
		t.Fatal("textarea not found")
	}
	return value
}

// TestCreate_FieldErrorSummary keeps all corrections visible together while
// leaving focus on the first invalid control in the form's reading order.
//
// [Ja] TestCreate_FieldErrorSummary は修正箇所をまとめて表示しつつ、フォームの読み順で
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
				t.Fatalf("status = %d", rec.Code)
			}
			markup := rec.Body.String()
			summary := testutil.Element(t, markup, `id="thread-new-errors"`, "</section>")
			if !strings.Contains(summary, tt.heading) {
				t.Errorf("summary missing %q", tt.heading)
			}

			// The section holding the list is left unnamed. A named one becomes a
			// region landmark, which would put the error list in the list of the
			// page's major areas beside main and nav.
			//
			// [Ja] 一覧を包む section には名前を付けない。名前を持つ section は region
			// ランドマークになり、エラーの一覧が main や nav と並んでページの主要領域の
			// 一覧に載ってしまう。
			if got := testutil.OpeningTag(t, summary, "section"); got != "<section>" {
				t.Errorf("summary section = %s, want <section> (a named one is a landmark)", got)
			}
			if strings.Contains(testutil.OpeningTag(t, markup, `id="thread-new-errors"`), "autofocus") {
				t.Error("field errors should focus the first invalid control")
			}
			previous := -1
			for i, field := range []string{"title", "language", "body"} {
				link := testutil.Element(t, summary, `href="#`+field+`"`, "</a>")
				if !strings.Contains(link, tt.labels[i]+": ") {
					t.Errorf("link missing field label %q: %s", tt.labels[i], link)
				}
				position := strings.Index(summary, `href="#`+field+`"`)
				if position <= previous {
					t.Error("summary links are not in field order")
				}
				previous = position
				fieldError := testutil.Element(t, markup, `id="`+field+`-error-0"`, "</p>")
				message := fieldError[strings.Index(fieldError, ">")+1:]
				if !strings.Contains(link, message) {
					t.Errorf("summary is missing %s error: %s", field, message)
				}
				control := testutil.OpeningTag(t, markup, `id="`+field+`"`)
				if !strings.Contains(control, `aria-invalid="true"`) || !strings.Contains(control, field+"-error-0") {
					t.Errorf("control is missing its error association: %s", control)
				}
				if strings.Contains(control, "autofocus") != (field == "title") {
					t.Errorf("incorrect focus: %s", control)
				}
			}
			if strings.Index(markup, `id="thread-new-errors"`) > strings.Index(markup, `action="/b/jazz/threads"`) {
				t.Error("summary is not before the form")
			}
		})
	}
}
