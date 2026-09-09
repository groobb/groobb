package post_test

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
	"github.com/groobb/groobb/go/internal/handler/post"
	"github.com/groobb/groobb/go/internal/handler/thread"
	"github.com/groobb/groobb/go/internal/httperror"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/templates"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/validator"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

const (
	communityName = "ジャズ喫茶"
	appURL        = "https://example.com"
)

// fixture is the community a case submits into: an open thread with one post in
// it, a thread that already holds every post it can hold, and the board both
// stand in.
//
// [Ja] fixture はケースが送信を行うコミュニティです。投稿を 1 つ持つ開いたスレッド、
// 既に持てる投稿をすべて持っているスレッド、そして両者が立っている掲示板です。
type fixture struct {
	db      *database.DB
	board   model.BoardID
	open    model.ThreadID
	quiet   model.ThreadID
	full    model.ThreadID
	handler *post.Handler
}

// newFixture builds the community the cases submit into. The threads' posts are
// written by nobody, so the account a case signs in as has written nothing yet
// and is not held back by the interval between one person's posts.
//
// The board holds a second thread whose last post is more recent than the open
// one's, so a case can say where a reply moves the thread it was written in
// within the board's listing.
//
// [Ja] newFixture は、各ケースが送信を行うコミュニティを組み立てます。スレッドの投稿は
// 誰のものでもないため、ケースがサインインするアカウントはまだ何も書いておらず、1 人の
// 投稿と投稿の間隔に阻まれることがありません。
//
// 掲示板は、最後の投稿が開いたスレッドのそれより新しい 2 つ目のスレッドを持ちます。返信が
// それを書いたスレッドを掲示板の一覧の中でどこへ動かすかを、ケースが述べられるようにする
// ためです。
func newFixture(t *testing.T) fixture {
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
	board, err := boardRepo.Create(ctx, repository.CreateBoardInput{CategoryID: &music.ID, Slug: "jazz", Name: "ジャズ・ファンク"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	open := newThread(t, db, board.ID, "枯葉の名演", 1, time.Now().Add(-2*time.Hour))
	quiet := newThread(t, db, board.ID, "最近買ったレコード", 1, time.Now().Add(-1*time.Hour))
	full := newThread(t, db, board.ID, "埋まったスレッド", model.ThreadPostLimit, time.Now().Add(-48*time.Hour))

	return fixture{db: db, board: board.ID, open: open, quiet: quiet, full: full, handler: newHandlerForDB(db)}
}

// newThread adds a thread holding one post to the board, and states the number
// of posts the thread carries so a case can place it at the cap without writing
// a thousand rows.
//
// [Ja] newThread は投稿を 1 つ持つスレッドを掲示板へ追加し、そのスレッドが持つ投稿の件数を
// 述べます。1000 行を書かずにスレッドを上限に置けるようにするためです。
func newThread(t *testing.T, db *database.DB, boardID model.BoardID, title string, postsCount int, lastPostedAt time.Time) model.ThreadID {
	t.Helper()

	ctx := context.Background()
	threadRepo := repository.NewThreadRepository(db)
	postRepo := repository.NewPostRepository(db)

	thread, err := threadRepo.Create(ctx, repository.CreateThreadInput{
		BoardID:  boardID,
		Title:    title,
		Language: model.LocaleJa.ThreadLanguage(),
	})
	if err != nil {
		t.Fatalf("テスト用スレッドの作成に失敗: %v", err)
	}
	first, err := postRepo.Create(ctx, repository.CreatePostInput{ThreadID: thread.ID, Number: 1, Body: "最初の投稿"})
	if err != nil {
		t.Fatalf("テスト用投稿の作成に失敗: %v", err)
	}
	if err := threadRepo.UpdateLastPost(ctx, thread.ID, repository.UpdateThreadLastPostInput{
		PostsCount:   postsCount,
		LastPostID:   first.ID,
		LastPostedAt: lastPostedAt,
	}); err != nil {
		t.Fatalf("テスト用スレッドの集計の更新に失敗: %v", err)
	}

	return thread.ID
}

// newHandlerForDB builds the post Handler over the supplied application
// database.
//
// [Ja] newHandlerForDB は、渡されたアプリケーションデータベース上に post Handler を
// 構築します。
func newHandlerForDB(db *database.DB) *post.Handler {
	getCommunityNavigationUC := usecase.NewGetCommunityNavigationUsecase(
		repository.NewCommunityRepository(db),
		repository.NewBoardRepository(db),
		repository.NewRoleRepository(db),
	)
	getThreadSummaryUC := usecase.NewGetThreadSummaryUsecase(repository.NewThreadRepository(db))
	createPostUC := usecase.NewCreatePostUsecase(
		db.Writer,
		validator.NewPostCreateValidator(),
		repository.NewThreadRepository(db),
		repository.NewPostRepository(db),
		repository.NewPostReferenceRepository(db),
		repository.NewUserRepository(db),
	)

	cfg := &config.Config{Env: "dev", AppURL: appURL}
	return post.NewHandler(cfg, httperror.NewRenderer(cfg), getCommunityNavigationUC, getThreadSummaryUC, createPostUC)
}

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

// newSubmitRequest builds a POST /t/{id}/posts request as the router would hand
// it to the handler: the form as the body it is submitted in, the id in chi's
// route context, and the locale, the current path and the author in the request
// context, placed there directly the way i18n's, templates' and the auth
// middleware would.
//
// [Ja] newSubmitRequest は、ルーターがハンドラーへ渡すのと同じ形で POST /t/{id}/posts の
// リクエストを組み立てます。フォームはそれが送信されるボディとして、id は chi のルート
// context に、ロケール・現在のパス・書き手はリクエスト context に、i18n・templates・認証の
// 各ミドルウェアがするのと同じように直接置きます。
func newSubmitRequest(t *testing.T, id string, locale model.Locale, author *model.User, form url.Values) *http.Request {
	t.Helper()

	path := "/t/" + id + "/posts"
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	ctx := i18n.SetLocale(req.Context(), locale)
	ctx = templates.SetCurrentPath(ctx, path)
	ctx = viewmodel.SetSiteName(ctx, communityName)
	ctx = middleware.SetUserToContext(ctx, author)

	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", id)
	ctx = context.WithValue(ctx, chi.RouteCtxKey, routeCtx)

	return req.WithContext(ctx)
}

// reply is a submission that passes validation, for the cases whose subject is
// what happens around a well-formed reply rather than the form itself.
//
// [Ja] reply は検証を通る送信であり、フォームそのものではなく、整った返信の周りで何が
// 起きるかを問うケースのためのものです。
func reply(body string) url.Values {
	return url.Values{"body": {body}}
}

// savedPost is a post the database holds, read back as the row says it.
//
// [Ja] savedPost はデータベースが持つ投稿を、行が述べるとおりに読み戻したものです。
type savedPost struct {
	id       model.PostID
	number   int
	body     string
	authorID *model.UserID
}

// readLatestPost reads back the post of the thread with the highest reply
// number, which is the one a submission just added.
//
// [Ja] readLatestPost は、スレッドの中でレス番号が最も大きい投稿、すなわち送信がたった今
// 加えたものを読み戻します。
func readLatestPost(t *testing.T, db *database.DB, threadID model.ThreadID) savedPost {
	t.Helper()

	var saved savedPost
	err := db.Reader.QueryRowContext(context.Background(), `
		SELECT id, number, body, user_id FROM posts WHERE thread_id = ? ORDER BY number DESC LIMIT 1
	`, int64(threadID)).Scan(&saved.id, &saved.number, &saved.body, &saved.authorID)
	if err != nil {
		t.Fatalf("保存された投稿の読み戻しに失敗: %v", err)
	}

	return saved
}

func countPosts(t *testing.T, db *database.DB, threadID model.ThreadID) int {
	t.Helper()

	var count int
	err := db.Reader.QueryRowContext(context.Background(), "SELECT count(*) FROM posts WHERE thread_id = ?", int64(threadID)).Scan(&count)
	if err != nil {
		t.Fatalf("投稿の件数の取得に失敗: %v", err)
	}

	return count
}

// TestCreate verifies that a well-formed reply is added to the thread it was
// written in and its author sent to it: HTTP 303 to the thread carrying the
// anchor of the new post, the post saved under the next reply number, and the
// thread's own view of it — the count, the last post and when it arrived —
// brought up to date with it.
//
// The post is attributed to the account the session names. The form carries no
// field naming an author, and the case submits one to say that none is read: a
// submission that could say whose post it is could sign it with somebody else's
// name.
//
// [Ja] TestCreate は、整った返信が、それが書かれたスレッドへ加えられ、その書き手がそこへ
// 送られることを検証します。HTTP 303 で、新しい投稿のアンカーを付けたスレッドへ送り、投稿は
// 次のレス番号で保存され、スレッド自身が持つその姿 (件数・最後の投稿・それが届いた時刻) も
// それに合わせて更新されます。
//
// 投稿はセッションが名指すアカウントに帰属します。フォームは書き手を名指すフィールドを
// 持たず、本ケースはそれが読まれないことを述べるために 1 つ送ります。誰の投稿かを述べうる
// 送信は、他人の名前で署名できてしまうためです。
func TestCreate(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	author := newAuthor(t, f.db, "alice")
	other := newAuthor(t, f.db, "bob")

	form := reply(">>1 Bill Evans の演奏が好きです")
	form.Set("user_id", other.ID.String())

	rec := httptest.NewRecorder()
	f.handler.Create(rec, newSubmitRequest(t, f.open.String(), model.LocaleJa, author, form))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusSeeOther)
	}

	saved := readLatestPost(t, f.db, f.open)
	wantLocation := templates.ThreadPath(viewmodel.ThreadID(f.open)).String() + templates.PostAnchor(2).String()
	if got := rec.Header().Get("Location"); got != wantLocation {
		t.Errorf("Location = %q, want %q", got, wantLocation)
	}
	if saved.number != 2 {
		t.Errorf("保存されたレス番号 = %d, want 2", saved.number)
	}
	if saved.body != ">>1 Bill Evans の演奏が好きです" {
		t.Errorf("保存された本文 = %q, want %q", saved.body, ">>1 Bill Evans の演奏が好きです")
	}
	if saved.authorID == nil || *saved.authorID != author.ID {
		t.Errorf("投稿の作者 = %v, want %v (セッションのアカウント)", saved.authorID, author.ID)
	}

	thread, err := repository.NewThreadRepository(f.db).FindByID(context.Background(), f.open)
	if err != nil {
		t.Fatalf("スレッドの読み戻しに失敗: %v", err)
	}
	if thread.PostsCount != 2 {
		t.Errorf("スレッドの投稿数 = %d, want 2", thread.PostsCount)
	}
	if thread.LastPostID == nil || *thread.LastPostID != saved.id {
		t.Errorf("スレッドの最終投稿 = %v, want %v", thread.LastPostID, saved.id)
	}
}

