package admin_user_role_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/handler/admin_user"
	"github.com/groobb/groobb/go/internal/handler/admin_user_role"
	"github.com/groobb/groobb/go/internal/httperror"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/templates"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// csrfToken is the token every request in this file carries in both the cookie
// and the submission, which is what the CSRF check compares. Its value says
// nothing; that the two sides agree is the whole of it.
//
// [Ja] csrfToken は、本ファイルのどのリクエストも Cookie と送信の両方で運ぶトークンで、
// CSRF の検証が突き合わせる相手です。値そのものに意味は無く、両者が一致していることが
// すべてです。
const csrfToken = "test-csrf-token"

// fixture is a test database with the two role handlers wired over its
// repositories, so a test drives the grant and the revoke against roles that are
// really stored.
//
// [Ja] fixture は、そのリポジトリでロールの 2 つのハンドラーを組み立てたテスト用
// データベースです。テストが、実際に保存されたロールに対して付与と剥奪を駆動できる
// ようにするためです。
type fixture struct {
	db      *database.DB
	cfg     *config.Config
	handler *admin_user_role.Handler
}

// newFixture builds the fixture for one test.
//
// [Ja] newFixture は 1 つのテストのための fixture を組み立てます。
func newFixture(t *testing.T) *fixture {
	t.Helper()

	db := testutil.SetupDB(t)
	cfg := &config.Config{Env: "test"}
	roleRepo := repository.NewRoleRepository(db)
	userRepo := repository.NewUserRepository(db)
	userRoleRepo := repository.NewUserRoleRepository(db)

	handler := admin_user_role.NewHandler(
		httperror.NewRenderer(cfg),
		session.NewFlashManager(cfg),
		usecase.NewGrantUserRoleUsecase(db.Writer, roleRepo, userRepo, userRoleRepo),
		usecase.NewRevokeUserRoleUsecase(db.Writer, roleRepo, userRepo, userRoleRepo),
	)
	return &fixture{db: db, cfg: cfg, handler: handler}
}

// newRouter mounts the way serve.go does — behind the CSRF check and the method
// override — the two role routes and the listing whose buttons submit to them,
// with actor standing in for the account RequireAuth resolves from a session.
// Going through a router is what lets a test submit the revoke as a form does,
// and what makes the id and the role name in the address reach the handler.
//
// The listing is served from the same chain rather than from a handler of its
// own, because the token it writes into a form and the token a submission is
// checked against are only the same value if one chain issued both.
//
// [Ja] newRouter は、serve.go と同じ形で、ロールの 2 つのルートと、そのボタンが送信する
// 先である一覧とを、CSRF の検証とメソッドオーバーライドの背後に置きます。actor は
// RequireAuth がセッションから解決するアカウントの代わりです。ルーターを通すことで、
// テストはフォームと同じ形で剥奪を送信でき、アドレスが運ぶ id とロール名がハンドラーへ
// 届きます。
//
// 一覧を専用のハンドラーからではなく同じチェーンから応答させるのは、一覧がフォームへ
// 書き込むトークンと、送信が突き合わされるトークンとが同じ値になるのは、1 つのチェーンが
// 両方を発行した場合だけであるためです。
func newRouter(f *fixture, actor model.UserID) http.Handler {
	router := chi.NewRouter()
	router.Use(middleware.PostFormLimit)
	router.Use(middleware.NewCSRF(f.cfg).Middleware)
	router.Use(middleware.MethodOverride)

	signedIn := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := middleware.SetUserToContext(r.Context(), &model.User{ID: actor, Atname: "actor"})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}

	listing := admin_user.NewHandler(
		f.cfg,
		httperror.NewRenderer(f.cfg),
		usecase.NewGetAdminUsersUsecase(repository.NewRoleRepository(f.db), repository.NewUserRepository(f.db)),
	)

	router.With(signedIn).Get("/admin/users", listing.Index)
	router.With(signedIn).Post("/admin/users/{id}/roles", f.handler.Create)
	router.With(signedIn).Delete("/admin/users/{id}/roles/{name}", f.handler.Delete)
	return router
}

