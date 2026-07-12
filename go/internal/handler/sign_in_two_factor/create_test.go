package sign_in_two_factor_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/testutil"
)

// postCreate builds a POST /sign_in/two_factor request carrying the TOTP code as
// form data, attaching the pending cookie when pendingUserID is non-empty, with the
// locale set in its context.
//
// [Ja] postCreate は TOTP コードをフォームデータとして運ぶ POST /sign_in/two_factor
// リクエストを組み立て、pendingUserID が空でなければ pending Cookie を付け、context に
// ロケールを設定する。
func postCreate(pendingUserID, code, locale string) *http.Request {
	form := url.Values{"code": {code}}
	req := httptest.NewRequest(http.MethodPost, "/sign_in/two_factor", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if pendingUserID != "" {
		req.AddCookie(&http.Cookie{Name: session.TwoFactorPendingCookieName, Value: pendingUserID})
	}
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

// TestCreate_Success verifies that a correct TOTP code completes sign-in: the
// session cookie is set, the pending cookie is cleared, and the response redirects
// to the top page.
//
// [Ja] TestCreate_Success は、正しい TOTP コードがサインインを完了させることを検証する。
// セッション Cookie が設定され、pending Cookie が消去され、レスポンスがトップページへ
// リダイレクトする。
func TestCreate_Success(t *testing.T) {
	t.Parallel()

	handler := newSignInTwoFactorHandler(t)
	userID := seedUserWithEnabledTwoFactor(t)

	code, err := totp.GenerateCode(testutil.DefaultBuilderTOTPSecret, time.Now())
	if err != nil {
		t.Fatalf("テスト用 TOTP コードの生成に失敗: %v", err)
	}

	rec := httptest.NewRecorder()
	handler.Create(rec, postCreate(userID.String(), code, i18n.LangJa))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/" {
		t.Errorf("Location = %q, want %q", loc, "/")
	}
	if sessionCookie := findCookie(rec, session.CookieName); sessionCookie == nil || sessionCookie.Value == "" {
		t.Error("サインイン完了後にセッション Cookie が設定されていない")
	}
	// The completed challenge clears the pending cookie (MaxAge < 0 = deletion).
	//
	// [Ja] 完了したチャレンジは pending Cookie を消去する (MaxAge < 0 = 削除)。
	pending := findCookie(rec, session.TwoFactorPendingCookieName)
	if pending == nil || pending.MaxAge >= 0 {
		t.Error("完了後に pending Cookie が消去されていない")
	}
}

// TestCreate_WrongCode verifies that a well-formed but non-matching code re-renders
// the form with 422 and the incorrect-code message, echoes the entered code back,
// and issues no session.
//
// [Ja] TestCreate_WrongCode は、形式は正しいが一致しないコードがフォームを 422 とコード
// 誤りのメッセージで再描画し、入力したコードをエコーバックし、セッションを発行しないことを
// 検証する。
func TestCreate_WrongCode(t *testing.T) {
	t.Parallel()

	handler := newSignInTwoFactorHandler(t)
	userID := seedUserWithEnabledTwoFactor(t)

	validCode, err := totp.GenerateCode(testutil.DefaultBuilderTOTPSecret, time.Now())
	if err != nil {
		t.Fatalf("テスト用 TOTP コードの生成に失敗: %v", err)
	}
	wrongCode := "000000"
	if wrongCode == validCode {
		wrongCode = "111111"
	}

	rec := httptest.NewRecorder()
	handler.Create(rec, postCreate(userID.String(), wrongCode, i18n.LangJa))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "認証コードが正しくありません") {
		t.Error("コード誤りのエラーメッセージが描画されていない")
	}
	if !strings.Contains(body, `value="`+wrongCode+`"`) {
		t.Error("入力したコードがエコーバックされていない")
	}
	if findCookie(rec, session.CookieName) != nil {
		t.Error("コード誤りなのにセッション Cookie が設定されている")
	}
}

// TestCreate_InvalidFormat verifies that a malformed code re-renders the form with
// 422 and an accessible field error on the code input (aria-invalid).
//
// [Ja] TestCreate_InvalidFormat は、形式が不正なコードがフォームを 422 と、code 入力欄の
// アクセシブルなフィールドエラー (aria-invalid) 付きで再描画することを検証する。
func TestCreate_InvalidFormat(t *testing.T) {
	t.Parallel()

	handler := newSignInTwoFactorHandler(t)
	userID := seedUserWithEnabledTwoFactor(t)

	rec := httptest.NewRecorder()
	handler.Create(rec, postCreate(userID.String(), "abc", i18n.LangJa))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `aria-invalid="true"`) {
		t.Error("形式エラー時の入力欄に aria-invalid='true' が無い")
	}
	if !strings.Contains(body, "認証コードは 6 桁の数字で入力してください") {
		t.Error("形式エラーのメッセージが描画されていない")
	}
}

// TestCreate_NoEnabledTwoFactor verifies that a pending cookie whose user has no
// enabled 2FA (a stale or forged cookie) fails with 422 and the form-wide
// challenge-invalid message, issuing no session.
//
// [Ja] TestCreate_NoEnabledTwoFactor は、有効な 2FA を持たないユーザーの pending Cookie
// (失効・偽造した Cookie) が 422 とフォーム全体のチャレンジ無効メッセージで失敗し、セッションを
// 発行しないことを検証する。
func TestCreate_NoEnabledTwoFactor(t *testing.T) {
	t.Parallel()

	handler := newSignInTwoFactorHandler(t)

	rec := httptest.NewRecorder()
	// A random user id that has no enabled 2FA setting: the challenge cannot succeed.
	//
	// [Ja] 有効な 2FA 設定を持たないランダムなユーザー id: チャレンジは成功しえない。
	handler.Create(rec, postCreate(uuid.NewString(), "123456", i18n.LangJa))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	if !strings.Contains(rec.Body.String(), "2 段階認証を完了できませんでした") {
		t.Error("チャレンジ無効のフォーム全体メッセージが描画されていない")
	}
	if findCookie(rec, session.CookieName) != nil {
		t.Error("有効な 2FA が無いのにセッション Cookie が設定されている")
	}
}

// TestCreate_NoCookieRedirectsToSignIn verifies that POST /sign_in/two_factor
// without the pending cookie redirects to sign-in, since there is no pending
// challenge to complete.
//
// [Ja] TestCreate_NoCookieRedirectsToSignIn は、pending Cookie の無い
// POST /sign_in/two_factor がサインインへリダイレクトすることを検証する。完了すべき保留中の
// チャレンジが無いためである。
func TestCreate_NoCookieRedirectsToSignIn(t *testing.T) {
	t.Parallel()

	handler := newSignInTwoFactorHandler(t)

	rec := httptest.NewRecorder()
	handler.Create(rec, postCreate("", "123456", i18n.LangJa))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/sign_in" {
		t.Errorf("Location = %q, want %q", loc, "/sign_in")
	}
}