// TestCreate_MovesTheThreadToTheTopOfItsBoard verifies that the board's listing
// answers a saved reply: the thread it was written in stands above one whose
// last post is older, where before the reply it stood below it.
//
// [Ja] TestCreate_MovesTheThreadToTheTopOfItsBoard は、保存された返信に掲示板の一覧が
// 応じることを検証します。返信が書かれたスレッドは、最後の投稿がより古いスレッドの上に
// 立ちます。返信の前には、その下に立っていました。
func TestCreate_MovesTheThreadToTheTopOfItsBoard(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	f := newFixture(t)
	author := newAuthor(t, f.db, "alice")
	threadRepo := repository.NewThreadRepository(f.db)

	before, err := threadRepo.ListByBoardID(ctx, f.board)
	if err != nil {
		t.Fatalf("ListByBoardID() error = %v", err)
	}
	if before[0].ID != f.quiet {
		t.Fatalf("返信前の一覧の先頭 = %v, want %v", before[0].ID, f.quiet)
	}

	rec := httptest.NewRecorder()
	f.handler.Create(rec, newSubmitRequest(t, f.open.String(), model.LocaleJa, author, reply("久しぶりに聴き返しました")))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusSeeOther)
	}

	after, err := threadRepo.ListByBoardID(ctx, f.board)
	if err != nil {
		t.Fatalf("ListByBoardID() error = %v", err)
	}
	if after[0].ID != f.open {
		t.Errorf("返信後の一覧の先頭 = %v, want %v (返信のあったスレッド)", after[0].ID, f.open)
	}
}