// submit sends form to path as a browser's form does: a POST carrying the
// urlencoded body, with the CSRF cookie alongside it when withCSRFCookie is set.
// The method override in the chain turns it into the DELETE when the form says
// so, which is the only way the revoke route is reached.
//
// [Ja] submit は、ブラウザのフォームと同じ形で form を path へ送ります。すなわち
// urlencoded のボディを運ぶ POST で、withCSRFCookie のときは CSRF Cookie を添えます。
// チェーンのメソッドオーバーライドは、フォームがそう述べていればこれを DELETE へ変えます。
// 剥奪のルートへ到達する手立てはそれだけです。
func submit(router http.Handler, path string, form url.Values, withCSRFCookie bool) *httptest.ResponseRecorder {
	var csrf *http.Cookie
	if withCSRFCookie {
		csrf = &http.Cookie{Name: middleware.CSRFCookieName, Value: csrfToken}
	}
	return submitWith(router, path, form, csrf)
}

// submitWith sends form to path carrying the given CSRF cookie, which is how a
// submission whose token came from a page the test read is sent: the cookie that
// response set is the one the check compares against.
//
// [Ja] submitWith は、指定した CSRF Cookie を添えて form を path へ送ります。テストが
// 読んだページからトークンを得た送信は、この形で送ります。その応答が設定した Cookie が、
// 検証が突き合わせる相手であるためです。
func submitWith(router http.Handler, path string, form url.Values, csrf *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if csrf != nil {
		req.AddCookie(csrf)
	}
	req = req.WithContext(i18n.SetLocale(req.Context(), model.LocaleJa))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// readListing reads the first page of the listing through the same chain the
// buttons submit through, and hands back what was drawn together with the CSRF
// cookie the response set.
//
// [Ja] readListing は、ボタンが送信するのと同じチェーンを通して一覧の 1 ページ目を読み、
// 描かれたものを、その応答が設定した CSRF Cookie と共に返します。
func readListing(t *testing.T, router http.Handler) (string, *http.Cookie) {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, templates.AdminUsersPath().String(), nil)
	req = req.WithContext(i18n.SetLocale(req.Context(), model.LocaleJa))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("一覧の status code = %d, want %d", rec.Code, http.StatusOK)
	}
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == middleware.CSRFCookieName {
			return rec.Body.String(), cookie
		}
	}

	t.Fatal("一覧の応答が CSRF Cookie を設定していない")
	return "", nil
}

// formCSRFTokenPattern reads the value of the token field the row forms carry.
// The listing writes the same one into every row, so the first is the page's.
//
// [Ja] formCSRFTokenPattern は、行のフォームが運ぶトークンのフィールドの値を読み取ります。
// 一覧はどの行にも同じものを書き込むため、最初の 1 つがそのページのものです。
var formCSRFTokenPattern = regexp.MustCompile(`name="csrf_token" value="([^"]*)"`)

// grantForm is the submission the listing's grant button sends: the role being
// handed out, the CSRF token, and the address of the listing it was pressed on.
//
// [Ja] grantForm は一覧の付与のボタンが送る送信です。渡されるロール、CSRF トークン、
// そしてそれが押された一覧のアドレスです。
func grantForm(roleName model.RoleName, atnamePrefix, page string) url.Values {
	return url.Values{
		"csrf_token": {csrfToken},
		"role_name":  {string(roleName)},
		"q":          {atnamePrefix},
		"page":       {page},
	}
}

// revokeForm is the submission the listing's revoke button sends. It names no
// role, since the address does, and it carries the method override that turns
// the POST a form can send into the DELETE the route answers.
//
// [Ja] revokeForm は一覧の剥奪のボタンが送る送信です。ロールを名指さないのはアドレスが
// 名指すためで、フォームが送れる POST を、ルートが応答する DELETE に変えるメソッド
// オーバーライドを運びます。
func revokeForm(atnamePrefix, page string) url.Values {
	return url.Values{
		"_method":    {http.MethodDelete},
		"csrf_token": {csrfToken},
		"q":          {atnamePrefix},
		"page":       {page},
	}
}

// buildAdministrator creates an account holding the built-in admin role.
//
// [Ja] buildAdministrator は組み込みの admin ロールを持つアカウントを作成します。
func buildAdministrator(t *testing.T, db *database.DB, atname string) model.UserID {
	t.Helper()

	userID := testutil.NewUserBuilder(t, db).WithAtname(atname).Build()
	testutil.NewUserRoleBuilder(t, db).WithUserID(userID).Build()
	return userID
}

