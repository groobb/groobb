package thread_unpublication_test

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
	"github.com/groobb/groobb/go/internal/handler/thread_unpublication"
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

// csrfTokenは、本パッケージのどの送信もCookieとボディの両方で運ぶトークンで、CSRFの
// 検証が突き合わせる相手です。値そのものに意味は無く、両者が一致していることがすべてです。
const csrfToken = "test-csrf-token"

// fixtureは、そのリポジトリ上に非公開のハンドラーを組み立てたテスト用データベースと、
// スレッドが立っている掲示板、リクエストが働きかけるスレッド、そして操作する管理者です。
// テストが、実際に保存された行に対して2つのルートを駆動できるようにするためです。
//
// 掲示板を保持するのは、そこが操作の降り立つ先であるためです。操作が行われたスレッドは、もう
// 誰かを送る先ではありません。
type fixture struct {
	db      *database.DB
	cfg     *config.Config
	handler *thread_unpublication.Handler
	board   *model.Board
	thread  *model.Thread
	admin   model.UserID
	plain   model.UserID
}

// newFixtureは1つのテストのためのfixtureを組み立てます。スレッドが1つ立っている
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

	handler := thread_unpublication.NewHandler(
		cfg,
		httperror.NewRenderer(cfg),
		session.NewFlashManager(cfg),
		usecase.NewGetThreadModerationUsecase(roleRepo, threadRepo, postRepo, userRepo),
		usecase.NewUnpublishThreadUsecase(
			db.Writer,
			validator.NewModerationLogCreateValidator(),
			roleRepo,
			threadRepo,
			boardRepo,
			moderationLogRepo,
		),
	)

	return &fixture{
		db:      db,
		cfg:     cfg,
		handler: handler,
		board:   board,
		thread:  thread,
		admin:   seedAdmin(t, db),
		plain:   testutil.NewUserBuilder(t, db).WithAtname("plainuser").Build(),
	}
}

// seedAdminは、組み込みの管理者ロールを持つアカウントを作ります。スレッドに対する
// あらゆる操作を許すのがこれです。
func seedAdmin(t *testing.T, db *database.DB) model.UserID {
	t.Helper()

	userID := testutil.NewUserBuilder(t, db).WithAtname("adminuser").Build()
	testutil.NewUserRoleBuilder(t, db).WithUserID(userID).Build()
	return userID
}

// newRouterは、serve.goと同じ形で、2つのルートをCSRFの検証の背後に置きます。actorは
// RequireAuthがセッションから解決するアカウントの代わりです。ルーターを通すことで、テストは
// フォームと同じ形で送信でき、アドレスが運ぶidがハンドラーへ届きます。
func newRouter(f *fixture, actor model.UserID) http.Handler {
	signedIn := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := middleware.SetUserToContext(r.Context(), &model.User{ID: actor, Atname: "actor"})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
	return mount(f, signedIn)
}

// newAnonymousRouterは、同じ2つのルートを、このデータベースを読むセッションマネージャ
// 上の本物のRequireAuthの背後に置きます。セッションを運ばないリクエストが、本番と同じ形で
// 応答されるようにするためです。
func newAnonymousRouter(f *fixture) http.Handler {
	auth := middleware.NewAuth(session.NewManager(repository.NewUserRepository(f.db), f.cfg))
	return mount(f, auth.RequireAuth)
}

// mountは、上の2つのコンストラクタが共有するルーターを組み立てます。違うのは、
// サインインの検査の位置に何が立つかだけです。
func mount(f *fixture, auth func(http.Handler) http.Handler) http.Handler {
	router := chi.NewRouter()
	router.Use(middleware.PostFormLimit)
	router.Use(middleware.NewCSRF(f.cfg).Middleware)

	router.With(auth).Get("/t/{id}/unpublication/new", f.handler.New)
	router.With(auth).Post("/t/{id}/unpublication", f.handler.Create)
	return router
}

// unpublicationPathはスレッドの非公開のアドレスで、送信の宛先です。
func unpublicationPath(id model.ThreadID) string {
	return templates.ThreadUnpublicationPath(viewmodel.ThreadID(id)).String()
}