// TestCreate_IgnoresQueryStringFields verifies that the body is read from the
// submitted request body alone. A link carrying it in its query string writes
// nothing: followed by a signed-in visitor, such a link would otherwise post in
// their name without them having written anything.
//
// [Ja] TestCreate_IgnoresQueryStringFields は、本文が送信されたボディからのみ読まれる
// ことを検証します。クエリ文字列にそれを載せたリンクは何も書きません。そうでなければ、
// サインイン済みの訪問者がそのリンクを踏んだだけで、何も書いていないのにその人の名前で
// 投稿されることになります。
func TestCreate_IgnoresQueryStringFields(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	author := newAuthor(t, f.db, "alice")

	req := newSubmitRequest(t, f.open.String(), model.LocaleJa, author, url.Values{})
	req.URL.RawQuery = reply("クエリ文字列からの投稿").Encode()

	rec := httptest.NewRecorder()
	f.handler.Create(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	if got := countPosts(t, f.db, f.open); got != 1 {
		t.Errorf("スレッドの投稿 = %d 件, want 1 件 (用意した投稿のみ)", got)
	}
	if body := rec.Body.String(); !strings.Contains(body, "入力してください") {
		t.Error("クエリ文字列だけの送信が、空のフォームとして扱われていない")
	}
}

// TestCreate_ValidationError verifies that a reply with something to fix comes
// back on the page it can be corrected on: HTTP 422, the message against the
// field it belongs to, and what was written still in the form, so nothing has to
// be typed twice.
//
// The page names the thread the reply was written in and links to it, because
// the address it answers under says nothing a visitor can read, and text that
// looks like markup is shown as the text it is rather than becoming part of the
// page.
//
// [Ja] TestCreate_ValidationError は、直すところのある返信が、それを直せるページとして
// 返ってくることを検証します。HTTP 422 と、それが属するフィールドに紐づくメッセージ、
// そして書かれたものはフォームに残り、何も 2 度打たずに済みます。
//
// このページは返信が書かれたスレッドを名指してリンクします。応答するアドレスは訪問者が
// 読めることを何も述べないためです。マークアップに見えるテキストは、ページの一部になるので
// はなく、そのままのテキストとして表示されます。
func TestCreate_ValidationError(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	author := newAuthor(t, f.db, "alice")

	rec := httptest.NewRecorder()
	f.handler.Create(rec, newSubmitRequest(t, f.open.String(), model.LocaleJa, author, reply(strings.Repeat("あ", 10001))))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	if got, want := rec.Header().Get("Cache-Control"), "private, no-store"; got != want {
		t.Errorf("Cache-Control = %q, want %q", got, want)
	}
	if got := countPosts(t, f.db, f.open); got != 1 {
		t.Errorf("スレッドの投稿 = %d 件, want 1 件 (用意した投稿のみ)", got)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "本文は10,000文字以内で入力してください") {
		t.Error("再描画されたページに本文のエラーメッセージが含まれていない")
	}
	if !strings.Contains(body, `href="`+templates.ThreadPath(viewmodel.ThreadID(f.open)).String()+`"`) {
		t.Error("再描画されたページに投稿先のスレッドへのリンクが含まれていない")
	}
	if !strings.Contains(body, "枯葉の名演") {
		t.Error("再描画されたページに投稿先のスレッドのタイトルが含まれていない")
	}
	if !strings.Contains(testutil.Element(t, body, `id="body"`, "</textarea>"), strings.Repeat("あ", 10001)) {
		t.Error("再描画されたフォームに、送信された本文が残っていない")
	}
	if !strings.Contains(body, `action="`+templates.ThreadPostsPath(viewmodel.ThreadID(f.open)).String()+`"`) {
		t.Error("再描画されたフォームが、元の投稿先へ送信できる形になっていない")
	}
}

// TestCreate_EscapesTheSubmittedBody verifies that a body which looks like
// markup comes back as the text it is rather than as part of the page.
//
// [Ja] TestCreate_EscapesTheSubmittedBody は、マークアップに見える本文が、ページの一部
// ではなく、そのままのテキストとして返ってくることを検証します。
func TestCreate_EscapesTheSubmittedBody(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	author := newAuthor(t, f.db, "alice")

	rec := httptest.NewRecorder()
	f.handler.Create(rec, newSubmitRequest(t, f.open.String(), model.LocaleJa, author, reply("<script>alert(1)</script>\n"+strings.Repeat("あ", 10001))))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}

	body := rec.Body.String()
	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Error("送信された本文がマークアップとして描画されている")
	}
	if !strings.Contains(body, "&lt;script&gt;") {
		t.Error("再描画されたフォームに、送信された本文がテキストとして残っていない")
	}
}

