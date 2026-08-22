package email_confirmation_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/testutil"
)

// postEmailConfirmation builds a POST /email_confirmation request carrying the
// given code as form data, attaching the handoff cookie when confirmationID is
// non-empty, with the locale set in its context.
//
// [Ja] postEmailConfirmation は指定したコードをフォームデータとして運ぶ
// POST /email_confirmation リクエストを組み立て、confirmationID が空でなければ受け渡し
// Cookie を付け、context にロケールを設定する。
func postEmailConfirmation(confirmationID, code, locale string) *http.Request {
	form := url.Values{"code": {code}}
	req := httptest.NewRequest(http.MethodPost, "/email_confirmation", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if confirmationID != "" {
		req.AddCookie(&http.Cookie{Name: session.EmailConfirmationCookieName, Value: confirmationID})
	}
	return req.WithContext(i18n.SetLocale(req.Context(), locale))
}

// TestCreate_Success verifies that a correct code redirects to account creation
// (the next step), which is where the verified flow continues.
//
// [Ja] TestCreate_Success は、正しいコードがアカウント作成 (次のステップ) へ
// リダイレクトすることを検証する。検証済みフローはそこへ続く。
func TestCreate_Success(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler := newEmailConfirmationHandler(t, db)

	id := seedActiveConfirmation(t, db, "123456")

	rec := httptest.NewRecorder()
	handler.Create(rec, postEmailConfirmation(emailConfirmationToken(t, id), "123456", i18n.LangJa))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/account/new" {
		t.Errorf("Location = %q, want %q", loc, "/account/new")
	}
}

// TestCreate_WrongCode verifies that a well-formed but non-matching code
// re-renders the form with 422 and the form-wide incorrect-or-expired message,
// echoing the entered code back.
//
// [Ja] TestCreate_WrongCode は、形式は正しいが一致しないコードがフォームを 422 と
// フォーム全体の「不正または期限切れ」メッセージで再描画し、入力したコードをエコー
// バックすることを検証する。
func TestCreate_WrongCode(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler := newEmailConfirmationHandler(t, db)

	id := seedActiveConfirmation(t, db, "123456")

	rec := httptest.NewRecorder()
	handler.Create(rec, postEmailConfirmation(emailConfirmationToken(t, id), "000000", i18n.LangJa))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "確認コードが正しくないか、有効期限が切れています") {
		t.Error("フォーム全体のエラーメッセージが描画されていない")
	}
	if !strings.Contains(body, `value="000000"`) {
		t.Error("入力したコードがエコーバックされていない")
	}
	if !strings.Contains(body, `role="alert"`) {
		t.Error("フォーム全体のエラーに role='alert' が無い")
	}
}

// TestCreate_InvalidFormat verifies that a malformed code re-renders the form
// with 422 and an accessible field error on the code input (aria-invalid).
//
// [Ja] TestCreate_InvalidFormat は、形式が不正なコードがフォームを 422 と、code 入力欄の
// アクセシブルなフィールドエラー (aria-invalid) 付きで再描画することを検証する。
func TestCreate_InvalidFormat(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler := newEmailConfirmationHandler(t, db)

	id := seedActiveConfirmation(t, db, "123456")

	rec := httptest.NewRecorder()
	handler.Create(rec, postEmailConfirmation(emailConfirmationToken(t, id), "abc", i18n.LangJa))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `aria-invalid="true"`) {
		t.Error("形式エラー時の入力欄に aria-invalid='true' が無い")
	}
	if !strings.Contains(body, "確認コードは 6 桁の数字で入力してください") {
		t.Error("形式エラーのメッセージが描画されていない")
	}
}

// TestCreate_NoCookieRedirectsToSignUp verifies that POST /email_confirmation
// without the handoff cookie redirects to sign-up, since there is no confirmation
// to verify against.
//
// [Ja] TestCreate_NoCookieRedirectsToSignUp は、受け渡し Cookie の無い
// POST /email_confirmation がサインアップへリダイレクトすることを検証する。検証対象の
// 確認が無いためである。
func TestCreate_NoCookieRedirectsToSignUp(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler := newEmailConfirmationHandler(t, db)

	rec := httptest.NewRecorder()
	handler.Create(rec, postEmailConfirmation("", "123456", i18n.LangJa))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/sign_up" {
		t.Errorf("Location = %q, want %q", loc, "/sign_up")
	}
}
