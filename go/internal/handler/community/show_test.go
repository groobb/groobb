package community_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/groobb/groobb/go/internal/auth"
	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/handler/community"
	"github.com/groobb/groobb/go/internal/handler/sign_in"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/query"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/validator"
)

// showRouter mounts Show on the route pattern the server registers, so the
// handler reads {identifier} back through chi the same way it does in production
// (calling the handler directly would leave the URL parameter empty).
//
// [Ja] showRouter はサーバーが登録するルートパターンで Show をマウントし、ハンドラーが
// 本番と同じく chi 経由で {identifier} を読み戻すようにする (ハンドラーを直接呼ぶと URL
// パラメータが空になる)。
func showRouter(handler *community.Handler) http.Handler {
	r := chi.NewRouter()
	r.Get("/c/{identifier}", handler.Show)
	return r
}

// getCommunity builds a GET /c/{identifier} request with the locale set. The
// community page needs no user in the context: it is registered behind
// RequireAuth but renders the same page for every signed-in visitor.
//
// [Ja] getCommunity はロケールを設定した GET /c/{identifier} リクエストを組み立てる。
// コミュニティ画面は context にユーザーを必要としない。RequireAuth の背後に登録されるが、
// サインイン済みの訪問者にはすべて同じ画面を描画するためである。
func getCommunity(identifier, locale string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/c/"+identifier, nil)
	return req.WithContext(i18n.SetLocale(req.Context(), locale))
}

// seedCommunity creates a committed community for the page to render.
//
// [Ja] seedCommunity は画面が描画するコミュニティをコミットして作成する。
func seedCommunity(t *testing.T, communityRepo *repository.CommunityRepository, name, identifier string) {
	t.Helper()

	if _, err := communityRepo.Create(context.Background(), repository.CreateCommunityInput{
		Name:       name,
		Identifier: identifier,
	}); err != nil {
		t.Fatalf("コミュニティの作成に失敗: %v", err)
	}
}

// TestShow_Success verifies that an existing identifier renders the community's
// page: HTTP 200 with the name as both the heading and the title, the localized
// description naming the community, and the noindex robots meta that keeps the
// page behind authentication out of search indexes.
//
// [Ja] TestShow_Success は、存在する識別子がコミュニティの画面を描画することを検証する。
// HTTP 200 と、見出しかつタイトルとしての名前、コミュニティ名を含むローカライズ済みの
// description、そして認証の背後のページを検索インデックスから除外する noindex の robots
// メタである。
func TestShow_Success(t *testing.T) {
	t.Parallel()

	handler, communityRepo := newCommunityHandler(t)
	identifier := uniqueIdentifier("show")
	seedCommunity(t, communityRepo, "表示テストコミュニティ", identifier)

	tests := []struct {
		name            string
		locale          string
		wantDescription string
	}{
		{
			name:            "Japanese",
			locale:          i18n.LangJa,
			wantDescription: "表示テストコミュニティのコミュニティです。",
		},
		{
			name:            "English",
			locale:          i18n.LangEn,
			wantDescription: "The 表示テストコミュニティ community.",
		},
	}

	router := showRouter(handler)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, getCommunity(identifier, tt.locale))

			if rec.Code != http.StatusOK {
				t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
			}
			if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
				t.Errorf("Content-Type = %q, want prefix %q", got, "text/html")
			}

			body := rec.Body.String()
			wants := []string{
				"<title>表示テストコミュニティ</title>",
				fmt.Sprintf(`<meta name="description" content="%s"`, tt.wantDescription),
				`<meta name="robots" content="noindex"`,
				`<h1 class="card-title">表示テストコミュニティ</h1>`,
			}
			for _, want := range wants {
				if !strings.Contains(body, want) {
					t.Errorf("response body does not contain %q", want)
				}
			}
		})
	}
}

// TestShow_NotFound verifies that an identifier nobody has claimed is answered
// with 404 and the localized message, rather than a redirect elsewhere or an
// empty page served with 200.
//
// [Ja] TestShow_NotFound は、誰も取得していない識別子が、別の場所へのリダイレクトや
// 200 で配信される空のページではなく、404 とローカライズ済みメッセージで応答されることを
// 検証する。
func TestShow_NotFound(t *testing.T) {
	t.Parallel()

	handler, _ := newCommunityHandler(t)

	rec := httptest.NewRecorder()
	showRouter(handler).ServeHTTP(rec, getCommunity(uniqueIdentifier("miss"), i18n.LangJa))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusNotFound)
	}
	want := i18n.T(i18n.SetLocale(context.Background(), i18n.LangJa), "error_not_found_message")
	if !strings.Contains(rec.Body.String(), want) {
		t.Errorf("response body does not contain %q", want)
	}
}