// holdsAdmin reports whether the account holds the built-in admin role, read
// back through the repository the screens read it through.
//
// [Ja] holdsAdmin は、そのアカウントが組み込みの admin ロールを持っているかどうかを、
// 画面が読むのと同じリポジトリを通して読み戻して返します。
func holdsAdmin(t *testing.T, db *database.DB, userID model.UserID) bool {
	t.Helper()

	roles, err := repository.NewRoleRepository(db).ListByUserID(context.Background(), userID)
	if err != nil {
		t.Fatalf("ロールの取得に失敗: %v", err)
	}
	for _, role := range roles {
		if role.Name == model.RoleNameAdmin {
			return true
		}
	}
	return false
}

// decodeFlash reads the flash message the response carries across the redirect,
// which is how both handlers say what happened.
//
// [Ja] decodeFlash は、レスポンスがリダイレクトをまたいで運ぶフラッシュメッセージを
// 読み取ります。どちらのハンドラーも、何が起きたのかをこれで述べるためです。
func decodeFlash(t *testing.T, rec *httptest.ResponseRecorder) *session.FlashMessage {
	t.Helper()

	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name != session.FlashCookieName {
			continue
		}
		data, err := base64.StdEncoding.DecodeString(cookie.Value)
		if err != nil {
			t.Fatalf("フラッシュ Cookie の base64 デコードに失敗: %v", err)
		}
		var flash session.FlashMessage
		if err := json.Unmarshal(data, &flash); err != nil {
			t.Fatalf("フラッシュ Cookie の JSON デコードに失敗: %v", err)
		}
		return &flash
	}

	t.Fatal("フラッシュ Cookie が設定されていない")
	return nil
}

// rolesPath is the address of one account's roles, which the grant submits to.
//
// [Ja] rolesPath は 1 つのアカウントのロールのアドレスで、付与の送信先です。
func rolesPath(id model.UserID) string {
	return templates.AdminUserRolesPath(viewmodel.UserID(id)).String()
}

// TestCreate_GrantsTheRole verifies that an administrator pressing the grant
// button gives the account the role, is told so, and is answered with the page
// of the listing the button was pressed on rather than its first page.
//
// [Ja] TestCreate_GrantsTheRole は、管理者が付与のボタンを押すとアカウントがロールを
// 得ること、そのことが伝えられること、そして応答が、一覧の最初のページではなく、ボタンが
// 押されたページであることを検証します。
func TestCreate_GrantsTheRole(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	adminID := buildAdministrator(t, f.db, "adminuser")
	targetID := testutil.NewUserBuilder(t, f.db).WithAtname("plainuser").Build()

	rec := submit(newRouter(f, adminID), rolesPath(targetID), grantForm(model.RoleNameAdmin, "plain", "2"), true)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	want := templates.AdminUsersPagePath("plain", 2).String()
	if got := rec.Header().Get("Location"); got != want {
		t.Errorf("Location = %q, want %q", got, want)
	}
	if flash := decodeFlash(t, rec); flash.Type != session.FlashSuccess || flash.Message != "@plainuserに管理者ロールを付与しました" {
		t.Errorf("フラッシュ = %+v, want 成功の「@plainuserに管理者ロールを付与しました」", flash)
	}
	if !holdsAdmin(t, f.db, targetID) {
		t.Error("付与した相手が admin ロールを持っていない")
	}
}