// TestCreate_TooSoonAfterTheLastPost verifies that a reply arriving before the
// interval between one person's posts has run out is refused with HTTP 429, the
// wait stated both to the browser as Retry-After and on the page, and the reply
// held so it can be sent again once the wait is over.
//
// The refusal is about the submission rather than about the field it carries, so
// the summary above the form takes the caret and the body is not marked as being
// at fault.
//
// [Ja] TestCreate_TooSoonAfterTheLastPost は、1 人の投稿と投稿の間隔が尽きる前に届いた
// 返信が HTTP 429 で拒否されることを検証します。待ち時間は Retry-After としてブラウザにも
// ページ上にも述べられ、返信は、待ち時間が明けたら送り直せるように保たれます。
//
// 拒否の理由は送信そのものであってそれが運ぶフィールドではないため、フォームの上の要約が
// キャレットを取り、本文に落ち度の印は付きません。
func TestCreate_TooSoonAfterTheLastPost(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	author := newAuthor(t, f.db, "alice")

	first := httptest.NewRecorder()
	f.handler.Create(first, newSubmitRequest(t, f.open.String(), model.LocaleJa, author, reply("最初の返信")))
	if first.Code != http.StatusSeeOther {
		t.Fatalf("1 件目の status code = %d, want %d", first.Code, http.StatusSeeOther)
	}

	rec := httptest.NewRecorder()
	f.handler.Create(rec, newSubmitRequest(t, f.quiet.String(), model.LocaleJa, author, reply("続けて書いた返信")))

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
	retryAfter, err := strconv.Atoi(rec.Header().Get("Retry-After"))
	if err != nil {
		t.Fatalf("Retry-After = %q, want whole seconds", rec.Header().Get("Retry-After"))
	}
	if wait := int(model.PostInterval.Seconds()); retryAfter <= 0 || retryAfter > wait {
		t.Errorf("Retry-After = %d, want 1 以上 %d 以下", retryAfter, wait)
	}
	if got := countPosts(t, f.db, f.quiet); got != 1 {
		t.Errorf("スレッドの投稿 = %d 件, want 1 件 (拒否された送信は何も残さない)", got)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "秒後にもう一度お試しください。") {
		t.Error("再描画されたページに、待ち時間を述べる文言が含まれていない")
	}
	if !strings.Contains(testutil.Element(t, body, `id="body"`, "</textarea>"), "続けて書いた返信") {
		t.Error("拒否された送信の本文がフォームに残っていない")
	}

	summary := testutil.Element(t, body, `tabindex="-1" autofocus`, "</div>")
	if !strings.Contains(summary, "秒後にもう一度お試しください。") {
		t.Errorf("フォーカスを取る要素 = %s, want the error summary", summary)
	}
	if control := testutil.OpeningTag(t, body, `id="body"`); strings.Contains(control, "aria-invalid") || strings.Contains(control, "autofocus") {
		t.Errorf("本文の入力欄が不正扱い、またはフォーカスを取っている: %s", control)
	}
}

