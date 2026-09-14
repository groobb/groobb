package post_unpublication_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/handler/post_unpublication"
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

// csrfToken is the token every submission in this package carries in both the
// cookie and the body, which is what the CSRF check compares. Its value says
// nothing; that the two sides agree is the whole of it.
//
// [Ja] csrfTokenは、本パッケージのどの送信もCookieとボディの両方で運ぶトークンで、CSRFの
// 検証が突き合わせる相手です。値そのものに意味は無く、両者が一致していることがすべてです。
const csrfToken = "test-csrf-token"

// postBody is what the fixture's post says. A test states that the confirmation
// page shows it, since what the administrator is deciding about is the text.
//
// [Ja] postBodyはフィクスチャの投稿が述べていることです。確認ページがそれを示すことをテストが
// 述べます。管理者が判断しようとしている対象がそのテキストであるためです。
const postBody = "好きな演奏は?"

// fixture is a test database with the unpublication handler wired over its
// repositories, together with the thread the post stands in, the post the
// requests act on, the account that wrote it, and the administrator acting, so a
// test drives both routes against rows that are really stored.
//
// [Ja] fixtureは、そのリポジトリ上に非公開のハンドラーを組み立てたテスト用データベースと、
// 投稿が立っているスレッド、リクエストが働きかける投稿、それを書いたアカウント、そして操作する
// 管理者です。テストが、実際に保存された行に対して2つのルートを駆動できるようにするためです。
type fixture struct {
	db      *database.DB
	cfg     *config.Config
	handler *post_unpublication.Handler
	thread  *model.Thread
	post    *model.Post
	starter model.UserID
	admin   model.UserID
	plain   model.UserID
}

// newFixture builds the fixture for one test: a board with one thread in it
// holding its first post, an administrator, and an account holding no role at
// all.
//
// [Ja] newFixtureは1つのテストのためのfixtureを組み立てます。最初の投稿を持つスレッドが1つ
// 立っている掲示板、管理者、そしてロールを1つも持たないアカウントです。
func newFixture(t *testing.T) *fixture {
	t.Helper()

	ctx := context.Background()
	db := testutil.SetupDB(t)
	cfg := &config.Config{Env: "test"}

	boardRepo := repository.NewBoardRepository(db)
	threadRepo := repository.NewThreadRepository(db)
	postRepo := repository.NewPostRepository(db)
	roleRepo := repository.NewRoleRepository(db)
	userRepo := repository.NewUserRepository(db)
	moderationLogRepo := repository.NewModerationLogRepository(db)

	board, err := boardRepo.Create(ctx, repository.CreateBoardInput{Slug: "jazz", Name: "ジャズ"})
	if err != nil {
		t.Fatalf("テスト用掲示板の作成に失敗: %v", err)
	}

	starter := testutil.NewUserBuilder(t, db).WithAtname("starter").Build()
	thread, err := threadRepo.Create(ctx, repository.CreateThreadInput{
		BoardID:  board.ID,
		UserID:   &starter,
		Title:    "枯葉の名演",
		Language: model.LocaleJa.ThreadLanguage(),
	})
	if err != nil {
		t.Fatalf("テスト用スレッドの作成に失敗: %v", err)
	}
	post, err := postRepo.Create(ctx, repository.CreatePostInput{
		ThreadID: thread.ID,
		UserID:   &starter,
		Number:   1,
		Body:     postBody,
	})
	if err != nil {
		t.Fatalf("テスト用投稿の作成に失敗: %v", err)
	}

	handler := post_unpublication.NewHandler(
		cfg,
		httperror.NewRenderer(cfg),
		session.NewFlashManager(cfg),
		usecase.NewGetThreadModerationUsecase(roleRepo, threadRepo, postRepo, userRepo),
		usecase.NewUnpublishPostUsecase(
			db.Writer,
			validator.NewModerationLogCreateValidator(),
			roleRepo,
			threadRepo,
			postRepo,
			moderationLogRepo,
		),
	)

	return &fixture{
		db:      db,
		cfg:     cfg,
		handler: handler,
		thread:  thread,
		post:    post,
		starter: starter,
		admin:   seedAdmin(t, db),
		plain:   testutil.NewUserBuilder(t, db).WithAtname("plainuser").Build(),
	}
}