// TestCreate_NormalizesTheReturnListingPage verifies that a submission carrying
// no valid page number is answered at the first page of the listing it came
// from. An unfiltered first page keeps its canonical address.
//
// [Ja] TestCreate_NormalizesTheReturnListingPage は、有効なページ番号を運ばない送信が、
// 送信元の一覧の 1 ページ目で応答されることを検証します。絞り込みの無い 1 ページ目は、
// 正規のアドレスを保ちます。
func TestCreate_NormalizesTheReturnListingPage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		atnamePrefix string
		page         string
		want         templates.Path
	}{
		{name: "ページ番号が無い", want: templates.AdminUsersPath()},
		{name: "整数でないページ番号", atnamePrefix: "plain", page: "abc", want: templates.AdminUsersPagePath("plain", 1)},
		{name: "0ページ", atnamePrefix: "plain", page: "0", want: templates.AdminUsersPagePath("plain", 1)},
		{name: "負のページ番号", atnamePrefix: "plain", page: "-1", want: templates.AdminUsersPagePath("plain", 1)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := newFixture(t)
			adminID := buildAdministrator(t, f.db, "adminuser")
			targetID := testutil.NewUserBuilder(t, f.db).WithAtname("plainuser").Build()

			rec := submit(
				newRouter(f, adminID),
				rolesPath(targetID),
				grantForm(model.RoleNameAdmin, tt.atnamePrefix, tt.page),
				true,
			)

			if rec.Code != http.StatusSeeOther {
				t.Errorf("status code = %d, want %d", rec.Code, http.StatusSeeOther)
			}
			if got, want := rec.Header().Get("Location"), tt.want.String(); got != want {
				t.Errorf("Location = %q, want %q", got, want)
			}
		})
	}
}

// TestCreate_WithoutPermission verifies that an account that may not hand roles
// out is answered with the 403 page, and that nothing was given away.
//
// [Ja] TestCreate_WithoutPermission は、ロールを配ってはならないアカウントが 403 ページで
// 応答されること、そして何も渡されていないことを検証します。
func TestCreate_WithoutPermission(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	actorID := testutil.NewUserBuilder(t, f.db).WithAtname("plainuser").Build()
	targetID := testutil.NewUserBuilder(t, f.db).WithAtname("otheruser").Build()

	rec := submit(newRouter(f, actorID), rolesPath(targetID), grantForm(model.RoleNameAdmin, "", ""), true)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if holdsAdmin(t, f.db, targetID) {
		t.Error("権限の無い操作者の送信で admin ロールが渡っている")
	}
}

// TestCreate_NamesNothing verifies that an address or a submission naming no
// account and no role is answered with the 404 page: an id that is not a whole
// number, an account that is not there, and a role this instance does not have.
//
// [Ja] TestCreate_NamesNothing は、どのアカウントもどのロールも名指していないアドレスや
// 送信が 404 ページで応答されることを検証します。整数でない id、存在しないアカウント、
// そしてこのインスタンスが持たないロールです。
func TestCreate_NamesNothing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		path     func(missing model.UserID) string
		roleName model.RoleName
	}{
		{
			name:     "id が整数ではない",
			path:     func(model.UserID) string { return "/admin/users/abc/roles" },
			roleName: model.RoleNameAdmin,
		},
		{
			name:     "アカウントが存在しない",
			path:     func(missing model.UserID) string { return rolesPath(missing) },
			roleName: model.RoleNameAdmin,
		},
		{
			name:     "ロールが存在しない",
			path:     func(missing model.UserID) string { return rolesPath(missing - 1) },
			roleName: model.RoleName("moderator"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := newFixture(t)
			adminID := buildAdministrator(t, f.db, "adminuser")
			// The account one past the administrator's id is not there, and the id
			// before it is the administrator: one path names nobody, the other names
			// somebody so that only the role is missing.
			//
			// [Ja] 管理者の id の 1 つ先のアカウントは存在せず、その 1 つ手前は管理者自身で
			// ある。一方のパスは誰も名指さず、もう一方は誰かを名指すため、欠けているのが
			// ロールだけになる。
			missing := adminID + 1

			rec := submit(newRouter(f, adminID), tt.path(missing), grantForm(tt.roleName, "", ""), true)

			if rec.Code != http.StatusNotFound {
				t.Errorf("status code = %d, want %d", rec.Code, http.StatusNotFound)
			}
		})
	}
}

// TestCreate_WithoutCSRFToken verifies that a submission arriving without the
// token the form embeds is refused before the handler, and that no role was
// handed out. A grant that a link on another site could trigger would let anyone
// make themselves an administrator.
//
// [Ja] TestCreate_WithoutCSRFToken は、フォームが埋め込むトークンを伴わずに届いた送信が
// ハンドラーの手前で拒否されること、そしてロールが渡っていないことを検証します。他サイトの
// リンクが起こせる付与は、誰もが自分を管理者にできることを意味するためです。
func TestCreate_WithoutCSRFToken(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	adminID := buildAdministrator(t, f.db, "adminuser")
	targetID := testutil.NewUserBuilder(t, f.db).WithAtname("plainuser").Build()

	form := grantForm(model.RoleNameAdmin, "", "")
	form.Del("csrf_token")
	rec := submit(newRouter(f, adminID), rolesPath(targetID), form, false)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if holdsAdmin(t, f.db, targetID) {
		t.Error("CSRF トークンの無い送信で admin ロールが渡っている")
	}
}