// TestShow_RedirectsToCanonicalIdentifier verifies that a spelling differing only
// in letter case is sent to the identifier the community was created with, and
// that the request's query reaches the canonical URL with it. The identifier
// column is citext, so both spellings resolve the same community; serving both
// would give one community several URLs.
//
// [Ja] TestShow_RedirectsToCanonicalIdentifier は、大文字小文字だけが異なる表記が、
// コミュニティが作成されたときの識別子へ送られること、そしてリクエストのクエリが一緒に
// 正規の URL へ届くことを検証する。identifier 列は citext のため、どちらの表記も同じ
// コミュニティを解決する。両方をそのまま配信すると 1 つのコミュニティが複数の URL を
// 持つことになる。
func TestShow_RedirectsToCanonicalIdentifier(t *testing.T) {
	t.Parallel()

	handler, communityRepo := newCommunityHandler(t)
	identifier := uniqueIdentifier("Case")
	seedCommunity(t, communityRepo, "表記テストコミュニティ", identifier)

	tests := []struct {
		name     string
		rawQuery string
		want     string
	}{
		{
			name: "クエリなし",
			want: "/c/" + identifier,
		},
		{
			// A shared link carries campaign parameters, and the redirect that
			// corrects its spelling must not be where they are lost.
			//
			// [Ja] 共有リンクは計測パラメータを載せてくる。表記を正す本リダイレクトが、
			// それを失う場所になってはならない。
			name:     "クエリあり",
			rawQuery: "utm_source=twitter",
			want:     "/c/" + identifier + "?utm_source=twitter",
		},
	}

	router := showRouter(handler)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := getCommunity(strings.ToLower(identifier), i18n.LangJa)
			req.URL.RawQuery = tt.rawQuery

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusMovedPermanently {
				t.Fatalf("status code = %d, want %d", rec.Code, http.StatusMovedPermanently)
			}
			if got := rec.Header().Get("Location"); got != tt.want {
				t.Errorf("Location = %q, want %q", got, tt.want)
			}
		})
	}
}

// newSignInHandler wires a sign-in Handler over the shared pool, with a Turnstile
// verifier that passes, so a test can follow the sign-in the community page's
// RequireAuth redirect sends an anonymous visitor to.
//
// [Ja] newSignInHandler は共有プールで、通過する Turnstile 検証器を伴ってサインイン
// Handler を組み立てる。コミュニティ画面の RequireAuth が匿名の訪問者を送るサインインを、
// テストが追えるようにするためのもの。
func newSignInHandler(t *testing.T, cfg *config.Config, sessionMgr *session.Manager) *sign_in.Handler {
	t.Helper()

	queries := query.New(testutil.GetTestDB())
	createSignInUC := usecase.NewCreateSignInUsecase(validator.NewSignInCreateValidator(
		repository.NewUserRepository(queries),
		repository.NewUserPasswordRepository(queries),
		repository.NewUserTwoFactorAuthRepository(queries),
	))
	createSessionUC := usecase.NewCreateSessionUsecase(repository.NewUserSessionRepository(queries))
	return sign_in.NewHandler(cfg, sessionMgr, createSignInUC, createSessionUC, &testutil.FakeTurnstileVerifier{Passed: true})
}

// seedUserWithPassword creates a committed user with a password credential,
// returning the email so a test can sign in as it.
//
// [Ja] seedUserWithPassword はパスワード資格情報を持つコミット済みユーザーを作成し、
// テストがそれとしてサインインできるよう email を返す。
func seedUserWithPassword(t *testing.T, password string) string {
	t.Helper()

	ctx := context.Background()
	queries := query.New(testutil.GetTestDB())
	email := fmt.Sprintf("community-show-%s@example.com", uuid.NewString())

	user, err := repository.NewUserRepository(queries).Create(ctx, repository.CreateUserInput{
		Email:    email,
		Atname:   testutil.UniqueAtname(),
		Locale:   i18n.LangJa,
		TimeZone: "Asia/Tokyo",
	})
	if err != nil {
		t.Fatalf("ユーザーの作成に失敗: %v", err)
	}

	digest, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("パスワードのハッシュ化に失敗: %v", err)
	}
	if _, err := repository.NewUserPasswordRepository(queries).Create(ctx, repository.CreateUserPasswordInput{
		UserID:         user.ID,
		PasswordDigest: digest,
	}); err != nil {
		t.Fatalf("パスワード資格情報の作成に失敗: %v", err)
	}
	return email
}