// seedAdmin creates an account holding the built-in administrator role, which is
// what admits every operation on a thread.
//
// [Ja] seedAdminは、組み込みの管理者ロールを持つアカウントを作ります。スレッドに対する
// あらゆる操作を許すのがこれです。
func seedAdmin(t *testing.T, db *database.DB) model.UserID {
	t.Helper()

	userID := testutil.NewUserBuilder(t, db).WithAtname("adminuser").Build()
	testutil.NewUserRoleBuilder(t, db).WithUserID(userID).Build()
	return userID
}

// newRouter mounts the two routes the way serve.go does — behind the CSRF check
// — with actor standing in for the account RequireAuth resolves from a session.
// Going through a router is what lets a test submit as a form does, and what
// makes the pair naming the post reach the handler.
//
// [Ja] newRouterは、serve.goと同じ形で、2つのルートをCSRFの検証の背後に置きます。actorは
// RequireAuthがセッションから解決するアカウントの代わりです。ルーターを通すことで、テストは
// フォームと同じ形で送信でき、投稿を名指す組がハンドラーへ届きます。
func newRouter(f *fixture, actor model.UserID) http.Handler {
	signedIn := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := middleware.SetUserToContext(r.Context(), &model.User{ID: actor, Atname: "actor"})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
	return mount(f, signedIn)
}

// newAnonymousRouter mounts the same two routes behind the real RequireAuth over
// a session manager reading this database, which is how a request carrying no
// session is answered the way it is in production.
//
// [Ja] newAnonymousRouterは、同じ2つのルートを、このデータベースを読むセッションマネージャ
// 上の本物のRequireAuthの背後に置きます。セッションを運ばないリクエストが、本番と同じ形で
// 応答されるようにするためです。
func newAnonymousRouter(f *fixture) http.Handler {
	auth := middleware.NewAuth(session.NewManager(repository.NewUserRepository(f.db), f.cfg))
	return mount(f, auth.RequireAuth)
}

// mount wires the router the two constructors above share, differing only in
// what stands in for the sign-in check.
//
// [Ja] mountは、上の2つのコンストラクタが共有するルーターを組み立てます。違うのは、
// サインインの検査の位置に何が立つかだけです。
func mount(f *fixture, auth func(http.Handler) http.Handler) http.Handler {
	router := chi.NewRouter()
	router.Use(middleware.PostFormLimit)
	router.Use(middleware.NewCSRF(f.cfg).Middleware)

	router.With(auth).Get("/t/{id}/posts/{number}/unpublication/new", f.handler.New)
	router.With(auth).Post("/t/{id}/posts/{number}/unpublication", f.handler.Create)
	return router
}

// unpublicationPath is the address of the post's unpublication, which the
// submission targets.
//
// [Ja] unpublicationPathは投稿の非公開のアドレスで、送信の宛先です。
func unpublicationPath(id model.ThreadID, number int) string {
	return templates.PostUnpublicationPath(viewmodel.ThreadID(id), number).String()
}

