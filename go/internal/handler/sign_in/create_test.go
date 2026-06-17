package sign_in_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/groobb/groobb/go/internal/auth"
	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/handler/sign_in"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/query"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/validator"
)

// newSignInHandler wires a sign-in Handler over the shared pool. The sign-in
// flow looks up the user and creates a session through the pool, so its tests
// commit rows and use unique emails (the test database is reset by make test)
// rather than the rolled-back transaction pattern.
//
// [Ja] newSignInHandler は共有プールでサインイン Handler を組み立てます。サインイン
// フローはプール経由でユーザーを引きセッションを作るため、そのテストはロールバックされる
// トランザクションパターンではなく、行をコミットしユニークな email を使います (テスト DB は
// make test がリセットする)。
func newSignInHandler(t *testing.T) *sign_in.Handler {
	t.Helper()

	cfg := &config.Config{Env: "test"}
	queries := query.New(testutil.GetTestDB())
	userRepo := repository.NewUserRepository(queries)
	userPasswordRepo := repository.NewUserPasswordRepository(queries)
	userSessionRepo := repository.NewUserSessionRepository(queries)

	sessionMgr := session.NewManager(userRepo, cfg)
	createSignInUC := usecase.NewCreateSignInUsecase(validator.NewSignInCreateValidator(userRepo, userPasswordRepo))
	createSessionUC := usecase.NewCreateSessionUsecase(userSessionRepo)
	return sign_in.NewHandler(cfg, sessionMgr, createSignInUC, createSessionUC)
}

// seedUserWithPassword creates a committed user with the given email and a
// password credential, returning the email so a handler test can sign in as it.
//
// [Ja] seedUserWithPassword は指定 email のユーザーとパスワード資格情報をコミットして
// 作成し、ハンドラーテストがそれとしてサインインできるよう email を返す。
func seedUserWithPassword(t *testing.T, password string) string {
	t.Helper()

	ctx := context.Background()
	queries := query.New(testutil.GetTestDB())
	email := fmt.Sprintf("signin-h-%s@example.com", uuid.NewString())

	user, err := repository.NewUserRepository(queries).Create(ctx, repository.CreateUserInput{
		Email:    email,
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

// postSignIn builds a POST /sign_in request carrying email and password as form
// data, with the locale set in its context.
//
// [Ja] postSignIn は email と password をフォームデータとして運ぶ POST /sign_in
// リクエストを組み立て、context にロケールを設定する。
func postSignIn(email, password, locale string) *http.Request {
	form := url.Values{"email": {email}, "password": {password}}
	req := httptest.NewRequest(http.MethodPost, "/sign_in", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req.WithContext(i18n.SetLocale(req.Context(), locale))
}

// findCookie returns the cookie with the given name from the response, or nil.
//
// [Ja] findCookie はレスポンスから指定名の Cookie を返す。無ければ nil。
func findCookie(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// TestCreate_Success verifies that valid credentials sign the user in (a session
// cookie is set) and redirect to the top page.
//
// [Ja] TestCreate_Success は、有効な資格情報がユーザーをサインインさせ (セッション
// Cookie を設定)、トップページへリダイレクトすることを検証する。
func TestCreate_Success(t *testing.T) {
	t.Parallel()

	handler := newSignInHandler(t)
	email := seedUserWithPassword(t, "password123")

	rec := httptest.NewRecorder()
	handler.Create(rec, postSignIn(email, "password123", i18n.LangJa))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/" {
		t.Errorf("Location = %q, want %q", loc, "/")
	}
	if sessionCookie := findCookie(rec, session.CookieName); sessionCookie == nil || sessionCookie.Value == "" {
		t.Error("サインイン後にセッション Cookie が設定されていない")
	}
}

// TestCreate_WrongPassword verifies that a wrong password re-renders the form
// with 422 and the generic credentials message, without setting a session cookie.
//
// [Ja] TestCreate_WrongPassword は、誤ったパスワードがフォームを 422 と汎用の資格情報
// メッセージで再描画し、セッション Cookie を設定しないことを検証する。
func TestCreate_WrongPassword(t *testing.T) {
	t.Parallel()

	handler := newSignInHandler(t)
	email := seedUserWithPassword(t, "password123")

	rec := httptest.NewRecorder()
	handler.Create(rec, postSignIn(email, "wrongpassword", i18n.LangJa))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	if !strings.Contains(rec.Body.String(), "メールアドレスかパスワードが正しくありません") {
		t.Error("資格情報エラーのメッセージが描画されていない")
	}
	// The submitted email is echoed back so the user does not retype it; the
	// password is not.
	//
	// [Ja] 送信された email はユーザーが再入力せずに済むようエコーバックされる。パスワードは
	// されない。
	if !strings.Contains(rec.Body.String(), email) {
		t.Error("入力した email が再描画フォームにエコーバックされていない")
	}
	if findCookie(rec, session.CookieName) != nil {
		t.Error("資格情報エラーなのにセッション Cookie が設定されている")
	}
}

// TestCreate_MissingEmail verifies that an empty email re-renders the form with
// 422 and a field-level required error.
//
// [Ja] TestCreate_MissingEmail は、空の email がフォームを 422 とフィールド単位の必須
// エラーで再描画することを検証する。
func TestCreate_MissingEmail(t *testing.T) {
	t.Parallel()

	handler := newSignInHandler(t)

	rec := httptest.NewRecorder()
	handler.Create(rec, postSignIn("", "password123", i18n.LangJa))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	if !strings.Contains(rec.Body.String(), `aria-invalid="true"`) {
		t.Error("エラー時の入力欄に aria-invalid='true' が無い")
	}
}
