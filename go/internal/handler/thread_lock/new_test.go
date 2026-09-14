package thread_lock_test

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
	"github.com/groobb/groobb/go/internal/handler/thread_lock"
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

// fixture is a test database with the lock handler wired over its repositories,
// together with the thread the requests act on and the administrator acting, so
// a test drives the three routes against rows that are really stored.
//
// [Ja] fixtureは、そのリポジトリ上にロックのハンドラーを組み立てたテスト用データベースと、
// リクエストが働きかけるスレッド、そして操作する管理者です。テストが、実際に保存された行に
// 対して3つのルートを駆動できるようにするためです。
type fixture struct {
	db      *database.DB
	cfg     *config.Config
	handler *thread_lock.Handler
	thread  *model.Thread
	admin   model.UserID
	plain   model.UserID
}

// newFixture builds the fixture for one test: a board with one thread in it, an
// administrator, and an account holding no role at all.
//
// [Ja] newFixtureは1つのテストのためのfixtureを組み立てます。スレッドが1つ立っている
// 掲示板、管理者、そしてロールを1つも持たないアカウントです。
func newFixture(t *testing.T) *fixture {
	t.Helper()

	ctx := context.Background()
	db := testutil.SetupDB(t)
	cfg := &config.Config{Env: "test"}

	boardRepo := repository.NewBoardRepository(db)
	threadRepo := repository.NewThreadRepository(db)
	postRepo := repository.NewPostRepository(db)
	roleRepo := repository.NewRoleRepository(db)
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

	handler := thread_lock.NewHandler(
		cfg,
		httperror.NewRenderer(cfg),
		session.NewFlashManager(cfg),
		usecase.NewGetThreadModerationUsecase(roleRepo, threadRepo, postRepo, repository.NewUserRepository(db)),
		usecase.NewLockThreadUsecase(db.Writer, validator.NewModerationLogCreateValidator(), roleRepo, threadRepo, moderationLogRepo),
		usecase.NewUnlockThreadUsecase(db.Writer, roleRepo, threadRepo, moderationLogRepo),
	)

	return &fixture{
		db:      db,
		cfg:     cfg,
		handler: handler,
		thread:  thread,
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

// newRouter mounts the three lock routes the way serve.go does — behind the CSRF
// check and the method override — with actor standing in for the account
// RequireAuth resolves from a session. Going through a router is what lets a
// test submit as a form does, and what makes the id in the address reach the
// handler.
//
// [Ja] newRouterは、serve.goと同じ形で、ロックの3つのルートをCSRFの検証とメソッド
// オーバーライドの背後に置きます。actorはRequireAuthがセッションから解決するアカウントの
// 代わりです。ルーターを通すことで、テストはフォームと同じ形で送信でき、アドレスが運ぶidが
// ハンドラーへ届きます。
func newRouter(f *fixture, actor model.UserID) http.Handler {
	signedIn := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := middleware.SetUserToContext(r.Context(), &model.User{ID: actor, Atname: "actor"})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
	return mount(f, signedIn)
}

// newAnonymousRouter mounts the same three routes behind the real RequireAuth
// over a session manager reading this database, which is how a request carrying
// no session is answered the way it is in production.
//
// [Ja] newAnonymousRouterは、同じ3つのルートを、このデータベースを読むセッション
// マネージャ上の本物のRequireAuthの背後に置きます。セッションを運ばないリクエストが、
// 本番と同じ形で応答されるようにするためです。
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
	router.Use(middleware.MethodOverride)

	router.With(auth).Get("/t/{id}/lock/new", f.handler.New)
	router.With(auth).Post("/t/{id}/lock", f.handler.Create)
	router.With(auth).Delete("/t/{id}/lock", f.handler.Delete)
	return router
}

// lockPath is the address of the thread's lock, which the two submissions target.
//
// [Ja] lockPathはスレッドのロックのアドレスで、2つの送信の宛先です。
func lockPath(id model.ThreadID) string {
	return templates.ThreadLockPath(viewmodel.ThreadID(id)).String()
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

// submit sends form to path as a browser's form does: a POST carrying the
// urlencoded body, with the CSRF cookie alongside it when withCSRFCookie is set.
// The method override in the chain turns it into the DELETE when the form says
// so, which is the only way the lift is reached.
//
// [Ja] submitは、ブラウザのフォームと同じ形でformをpathへ送ります。すなわちurlencodedの
// ボディを運ぶPOSTで、withCSRFCookieのときはCSRF Cookieを添えます。チェーンのメソッド
// オーバーライドは、フォームがそう述べていればこれをDELETEへ変えます。解除へ到達する手立ては
// それだけです。
func submit(router http.Handler, path string, form url.Values, withCSRFCookie bool) *httptest.ResponseRecorder {
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

// lockForm is what the confirmation page submits: the token and the note.
//
// [Ja] lockFormは確認ページが送信するものです。トークンと注記です。
func lockForm(reason string) url.Values {
	return url.Values{"csrf_token": {csrfToken}, "reason": {reason}}
}

// unlockForm is what the thread page's lift submits: the token and the override
// that turns the POST into the DELETE.
//
// [Ja] unlockFormは、スレッドページの解除が送信するものです。トークンと、POSTをDELETEへ
// 変えるオーバーライドです。
func unlockForm() url.Values {
	return url.Values{"csrf_token": {csrfToken}, "_method": {"DELETE"}}
}

// findThread reads the thread back for an assertion about the state it was left
// in.
//
// [Ja] findThreadは、どの状態で残されたかを問う検証のためにスレッドを読み戻します。
func findThread(t *testing.T, f *fixture, id model.ThreadID) *model.Thread {
	t.Helper()

	thread, err := repository.NewThreadRepository(f.db).FindByID(context.Background(), id)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}
	if thread == nil {
		t.Fatalf("スレッドを引けない: id=%s", id)
	}
	return thread
}

// lock marks the fixture's thread locked, which is the state the lift acts on.
//
// [Ja] lockはフィクスチャのスレッドにロックの印を付けます。解除が働きかける状態です。
func lock(t *testing.T, f *fixture) {
	t.Helper()

	if err := repository.NewThreadRepository(f.db).Lock(context.Background(), f.thread.ID); err != nil {
		t.Fatalf("テスト用スレッドのロックに失敗: %v", err)
	}
}

// unpublish marks the fixture's thread unpublished, which is the state every
// operation on what the community shows is refused on.
//
// [Ja] unpublishはフィクスチャのスレッドに非公開の印を付けます。コミュニティの示している
// ものに対するあらゆる操作が拒否される状態です。
func unpublish(t *testing.T, f *fixture) {
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
// what locking does, which thread it is about, the note field, and a form
// submitting to the thread's own lock, and that the page stays out of search
// indexes.
//
// [Ja] TestNewは、確認ページを開いた管理者に、ロックが何をするのか、どのスレッドについて
// のものか、注記の入力欄、そしてスレッド自身のロックへ送信するフォームが示されること、
// そしてこのページが検索インデックスの外に留まることを検証します。
func TestNew(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	rec := get(newRouter(f, f.admin), lockPath(f.thread.ID)+"/new")

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	wants := []string{
		"スレッドをロックする",
		"ロックすると、このスレッドには誰も返信できなくなります。",
		f.thread.Title,
		`action="` + lockPath(f.thread.ID) + `"`,
		`name="csrf_token"`,
		`name="reason"`,
		"理由 (任意)",
		"操作履歴にだけ記録されます。",
		`href="` + templates.ThreadPath(viewmodel.ThreadID(f.thread.ID)).String() + `"`,
		`<meta name="robots" content="noindex"`,
	}
	for _, want := range wants {
		if !strings.Contains(body, want) {
			t.Errorf("確認ページに %q が含まれていない", want)
		}
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

	rec := get(newRouter(f, f.plain), lockPath(f.thread.ID)+"/new")

	if rec.Code != http.StatusForbidden {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

// TestNew_UnpublishedThread verifies that the confirmation page for a thread the
// community no longer shows is answered with 404 rather than naming it as a
// target.
//
// [Ja] TestNew_UnpublishedThreadは、コミュニティがもう示していないスレッドの確認ページが、
// それを対象として名指すのではなく404で応答されることを検証します。
func TestNew_UnpublishedThread(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	if err := repository.NewThreadRepository(f.db).Unpublish(context.Background(), f.thread.ID); err != nil {
		t.Fatalf("テスト用スレッドの非公開に失敗: %v", err)
	}

	rec := get(newRouter(f, f.admin), lockPath(f.thread.ID)+"/new")

	if rec.Code != http.StatusNotFound {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

// TestNew_UnknownThread verifies that an address naming no thread is answered
// with the 404 page, and that an id that is not a number is too: neither names a
// thread, and the answer does not tell them apart.
//
// [Ja] TestNew_UnknownThreadは、どのスレッドも名指していないアドレスが404ページで応答
// されること、そして数として読めないidも同様であることを検証します。どちらもスレッドを名指して
// おらず、答えが両者を区別することはありません。
func TestNew_UnknownThread(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	router := newRouter(f, f.admin)

	for _, path := range []string{lockPath(f.thread.ID+1000) + "/new", "/t/abc/lock/new"} {
		if rec := get(router, path); rec.Code != http.StatusNotFound {
			t.Errorf("%s の status code = %d, want %d", path, rec.Code, http.StatusNotFound)
		}
	}
}

// TestNew_SignedOut verifies that a visitor with no session is sent to the
// sign-in form, carrying where they were headed, rather than being told whether
// the thread is there.
//
// [Ja] TestNew_SignedOutは、セッションを持たない訪問者が、スレッドの有無を告げられるのでは
// なく、向かっていた先を運んでサインインフォームへ送られることを検証します。
func TestNew_SignedOut(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	path := lockPath(f.thread.ID) + "/new"

	rec := get(newAnonymousRouter(f), path)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	want := templates.SignInPath().WithReturnTo(path).String()
	if got := rec.Header().Get("Location"); got != want {
		t.Errorf("Location = %q, want %q", got, want)
	}
}