// get reads a page through the chain, in Japanese, as a signed-in visitor's
// browser does.
//
// [Ja] getは、サインイン済みの訪問者のブラウザと同じ形で、日本語でチェーン越しにページを
// 読みます。
func get(router http.Handler, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req = req.WithContext(i18n.SetLocale(req.Context(), model.LocaleJa))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// submit sends the confirmation page's form to path as a browser does: a POST
// carrying the urlencoded body, with the CSRF cookie alongside it when
// withCSRFCookie is set.
//
// [Ja] submitは、確認ページのフォームをブラウザと同じ形でpathへ送ります。すなわち
// urlencodedのボディを運ぶPOSTで、withCSRFCookieのときはCSRF Cookieを添えます。
func submit(router http.Handler, path string, reason string, withCSRFCookie bool) *httptest.ResponseRecorder {
	form := url.Values{"csrf_token": {csrfToken}, "reason": {reason}}

	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if withCSRFCookie {
		req.AddCookie(&http.Cookie{Name: middleware.CSRFCookieName, Value: csrfToken})
	}
	req = req.WithContext(i18n.SetLocale(req.Context(), model.LocaleJa))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// findPost reads the post back for an assertion about the state it was left in.
//
// [Ja] findPostは、どの状態で残されたかを問う検証のために投稿を読み戻します。
func findPost(t *testing.T, f *fixture) *model.Post {
	t.Helper()

	post, err := repository.NewPostRepository(f.db).FindByThreadIDAndNumber(context.Background(), f.thread.ID, f.post.Number)
	if err != nil {
		t.Fatalf("FindByThreadIDAndNumber() error = %v", err)
	}
	if post == nil {
		t.Fatalf("投稿を引けない: thread_id=%s number=%d", f.thread.ID, f.post.Number)
	}
	return post
}

// unpublishPost marks the fixture's post unpublished, which is the state the
// confirmation page has nothing to show in.
//
// [Ja] unpublishPostはフィクスチャの投稿に非公開の印を付けます。確認ページに示すものが無い
// 状態です。
func unpublishPost(t *testing.T, f *fixture) {
	t.Helper()

	if err := repository.NewPostRepository(f.db).Unpublish(context.Background(), f.post.ID); err != nil {
		t.Fatalf("テスト用投稿の非公開に失敗: %v", err)
	}
}

// unpublishThread marks the fixture's thread unpublished, which takes the post
// out of view along with everything else under it.
//
// [Ja] unpublishThreadはフィクスチャのスレッドに非公開の印を付けます。その下のすべてと
// ともに投稿も視界から外れます。
func unpublishThread(t *testing.T, f *fixture) {
	t.Helper()

	if err := repository.NewThreadRepository(f.db).Unpublish(context.Background(), f.thread.ID); err != nil {
		t.Fatalf("テスト用スレッドの非公開に失敗: %v", err)
	}
}

// decodeFlash reads the flash the response set, which is where an operation that
// redirected says what it did.
//
// [Ja] decodeFlashは、レスポンスが設定したフラッシュを読みます。リダイレクトした操作が、
// 自身の行ったことを述べる場所がそこです。
func decodeFlash(t *testing.T, rec *httptest.ResponseRecorder) *session.FlashMessage {
	t.Helper()

	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name != session.FlashCookieName {
			continue
		}
		data, err := base64.StdEncoding.DecodeString(cookie.Value)
		if err != nil {
			t.Fatalf("フラッシュCookieのbase64デコードに失敗: %v", err)
		}
		var flash session.FlashMessage
		if err := json.Unmarshal(data, &flash); err != nil {
			t.Fatalf("フラッシュCookieのJSONデコードに失敗: %v", err)
		}
		return &flash
	}

	t.Fatal("フラッシュCookieが設定されていない")
	return nil
}

// TestNew verifies that an administrator opening the confirmation page is shown
// what unpublishing does, the post it is about — its reply number, its author
// and its body — the note field, and a form submitting to that post's own
// unpublication, that the note field does not take focus so the post is read
// before the note is written, and that the page stays out of search indexes.
//
// [Ja] TestNewは、確認ページを開いた管理者に、非公開が何をするのか、対象の投稿 (レス番号・
// 作者・本文)、注記の入力欄、そしてその投稿自身の非公開へ送信するフォームが示されること、
// 注記の入力欄が焦点を取らず、注記が書かれる前に投稿が読まれること、そしてこのページが検索
// インデックスの外に留まることを検証します。
func TestNew(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	rec := get(newRouter(f, f.admin), unpublicationPath(f.thread.ID, f.post.Number)+"/new")

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	wants := []string{
		"投稿を非公開にする",
		"非公開にすると、この投稿の本文と作者はスレッドに表示されなくなり",
		"@starter",
		postBody,
		`action="` + unpublicationPath(f.thread.ID, f.post.Number) + `"`,
		`name="csrf_token"`,
		`name="reason"`,
		"理由 (任意)",
		"操作履歴にだけ記録されます。",
		`href="` + templates.ThreadPostAnchorPath(viewmodel.ThreadID(f.thread.ID), f.post.Number).String() + `"`,
		`<meta name="robots" content="noindex"`,
	}
	for _, want := range wants {
		if !strings.Contains(body, want) {
			t.Errorf("確認ページに %q が含まれていない", want)
		}
	}
	if strings.Contains(body, "autofocus") {
		t.Error("初回の確認ページの理由欄にautofocusが付いている")
	}
}

// TestNew_WithdrawnAuthor verifies that a post whose author has withdrawn is
// still named on the confirmation page, with the absence of an account drawn in
// place of the name.
//
// [Ja] TestNew_WithdrawnAuthorは、作者が退会した投稿も確認ページで名指されること、そして
// 名前の位置にアカウントの不在が描かれることを検証します。
func TestNew_WithdrawnAuthor(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	userRepo := repository.NewUserRepository(f.db)
	if err := userRepo.SoftDeleteAndAnonymize(context.Background(), f.starter, "deleted@example.com", "deleted1"); err != nil {
		t.Fatalf("テスト用利用者の退会に失敗: %v", err)
	}

	rec := get(newRouter(f, f.admin), unpublicationPath(f.thread.ID, f.post.Number)+"/new")

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "退会した利用者") {
		t.Error("確認ページに退会した作者の文言が含まれていない")
	}
	if !strings.Contains(body, postBody) {
		t.Error("確認ページに投稿の本文が含まれていない")
	}
}