// TestShow_SignedOutVisitorReturnsAfterSignIn verifies the round trip a shared
// community link takes when whoever follows it is signed out: RequireAuth turns
// the request away to sign-in carrying the community URL, signing in lands the
// visitor on that URL instead of the home page, and the session it issued opens
// the page there. The hops are covered on their own by the middleware and sign-in
// tests; what this test adds is that the community page's own route closes the
// loop, since being reachable from a shared link is why the page exists. Going
// through the guard is also the only way to see a real page carry the cache
// policy the guard sets, so the page opened at the end is checked for it.
//
// [Ja] TestShow_SignedOutVisitorReturnsAfterSignIn は、コミュニティの共有リンクを踏んだ人が
// 未サインインだったときの往復を検証する。RequireAuth はリクエストをコミュニティの URL を
// 載せてサインインへ追い返し、サインインすると訪問者はホームではなくその URL に着地し、
// 発行されたセッションでその画面が開く。各ホップ自体はミドルウェアとサインインのテストが
// 個別に検証している。本テストが加えるのは、コミュニティ画面自身のルートでこの往復が閉じる
// ことの確認である。共有リンクから辿れることはこの画面が存在する理由そのものだからである。
// 実際のページがガードの設定するキャッシュ方針を載せていることを見られるのもガード越しの
// 経路だけであるため、最後に開いた画面でそれも検証する。
func TestShow_SignedOutVisitorReturnsAfterSignIn(t *testing.T) {
	t.Parallel()

	handler, communityRepo := newCommunityHandler(t)
	identifier := uniqueIdentifier("back")
	seedCommunity(t, communityRepo, "共有リンクコミュニティ", identifier)
	communityPath := "/c/" + identifier

	cfg := &config.Config{Env: "test"}
	sessionMgr := session.NewManager(repository.NewUserRepository(query.New(testutil.GetTestDB())), cfg)

	guarded := chi.NewRouter()
	guarded.With(middleware.NewAuth(sessionMgr).RequireAuth).Get("/c/{identifier}", handler.Show)

	rec := httptest.NewRecorder()
	guarded.ServeHTTP(rec, getCommunity(identifier, i18n.LangJa))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("未サインインの status code = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	location, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatalf("Location の解析に失敗: %v", err)
	}
	if location.Path != "/sign_in" {
		t.Errorf("未サインインのリダイレクト先 = %q, want %q", location.Path, "/sign_in")
	}
	returnTo := location.Query().Get("return_to")
	if returnTo != communityPath {
		t.Fatalf("return_to = %q, want %q", returnTo, communityPath)
	}

	password := "password123"
	email := seedUserWithPassword(t, password)
	form := url.Values{"email": {email}, "password": {password}, "return_to": {returnTo}}
	req := httptest.NewRequest(http.MethodPost, "/sign_in", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(i18n.SetLocale(req.Context(), i18n.LangJa))

	signInRec := httptest.NewRecorder()
	newSignInHandler(t, cfg, sessionMgr).Create(signInRec, req)

	if signInRec.Code != http.StatusSeeOther {
		t.Fatalf("サインインの status code = %d, want %d", signInRec.Code, http.StatusSeeOther)
	}
	if got := signInRec.Header().Get("Location"); got != communityPath {
		t.Errorf("サインイン後の Location = %q, want %q", got, communityPath)
	}

	// Follow that redirect carrying the session the sign-in issued. Without this
	// hop the test would show where the visitor is sent but not that they can
	// open it, which is the whole point of returning them there.
	//
	// [Ja] サインインが発行したセッションを載せて、そのリダイレクトを追う。このホップが
	// 無いと、訪問者がどこへ送られるかは示せても、そこを開けることは示せない。元の場所へ
	// 戻す目的そのものがそこにある。
	followUp := getCommunity(identifier, i18n.LangJa)
	for _, cookie := range signInRec.Result().Cookies() {
		followUp.AddCookie(cookie)
	}

	followUpRec := httptest.NewRecorder()
	guarded.ServeHTTP(followUpRec, followUp)

	if followUpRec.Code != http.StatusOK {
		t.Fatalf("サインイン後にコミュニティ画面を開いたときの status code = %d, want %d", followUpRec.Code, http.StatusOK)
	}
	if got := followUpRec.Header().Get("Cache-Control"); got != "private, no-cache" {
		t.Errorf("Cache-Control = %q, want %q", got, "private, no-cache")
	}
	if want := `<h1 class="card-title">共有リンクコミュニティ</h1>`; !strings.Contains(followUpRec.Body.String(), want) {
		t.Errorf("response body does not contain %q", want)
	}
}
