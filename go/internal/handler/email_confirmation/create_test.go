package email_confirmation_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/testutil"
)

// postEmailConfirmationは指定したコードをフォームデータとして運ぶ
// POST /email_confirmationリクエストを組み立て、confirmationIDが空でなければ受け渡し
// Cookieを付け、contextにロケールを設定する。
func postEmailConfirmation(confirmationID, code string, locale model.Locale) *http.Request {
	form := url.Values{"code": {code}}
	req := httptest.NewRequest(http.MethodPost, "/email_confirmation", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if confirmationID != "" {
		req.AddCookie(&http.Cookie{Name: session.EmailConfirmationCookieName, Value: confirmationID})
	}
	return req.WithContext(i18n.SetLocale(req.Context(), locale))
}

// TestCreate_Successは、正しいコードがアカウント作成 (次のステップ) へ
// リダイレクトすることを検証する。検証済みフローはそこへ続く。
func TestCreate_Success(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler := newEmailConfirmationHandler(t, db)

	id := seedActiveConfirmation(t, db, "123456")

	rec := httptest.NewRecorder()
	handler.Create(rec, postEmailConfirmation(emailConfirmationToken(t, id), "123456", model.LocaleJa))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/account/new" {
		t.Errorf("Location = %q、期待値 = %q", loc, "/account/new")
	}
}

// TestCreate_WrongCodeは、形式は正しいが一致しないコードがフォームを422と
// フォーム全体の「不正または期限切れ」メッセージで再描画し、入力したコードをエコー
// バックすることを検証する。
func TestCreate_WrongCode(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler := newEmailConfirmationHandler(t, db)

	id := seedActiveConfirmation(t, db, "123456")

	rec := httptest.NewRecorder()
	handler.Create(rec, postEmailConfirmation(emailConfirmationToken(t, id), "000000", model.LocaleJa))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusUnprocessableEntity)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "確認コードが正しくないか、有効期限が切れています") {
		t.Error("フォーム全体のエラーメッセージが描画されていない")
	}
	if !strings.Contains(body, `value="000000"`) {
		t.Error("入力したコードがエコーバックされていない")
	}
	if !strings.Contains(body, `role="alert"`) {
		t.Error("フォーム全体のエラーにrole='alert' が無い")
	}
}

// TestCreate_InvalidFormatは、形式が不正なコードがフォームを422と、code入力欄の
// アクセシブルなフィールドエラー (aria-invalid) 付きで再描画することを検証する。
func TestCreate_InvalidFormat(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler := newEmailConfirmationHandler(t, db)

	id := seedActiveConfirmation(t, db, "123456")

	rec := httptest.NewRecorder()
	handler.Create(rec, postEmailConfirmation(emailConfirmationToken(t, id), "abc", model.LocaleJa))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusUnprocessableEntity)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `aria-invalid="true"`) {
		t.Error("形式エラー時の入力欄にaria-invalid='true' が無い")
	}
	if !strings.Contains(body, "確認コードは6桁の数字で入力してください") {
		t.Error("形式エラーのメッセージが描画されていない")
	}
}

// TestCreate_NoCookieRedirectsToSignUpは、受け渡しCookieの無い
// POST /email_confirmationがサインアップへリダイレクトすることを検証する。検証対象の
// 確認が無いためである。
func TestCreate_NoCookieRedirectsToSignUp(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler := newEmailConfirmationHandler(t, db)

	rec := httptest.NewRecorder()
	handler.Create(rec, postEmailConfirmation("", "123456", model.LocaleJa))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/sign_up" {
		t.Errorf("Location = %q、期待値 = %q", loc, "/sign_up")
	}
}