// TestCreate_LockedThread verifies that a reply to a thread holding every post
// it can hold is refused with HTTP 409, the reason said in place of a message
// about the submission, and the text kept where it can be read and copied out
// rather than in a form that would be refused again.
//
// The thread takes nothing from the submission: the case checks the rows so that
// a post numbered beyond the cap cannot be saved by posting straight at the
// address, without the form that would have been drawn without one.
//
// [Ja] TestCreate_LockedThread は、持てる投稿をすべて持っているスレッドへの返信が
// HTTP 409 で拒否されること、送信についてのメッセージの代わりに理由が述べられること、
// そしてテキストが、もう一度拒否されるフォームの中ではなく、読んで写し取れる場所に
// 保たれることを検証します。
//
// スレッドはその送信から何も受け取りません。本ケースは行を確かめ、上限を越える番号の投稿が、
// フォームを介さずアドレスへ直接送ることでも保存できないことを述べます。
func TestCreate_LockedThread(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	author := newAuthor(t, f.db, "alice")

	rec := httptest.NewRecorder()
	f.handler.Create(rec, newSubmitRequest(t, f.full.String(), model.LocaleJa, author, reply("1001 件目の投稿")))

	if rec.Code != http.StatusConflict {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusConflict)
	}
	if got := countPosts(t, f.db, f.full); got != 1 {
		t.Errorf("スレッドの投稿 = %d 件, want 1 件 (上限に達したスレッドは投稿を受け付けない)", got)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "このスレッドは投稿数の上限 (1000 件) に達しました。") {
		t.Error("ロック中のスレッドへの返信の応答に、上限に達した旨の文言が含まれていない")
	}
	if strings.Contains(body, "投稿を保存できませんでした。") {
		t.Error("ロックの案内の傍らに、保存の失敗を述べる汎用の文言が並んでいる")
	}
	if !strings.Contains(body, "1001 件目の投稿") {
		t.Error("拒否された送信の本文がページに残っていない")
	}
	if !strings.Contains(body, "この本文はコピーできます。") {
		t.Error("本文を控えられることを伝えるヒントが含まれていない")
	}
	if strings.Contains(body, `action="`+templates.ThreadPostsPath(viewmodel.ThreadID(f.full)).String()+`"`) {
		t.Error("ロック中のスレッドへの再送フォームが描かれている")
	}
	if strings.Contains(body, "返信する") {
		t.Error("ロック中のスレッドの応答に、返信を送信するボタンが描かれている")
	}
}

// TestCreate_WithdrawnAccount verifies that a session whose account has left is
// refused with HTTP 403 and told so, rather than having its post attributed to
// an account that is no longer there.
//
// [Ja] TestCreate_WithdrawnAccount は、アカウントが去ったセッションが HTTP 403 で拒否
// され、その旨を伝えられることを検証します。もう存在しないアカウントに投稿が帰属すること
// はありません。
func TestCreate_WithdrawnAccount(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	withdrawn := &model.User{
		ID:     testutil.NewUserBuilder(t, f.db).WithAtname("bob").WithDeletedAt(time.Now().Add(-24 * time.Hour)).Build(),
		Atname: "bob",
	}

	rec := httptest.NewRecorder()
	f.handler.Create(rec, newSubmitRequest(t, f.open.String(), model.LocaleJa, withdrawn, reply("退会後の投稿")))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if got := countPosts(t, f.db, f.open); got != 1 {
		t.Errorf("スレッドの投稿 = %d 件, want 1 件", got)
	}
	if !strings.Contains(rec.Body.String(), "このアカウントでは投稿できません。") {
		t.Error("再描画されたページに、投稿できないことを伝える文言が含まれていない")
	}
}

// TestCreate_UnknownThread verifies that a submission to an address naming no
// thread is answered with the shared 404 page rather than with a page for a
// thread that is not there. A path that cannot spell a thread's id at all is
// answered the same way, without a lookup.
//
// [Ja] TestCreate_UnknownThread は、どのスレッドも名指さないアドレスへの送信が、そこに
// 無いスレッドのページではなく共通の 404 ページで応答されることを検証します。そもそも
// スレッドの id を表しえないパスも、ルックアップを伴わずに同じ応答になります。
func TestCreate_UnknownThread(t *testing.T) {
	t.Parallel()

	for _, id := range []string{"999999", "abc", "0", "-1"} {
		t.Run(id, func(t *testing.T) {
			t.Parallel()

			f := newFixture(t)
			author := newAuthor(t, f.db, "alice")

			rec := httptest.NewRecorder()
			f.handler.Create(rec, newSubmitRequest(t, id, model.LocaleJa, author, reply("どこにも属さない投稿")))

			if rec.Code != http.StatusNotFound {
				t.Errorf("status code = %d, want %d", rec.Code, http.StatusNotFound)
			}
			if !strings.Contains(rec.Body.String(), "ページが見つかりません") {
				t.Error("404 ページの文言が含まれていない")
			}
		})
	}
}