// TestNew_WithoutPermission verifies that an account admitted to none of the
// operations on a thread is answered with the 403 page rather than being shown
// the screen that leads to them.
//
// [Ja] TestNew_WithoutPermissionは、スレッドに対するどの操作も許されていないアカウントが、
// そこへ至る画面を見せられるのではなく403ページで応答されることを検証します。
func TestNew_WithoutPermission(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	rec := get(newRouter(f, f.plain), unpublicationPath(f.thread.ID, f.post.Number)+"/new")

	if rec.Code != http.StatusForbidden {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

// TestNew_UnpublishedThread verifies that the confirmation page for a post whose
// thread the community no longer shows is answered with 404: the thread's own
// mark already took the post out of view.
//
// [Ja] TestNew_UnpublishedThreadは、コミュニティがもう示していないスレッドの投稿の確認
// ページが404で応答されることを検証します。スレッド自身の印が既に投稿を視界から外している
// ためです。
func TestNew_UnpublishedThread(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	unpublishThread(t, f)

	rec := get(newRouter(f, f.admin), unpublicationPath(f.thread.ID, f.post.Number)+"/new")

	if rec.Code != http.StatusNotFound {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

// TestNew_MissingPost verifies that an address naming no post the thread still
// shows is answered with the 404 page: a number the thread never issued, a post
// already out of view, and a number that is not a number at all are alike in
// having nothing to put in front of the administrator.
//
// [Ja] TestNew_MissingPostは、スレッドがまだ示している投稿をどれも名指していないアドレスが
// 404ページで応答されることを検証します。スレッドが一度も発行していない番号も、既に視界の外に
// ある投稿も、そもそも数でない番号も、管理者の前に置くものが無い点では同じです。
func TestNew_MissingPost(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	router := newRouter(f, f.admin)

	paths := []string{
		unpublicationPath(f.thread.ID, f.post.Number+1000) + "/new",
		"/t/" + f.thread.ID.String() + "/posts/abc/unpublication/new",
		"/t/abc/posts/1/unpublication/new",
	}
	for _, path := range paths {
		if rec := get(router, path); rec.Code != http.StatusNotFound {
			t.Errorf("%s の status code = %d, want %d", path, rec.Code, http.StatusNotFound)
		}
	}

	unpublishPost(t, f)
	path := unpublicationPath(f.thread.ID, f.post.Number) + "/new"
	if rec := get(router, path); rec.Code != http.StatusNotFound {
		t.Errorf("非公開の投稿の status code = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

// TestNew_SignedOut verifies that a visitor with no session is sent to the
// sign-in form, carrying where they were headed, rather than being told whether
// the post is there.
//
// [Ja] TestNew_SignedOutは、セッションを持たない訪問者が、投稿の有無を告げられるのではなく、
// 向かっていた先を運んでサインインフォームへ送られることを検証します。
func TestNew_SignedOut(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	path := unpublicationPath(f.thread.ID, f.post.Number) + "/new"

	rec := get(newAnonymousRouter(f), path)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	want := templates.SignInPath().WithReturnTo(path).String()
	if got := rec.Header().Get("Location"); got != want {
		t.Errorf("Location = %q, want %q", got, want)
	}
}
