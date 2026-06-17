package account_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/query"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/testutil"
)

// postAccount builds a POST /account request carrying the password fields as
// form data, attaching the handoff cookie when confirmationID is non-empty, with
// the locale set in its context.
//
// [Ja] postAccount は password フィールドをフォームデータとして運ぶ POST /account
// リクエストを組み立て、confirmationID が空でなければ受け渡し Cookie を付け、context に
// ロケールを設定する。
func postAccount(confirmationID, password, passwordConfirmation, locale string) *http.Request {
	form := url.Values{"password": {password}, "password_confirmation": {passwordConfirmation}}
	req := httptest.NewRequest(http.MethodPost, "/account", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if confirmationID != "" {
		req.AddCookie(&http.Cookie{Name: session.EmailConfirmationCookieName, Value: confirmationID})
	}
	return req.WithContext(i18n.SetLocale(req.Context(), locale))
}

// seedSucceededConfirmation creates and stamps a sign-up confirmation as
// succeeded (committed) for the given email, returning its id so a handler test
// can drive account creation from a verified confirmation.
//
// [Ja] seedSucceededConfirmation は指定 email のサインアップ確認を作成し成功済みとして
// 打刻 (コミット) し、ハンドラーテストが検証済みの確認からアカウント作成を駆動できるよう
// その id を返す。
func seedSucceededConfirmation(t *testing.T, email string) model.EmailConfirmationID {
	t.Helper()

	ctx := context.Background()
	repo := repository.NewEmailConfirmationRepository(query.New(testutil.GetTestDB()))
	confirmation, err := repo.Create(ctx, repository.CreateEmailConfirmationInput{
		Email: email,
		Event: model.EmailConfirmationEventSignUp,
		Code:  "123456",
	})
	if err != nil {
		t.Fatalf("確認の作成に失敗: %v", err)
	}
	if err := repo.Succeed(ctx, confirmation.ID); err != nil {
		t.Fatalf("確認の成功打刻に失敗: %v", err)
	}
	return confirmation.ID
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

// TestCreate_Success verifies that a verified confirmation plus a valid password
// creates the user, signs them in (a session cookie is set), clears the handoff
// cookie, and redirects to the top page.
//
// [Ja] TestCreate_Success は、検証済みの確認と有効なパスワードが、ユーザーを作成し、
// サインインさせ (セッション Cookie を設定)、受け渡し Cookie を消去し、トップページへ
// リダイレクトすることを検証する。
func TestCreate_Success(t *testing.T) {
	t.Parallel()

	handler := newAccountHandler(t)
	email := fmt.Sprintf("acct-h-success-%s@example.com", uuid.NewString())
	id := seedSucceededConfirmation(t, email)

	rec := httptest.NewRecorder()
	handler.Create(rec, postAccount(id.String(), "password123", "password123", i18n.LangJa))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/" {
		t.Errorf("Location = %q, want %q", loc, "/")
	}

	sessionCookie := findCookie(rec, session.CookieName)
	if sessionCookie == nil || sessionCookie.Value == "" {
		t.Error("サインイン後にセッション Cookie が設定されていない")
	}
	if ecCookie := findCookie(rec, session.EmailConfirmationCookieName); ecCookie == nil || ecCookie.MaxAge >= 0 {
		t.Error("受け渡し Cookie が消去されていない")
	}

	user, err := repository.NewUserRepository(query.New(testutil.GetTestDB())).FindByEmail(context.Background(), email)
	if err != nil {
		t.Fatalf("FindByEmail() error = %v", err)
	}
	if user == nil {
		t.Error("アカウント作成後にユーザーが永続化されていない")
	}
}

// TestCreate_NoCookieRedirectsToSignUp verifies that POST /account without the
// handoff cookie redirects to sign-up, since there is no confirmation to build
// the account from.
//
// [Ja] TestCreate_NoCookieRedirectsToSignUp は、受け渡し Cookie の無い POST /account が
// サインアップへリダイレクトすることを検証する。アカウント作成の元となる確認が無い
// ためである。
func TestCreate_NoCookieRedirectsToSignUp(t *testing.T) {
	t.Parallel()

	handler := newAccountHandler(t)

	rec := httptest.NewRecorder()
	handler.Create(rec, postAccount("", "password123", "password123", i18n.LangJa))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/sign_up" {
		t.Errorf("Location = %q, want %q", loc, "/sign_up")
	}
}

// TestCreate_ValidationError verifies that a mismatched password confirmation
// re-renders the form with 422 and the mismatch message, even though the
// confirmation is verified.
//
// [Ja] TestCreate_ValidationError は、確認が検証済みでも、パスワード確認の不一致が
// フォームを 422 と不一致メッセージで再描画することを検証する。
func TestCreate_ValidationError(t *testing.T) {
	t.Parallel()

	handler := newAccountHandler(t)
	email := fmt.Sprintf("acct-h-badpw-%s@example.com", uuid.NewString())
	id := seedSucceededConfirmation(t, email)

	rec := httptest.NewRecorder()
	handler.Create(rec, postAccount(id.String(), "password123", "different456", i18n.LangJa))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "パスワードが一致しません") {
		t.Error("不一致のエラーメッセージが描画されていない")
	}
	if !strings.Contains(body, `aria-invalid="true"`) {
		t.Error("エラー時の入力欄に aria-invalid='true' が無い")
	}
}

// TestCreate_StaleConfirmationRedirectsToSignUp verifies that a handoff cookie
// pointing at no verified confirmation (here, a random id) clears the stale
// cookie and redirects to sign-up to start over.
//
// [Ja] TestCreate_StaleConfirmationRedirectsToSignUp は、検証済みの確認を指さない
// 受け渡し Cookie (ここではランダムな id) が、失効した Cookie を消去しサインアップの
// やり直しへリダイレクトすることを検証する。
func TestCreate_StaleConfirmationRedirectsToSignUp(t *testing.T) {
	t.Parallel()

	handler := newAccountHandler(t)

	rec := httptest.NewRecorder()
	handler.Create(rec, postAccount(uuid.NewString(), "password123", "password123", i18n.LangJa))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/sign_up" {
		t.Errorf("Location = %q, want %q", loc, "/sign_up")
	}
	if ecCookie := findCookie(rec, session.EmailConfirmationCookieName); ecCookie == nil || ecCookie.MaxAge >= 0 {
		t.Error("失効した受け渡し Cookie が消去されていない")
	}
}