// TestCreate_NonCanonicalID verifies that a reply reaching the thread through a
// spelling of its id the application would not have written is saved rather than
// redirected: a redirect would turn the POST into a GET and drop what was
// written. The canonical address arrives with the 303 that follows the save.
//
// [Ja] TestCreate_NonCanonicalID は、アプリケーションが書かない綴りの id でスレッドへ
// 到達した返信が、リダイレクトされずに保存されることを検証します。リダイレクトは POST を
// GET に変え、書かれたものを落としてしまいます。正規のアドレスは、保存に続く 303 で
// 届きます。
func TestCreate_NonCanonicalID(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	author := newAuthor(t, f.db, "alice")

	rec := httptest.NewRecorder()
	f.handler.Create(rec, newSubmitRequest(t, "0"+f.open.String(), model.LocaleJa, author, reply("先頭にゼロの付いたアドレスからの返信")))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	want := templates.ThreadPath(viewmodel.ThreadID(f.open)).String() + templates.PostAnchor(2).String()
	if got := rec.Header().Get("Location"); got != want {
		t.Errorf("Location = %q, want %q", got, want)
	}
	if got := countPosts(t, f.db, f.open); got != 2 {
		t.Errorf("スレッドの投稿 = %d 件, want 2 件", got)
	}
}

// TestCreate_SaveFailure verifies that a reply the application could not save
// comes back in the form it was written in (HTTP 500), holding what the visitor
// wrote, so the attempt can be made again without retyping it.
//
// [Ja] TestCreate_SaveFailure は、アプリケーションが保存できなかった返信が、それが書かれた
// フォームとして (HTTP 500) 返ってくることを検証します。訪問者が書いたものを保つため、
// 打ち直さずにもう一度試せます。
func TestCreate_SaveFailure(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	author := newAuthor(t, f.db, "alice")
	if err := f.db.Writer.Close(); err != nil {
		t.Fatalf("Writer の Close() error = %v", err)
	}

	rec := httptest.NewRecorder()
	f.handler.Create(rec, newSubmitRequest(t, f.open.String(), model.LocaleJa, author, reply("保存できない返信")))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusInternalServerError)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "投稿を保存できませんでした。") {
		t.Error("再描画されたページに、保存できなかったことを伝える文言が含まれていない")
	}
	if !strings.Contains(testutil.Element(t, body, `id="body"`, "</textarea>"), "保存できない返信") {
		t.Error("保存できなかった送信の本文がフォームに残っていない")
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
		wantPosts  int
	}{
		{name: "正常系: CSRF トークンを伴う送信は保存される", withCSRF: true, wantStatus: http.StatusSeeOther, wantPosts: 2},
		{name: "異常系: CSRF トークンの無い送信は拒否される", withCSRF: false, wantStatus: http.StatusForbidden, wantPosts: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := newFixture(t)
			author := newAuthor(t, f.db, "alice")
			sessionToken := "session-token-" + strconv.Itoa(tt.wantPosts)
			testutil.NewUserSessionBuilder(t, f.db).WithUserID(author.ID).WithToken(sessionToken).Build()

			cfg := &config.Config{Env: "test"}
			auth := middleware.NewAuth(session.NewManager(repository.NewUserRepository(f.db), cfg))
			router := chi.NewRouter()
			router.Use(middleware.PostFormLimit)
			router.Use(middleware.NewCSRF(cfg).Middleware)
			router.With(auth.RequireAuth).Post("/t/{id}/posts", f.handler.Create)

			form := reply("ルーター越しの返信")
			if tt.withCSRF {
				form.Set("csrf_token", csrfToken)
			}

			path := templates.ThreadPostsPath(viewmodel.ThreadID(f.open)).String()
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
			if got := countPosts(t, f.db, f.open); got != tt.wantPosts {
				t.Errorf("スレッドの投稿 = %d 件, want %d 件", got, tt.wantPosts)
			}
			if tt.wantPosts == 1 {
				return
			}

			saved := readLatestPost(t, f.db, f.open)
			want := templates.ThreadPath(viewmodel.ThreadID(f.open)).String() + templates.PostAnchor(saved.number).String()
			if got := rec.Header().Get("Location"); got != want {
				t.Errorf("Location = %q, want %q", got, want)
			}
			if saved.authorID == nil || *saved.authorID != author.ID {
				t.Errorf("投稿の作者 = %v, want %v (セッションのアカウント)", saved.authorID, author.ID)
			}
		})
	}
}

// TestCreate_ExpiredSession verifies that a reply submitted after the session it
// was written under has gone is answered by the way back into an account, and
// that the thread it was written in is left as it was.
//
// The address it arrives at accepts nothing but a submission, so RequireAuth
// sends it to sign-in carrying no return_to: a visitor brought back here would
// meet a POST-only URL with a GET. What was written goes with it. A refused
// reply is held by the page this handler draws, and a submission stopped before
// the handler reaches no page to be drawn back on.
//
// [Ja] TestCreate_ExpiredSession は、それが書かれたときのセッションが失われた後に送られた
// 返信が、アカウントへ戻る道で応答されること、そして書かれた先のスレッドがそのまま残ることを
// 検証します。
//
// 送信が届くアドレスは送信しか受け付けないため、RequireAuth は return_to を伴わない
// サインインへ送ります。ここへ連れ戻された訪問者は、POST しか受け付けない URL に GET で
// 辿り着くことになるためです。書かれたものはそれと共に失われます。拒否された返信を保つのは
// このハンドラーが描くページであり、ハンドラーの手前で止まった送信には、それを戻して置く
// ページがありません。
func TestCreate_ExpiredSession(t *testing.T) {
	t.Parallel()

	const csrfToken = "test-csrf-token"

	f := newFixture(t)
	cfg := &config.Config{Env: "test"}
	auth := middleware.NewAuth(session.NewManager(repository.NewUserRepository(f.db), cfg))
	router := chi.NewRouter()
	router.Use(middleware.PostFormLimit)
	router.Use(middleware.NewCSRF(cfg).Middleware)
	router.With(auth.RequireAuth).Post("/t/{id}/posts", f.handler.Create)

	form := reply("セッションが切れた後に送った返信")
	form.Set("csrf_token", csrfToken)

	path := templates.ThreadPostsPath(viewmodel.ThreadID(f.open)).String()
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
	if got := countPosts(t, f.db, f.open); got != 1 {
		t.Errorf("スレッドの投稿 = %d 件, want 1 件", got)
	}
}