// getは、サインイン済みの訪問者のブラウザと同じ形で、日本語でチェーン越しにページを
// 読みます。
func get(router http.Handler, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req = req.WithContext(i18n.SetLocale(req.Context(), model.LocaleJa))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// submitは、確認ページのフォームをブラウザと同じ形でpathへ送ります。すなわち
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

// findThreadは、どの状態で残されたかを問う検証のためにスレッドを読み戻します。
func findThread(t *testing.T, f *fixture, id model.ThreadID) *model.Thread {
	t.Helper()

	thread, err := repository.NewThreadRepository(f.db).FindByID(context.Background(), id)
	if err != nil {
		t.Fatalf("FindByID()のエラー = %v", err)
	}
	if thread == nil {
		t.Fatalf("スレッドを引けない: id=%s", id)
	}
	return thread
}

// unpublishはフィクスチャのスレッドに非公開の印を付けます。確認ページが対象を名指すのを
// 拒む状態です。
func unpublish(t *testing.T, f *fixture) {
	t.Helper()

	if err := repository.NewThreadRepository(f.db).Unpublish(context.Background(), f.thread.ID); err != nil {
		t.Fatalf("テスト用スレッドの非公開に失敗: %v", err)
	}
}

// decodeFlashは、レスポンスが設定したフラッシュを読みます。リダイレクトした操作が、
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

// TestNewは、確認ページを開いた管理者に、非公開が何をするのか、どのスレッドについての
// ものか、注記の入力欄、そしてスレッド自身の非公開へ送信するフォームが示されること、そして
// このページが検索インデックスの外に留まることを検証します。
func TestNew(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	rec := get(newRouter(f, f.admin), unpublicationPath(f.thread.ID)+"/new")

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	wants := []string{
		"スレッドを非公開にする",
		"非公開にすると、このスレッドは一覧に並ばなくなり",
		f.thread.Title,
		`action="` + unpublicationPath(f.thread.ID) + `"`,
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

// TestNew_WithoutPermissionは、スレッドに対するどの操作も許されていないアカウントが、
// そこへ至る画面を見せられるのではなく403ページで応答されることを検証します。
func TestNew_WithoutPermission(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	rec := get(newRouter(f, f.plain), unpublicationPath(f.thread.ID)+"/new")

	if rec.Code != http.StatusForbidden {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusForbidden)
	}
}

// TestNew_UnpublishedThreadは、コミュニティがもう示していないスレッドの確認ページが、
// 既に視界の外にあるものを対象として名指すのではなく404で応答されることを検証します。
func TestNew_UnpublishedThread(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	unpublish(t, f)

	rec := get(newRouter(f, f.admin), unpublicationPath(f.thread.ID)+"/new")

	if rec.Code != http.StatusNotFound {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusNotFound)
	}
}

// TestNew_UnknownThreadは、どのスレッドも名指していないアドレスが404ページで応答
// されること、そして数として読めないidも同様であることを検証します。どちらもスレッドを名指して
// おらず、答えが両者を区別することはありません。
func TestNew_UnknownThread(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	router := newRouter(f, f.admin)

	paths := []string{
		unpublicationPath(f.thread.ID+1000) + "/new",
		"/t/abc/unpublication/new",
	}
	for _, path := range paths {
		if rec := get(router, path); rec.Code != http.StatusNotFound {
			t.Errorf("%s のステータスコード = %d、期待値 = %d", path, rec.Code, http.StatusNotFound)
		}
	}
}

// TestNew_SignedOutは、セッションを持たない訪問者が、スレッドの有無を告げられるのでは
// なく、向かっていた先を運んでサインインフォームへ送られることを検証します。
func TestNew_SignedOut(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	path := unpublicationPath(f.thread.ID) + "/new"

	rec := get(newAnonymousRouter(f), path)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusSeeOther)
	}
	want := templates.SignInPath().WithReturnTo(path).String()
	if got := rec.Header().Get("Location"); got != want {
		t.Errorf("Location = %q、期待値 = %q", got, want)
	}
}