// TestCreate_TheListingShowsTheGrant verifies the round trip the visitor makes:
// the grant is submitted, and the listing it returns to draws the account with
// the role it now holds and offers to take it back, rather than to give it
// again.
//
// [Ja] TestCreate_TheListingShowsTheGrant は、訪問者が辿る往復を検証します。付与が
// 送信され、戻った先の一覧が、そのアカウントを今持っているロールと共に描き、もう一度
// 与えるのではなく取り上げることを差し出します。
func TestCreate_TheListingShowsTheGrant(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	adminID := buildAdministrator(t, f.db, "adminuser")
	targetID := testutil.NewUserBuilder(t, f.db).WithAtname("plainuser").Build()

	router := newRouter(f, adminID)
	if rec := submit(router, rolesPath(targetID), grantForm(model.RoleNameAdmin, "", ""), true); rec.Code != http.StatusSeeOther {
		t.Fatalf("付与の status code = %d, want %d", rec.Code, http.StatusSeeOther)
	}

	body, _ := readListing(t, router)
	revokePath := templates.AdminUserRolePath(viewmodel.UserID(targetID), string(model.RoleNameAdmin)).String()
	if !strings.Contains(body, `action="`+revokePath+`"`) {
		t.Errorf("一覧に %q へ送る剥奪フォームが無い", revokePath)
	}
	if strings.Contains(body, `action="`+rolesPath(targetID)+`"`) {
		t.Errorf("一覧に %q へ送る付与フォームが残っている", rolesPath(targetID))
	}
}

// TestCreate_TheListingCarriesTheTokenTheCheckCompares verifies that the token
// the listing writes into its role forms is the one a submission is checked
// against: it is the value of the cookie the same response set, and pressing the
// button with it goes through.
//
// The role forms are the first this page has, and the token reaches them from
// the request. A page that drew an empty value would still carry the field and
// still name the right address, so what says the value is the real one is the
// round trip: read the page, submit what it wrote, and be answered with the
// redirect rather than with the refusal an unchecked token earns.
//
// [Ja] TestCreate_TheListingCarriesTheTokenTheCheckCompares は、一覧がロールの
// フォームへ書き込むトークンが、送信の検証に使われるものであることを検証します。すなわち
// 同じ応答が設定した Cookie の値であり、それを添えてボタンを押せば通る、ということです。
//
// ロールのフォームはこのページが初めて持つフォームであり、トークンはリクエストから届き
// ます。空の値を描いたページであってもフィールドは運び、正しいアドレスも名指すため、値が
// 本物であることを述べるのは往復です。ページを読み、それが書いたものを送信し、突き合わせ
// られなかったトークンが受け取る拒否ではなくリダイレクトで応答されることです。
func TestCreate_TheListingCarriesTheTokenTheCheckCompares(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	adminID := buildAdministrator(t, f.db, "adminuser")
	targetID := testutil.NewUserBuilder(t, f.db).WithAtname("plainuser").Build()

	router := newRouter(f, adminID)
	body, cookie := readListing(t, router)

	matches := formCSRFTokenPattern.FindStringSubmatch(body)
	if matches == nil {
		t.Fatal("一覧のフォームに csrf_token のフィールドが無い")
	}
	token := html.UnescapeString(matches[1])
	if token == "" || token != cookie.Value {
		t.Fatalf("一覧が書き出したトークン = %q, want CSRF Cookie の %q", token, cookie.Value)
	}

	form := grantForm(model.RoleNameAdmin, "", "")
	form.Set("csrf_token", token)
	rec := submitWith(router, rolesPath(targetID), form, cookie)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if !holdsAdmin(t, f.db, targetID) {
		t.Error("一覧が描いたフォームのトークンでの送信で admin ロールが渡っていない")
	}
}