// TestCreate_ThreadFillsAfterFormIsOpened connects the form read before the cap
// to the refusal after another author fills it. Real post rows keep the GET,
// reply numbers and final database state consistent throughout the flow.
//
// [Ja] TestCreate_ThreadFillsAfterFormIsOpened は、上限到達前に取得したフォームと、別の
// 投稿者が上限に到達させた後の拒否を接続します。実際の投稿行を用意することで、GET・
// レス番号・最終的な DB の状態をフロー全体で一致させます。
func TestCreate_ThreadFillsAfterFormIsOpened(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ctx := context.Background()
	// Existing posts have no author, so neither visitor starts inside the
	// posting interval. Bulk insertion avoids hundreds of setup transactions.
	//
	// [Ja] 既存投稿は作者なしにし、どちらの訪問者も連投制限中にならないようにする。
	// 一括挿入により、準備のための数百回のトランザクションを避ける。
	if _, err := f.db.Writer.ExecContext(ctx, `
		WITH RECURSIVE numbers(number) AS (
			SELECT 2 UNION ALL SELECT number + 1 FROM numbers WHERE number < ?
		)
		INSERT INTO posts (thread_id, number, body)
		SELECT ?, number, '既存の投稿' FROM numbers
	`, model.ThreadPostLimit-1, int64(f.open)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Writer.ExecContext(ctx, `
		UPDATE threads SET posts_count = ?,
			last_post_id = (SELECT id FROM posts WHERE thread_id = ? ORDER BY number DESC LIMIT 1),
			last_posted_at = (SELECT created_at FROM posts WHERE thread_id = ? ORDER BY number DESC LIMIT 1)
		WHERE id = ?
	`, model.ThreadPostLimit-1, int64(f.open), int64(f.open), int64(f.open)); err != nil {
		t.Fatal(err)
	}
	if got := countPosts(t, f.db, f.open); got != model.ThreadPostLimit-1 {
		t.Fatalf("準備後の投稿数 = %d, want %d", got, model.ThreadPostLimit-1)
	}

	for _, name := range []string{"alice", "bob"} {
		author := newAuthor(t, f.db, name)
		testutil.NewUserSessionBuilder(t, f.db).WithUserID(author.ID).WithToken(name + "-session").Build()
	}
	cfg := &config.Config{Env: "test", AppURL: appURL}
	threadRepo := repository.NewThreadRepository(f.db)
	boardRepo := repository.NewBoardRepository(f.db)
	categoryRepo := repository.NewCategoryRepository(f.db)
	userRepo := repository.NewUserRepository(f.db)
	threadHandler := thread.NewHandler(cfg, httperror.NewRenderer(cfg),
		usecase.NewGetCommunityNavigationUsecase(repository.NewCommunityRepository(f.db), boardRepo, repository.NewRoleRepository(f.db)),
		usecase.NewGetBoardUsecase(boardRepo, categoryRepo),
		usecase.NewGetThreadUsecase(threadRepo, boardRepo, categoryRepo,
			repository.NewPostRepository(f.db), repository.NewPostReferenceRepository(f.db), userRepo),
		usecase.NewGetBoardThreadsUsecase(threadRepo), nil,
	)
	auth := middleware.NewAuth(session.NewManager(userRepo, cfg))
	router := chi.NewRouter()
	router.Use(middleware.PostFormLimit)
	router.Use(middleware.NewCSRF(cfg).Middleware)
	router.With(auth.SetUser).Get("/t/{id}", threadHandler.Show)
	router.With(auth.RequireAuth).Post("/t/{id}/posts", f.handler.Create)

	threadPath := templates.ThreadPath(viewmodel.ThreadID(f.open)).String()
	postPath := templates.ThreadPostsPath(viewmodel.ThreadID(f.open)).String()
	request := func(name, method, path string, form url.Values, csrf *http.Cookie) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{Name: session.CookieName, Value: name + "-session"})
		if csrf != nil {
			req.AddCookie(csrf)
		}
		req = req.WithContext(i18n.SetLocale(req.Context(), model.LocaleJa))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	openForm := func(name string) (url.Values, *http.Cookie) {
		t.Helper()
		rec := request(name, http.MethodGet, threadPath, nil, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("フォーム取得の status = %d, want 200", rec.Code)
		}
		form := replyFormValues(t, rec.Body.String(), postPath)
		for _, cookie := range rec.Result().Cookies() {
			if cookie.Name == middleware.CSRFCookieName {
				if form.Get("csrf_token") == "" || form.Get("csrf_token") != cookie.Value {
					t.Fatal("返信フォームの CSRF トークンが Cookie と一致しない")
				}
				return form, cookie
			}
		}
		t.Fatal("GET 応答に CSRF Cookie がない")
		return nil, nil
	}

	aliceForm, aliceCSRF := openForm("alice")
	bobForm, bobCSRF := openForm("bob")
	bobForm.Set("body", "最後の投稿")
	last := request("bob", http.MethodPost, postPath, bobForm, bobCSRF)
	if last.Code != http.StatusSeeOther {
		t.Fatalf("1000 番の status = %d, want 303", last.Code)
	}
	if got, want := last.Header().Get("Location"), threadPath+"#p1000"; got != want {
		t.Errorf("Location = %q, want %q", got, want)
	}
	shown := request("bob", http.MethodGet, threadPath, nil, bobCSRF)
	if shown.Code != http.StatusOK || !strings.Contains(shown.Body.String(), `id="p1000"`) {
		t.Fatal("投稿後の閲覧ページに 1000 番のアンカーがない")
	}

	const keptBody = "\n書きかけの返信 <続き> & 本文\n"
	aliceForm.Set("body", keptBody)
	refused := request("alice", http.MethodPost, postPath, aliceForm, aliceCSRF)
	if refused.Code != http.StatusConflict {
		t.Fatalf("取得済みフォーム送信の status = %d, want 409", refused.Code)
	}
	markup := refused.Body.String()
	if !strings.Contains(markup, "このスレッドは投稿数の上限 (1000 件) に達しました。") {
		t.Error("上限到達の理由が表示されていない")
	}
	if strings.Contains(markup, `action="`+postPath+`"`) {
		t.Error("ロック中のスレッドへの再送フォームがある")
	}
	control := testutil.OpeningTag(t, markup, `id="body"`)
	if !strings.Contains(control, "readonly") || strings.Contains(control, "aria-invalid") {
		t.Errorf("保持された本文の属性が不正: %s", control)
	}
	root, err := html.Parse(strings.NewReader(markup))
	if err != nil {
		t.Fatal(err)
	}
	var kept string
	var visit func(*html.Node)
	visit = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "textarea" {
			for child := n.FirstChild; child != nil; child = child.NextSibling {
				if child.Type == html.TextNode {
					kept += child.Data
				}
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(root)
	if kept != keptBody {
		t.Errorf("保持された本文 = %q, want %q", kept, keptBody)
	}

	direct := reply("フォームを介さない追加投稿")
	direct.Set("csrf_token", aliceCSRF.Value)
	if rec := request("alice", http.MethodPost, postPath, direct, aliceCSRF); rec.Code != http.StatusConflict {
		t.Errorf("直接 POST の status = %d, want 409", rec.Code)
	}
	if got := countPosts(t, f.db, f.open); got != model.ThreadPostLimit {
		t.Errorf("拒否後の投稿数 = %d, want %d", got, model.ThreadPostLimit)
	}
	if saved := readLatestPost(t, f.db, f.open); saved.number != model.ThreadPostLimit || saved.body != "最後の投稿" {
		t.Errorf("拒否後の最終投稿 = %+v, want 1000 番の成功した投稿", saved)
	}
	stored, err := threadRepo.FindByID(ctx, f.open)
	if err != nil {
		t.Fatal(err)
	}
	if stored.PostsCount != model.ThreadPostLimit {
		t.Errorf("拒否後の集計 = %d, want %d", stored.PostsCount, model.ThreadPostLimit)
	}
}

// replyFormValues extracts hidden fields from the actual reply form, excluding
// other forms such as sign-out. A disabled submit button cannot start the flow.
//
// [Ja] replyFormValues は実際の返信フォームから hidden フィールドを取り出し、サインアウト
// など他のフォームを除外します。無効な送信ボタンではフローを開始できません。
func replyFormValues(t *testing.T, markup, action string) url.Values {
	t.Helper()
	z := html.NewTokenizer(strings.NewReader(markup))
	inside := false
	values := url.Values{}
	for {
		switch z.Next() {
		case html.ErrorToken:
			t.Fatal("送信可能な返信フォームがない")
		case html.StartTagToken, html.SelfClosingTagToken:
			token := z.Token()
			attrs := map[string]string{}
			for _, attr := range token.Attr {
				attrs[attr.Key] = attr.Val
			}
			if token.Data == "form" {
				inside = attrs["action"] == action && strings.EqualFold(attrs["method"], "POST")
			}
			if !inside {
				continue
			}
			if token.Data == "input" && attrs["type"] == "hidden" {
				values.Set(attrs["name"], attrs["value"])
			}
			if token.Data == "button" && attrs["type"] == "submit" {
				if _, disabled := attrs["disabled"]; disabled {
					t.Fatal("返信フォームの送信ボタンが無効")
				}
				return values
			}
		case html.EndTagToken:
			if z.Token().Data == "form" {
				inside = false
			}
		}
	}
}
