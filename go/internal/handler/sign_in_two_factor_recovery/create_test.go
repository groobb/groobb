package sign_in_two_factor_recovery_test

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

// postCreate builds a POST /sign_in/two_factor/recovery request carrying the
// recovery code and optional returnTo as form data and pendingUserID in the
// pending cookie, with the locale set in its context.
//
// [Ja] postCreate はリカバリーコードと任意の returnTo をフォームデータとして、pendingUserID を
// pending Cookie に載せた POST /sign_in/two_factor/recovery リクエストを組み立て、context に
// ロケールを設定する。
func postCreate(pendingUserID, code, returnTo string, locale model.Locale) *http.Request {
	form := url.Values{"code": {code}}
	if returnTo != "" {
		form.Set("return_to", returnTo)
	}
	req := httptest.NewRequest(http.MethodPost, "/sign_in/two_factor/recovery", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: session.TwoFactorPendingCookieName, Value: pendingUserID})
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

// TestCreate_Success verifies that a stored recovery code completes sign-in: the
// session cookie is set, the pending cookie is cleared, and the response redirects
// to the home page.
//
// [Ja] TestCreate_Success は、保存済みリカバリーコードがサインインを完了させることを検証する。
// セッション Cookie が設定され、pending Cookie が消去され、レスポンスがホームへ
// リダイレクトする。
func TestCreate_Success(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler := newSignInTwoFactorRecoveryHandler(t, db)
	userID := seedUserWithRecoveryCodes(t, db, seededHandlerRecoveryCodes)

	rec := httptest.NewRecorder()
	handler.Create(rec, postCreate(twoFactorPendingToken(t, userID), "abcd1234", "", model.LocaleJa))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/home" {
		t.Errorf("Location = %q, want %q", loc, "/home")
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

// TestCreate_UnsignedNumericCookieCannotCompleteSignIn verifies that a known user
// id and valid recovery code cannot complete sign-in without the signed token from
// the preceding password step.
//
// [Ja] TestCreate_UnsignedNumericCookieCannotCompleteSignIn は、既知の user id と正しい
// リカバリーコードがあっても、直前のパスワードステップが発行する署名付き token が無ければ
// サインインを完了できないことを検証する。
func TestCreate_UnsignedNumericCookieCannotCompleteSignIn(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler := newSignInTwoFactorRecoveryHandler(t, db)
	userID := seedUserWithRecoveryCodes(t, db, seededHandlerRecoveryCodes)

	rec := httptest.NewRecorder()
	handler.Create(rec, postCreate(userID.String(), "abcd1234", "", model.LocaleJa))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/sign_in" {
		t.Errorf("Location = %q, want %q", loc, "/sign_in")
	}
	if findCookie(rec, session.CookieName) != nil {
		t.Error("未署名の連番 id Cookie でセッション Cookie が設定されている")
	}
}

// TestCreate_WrongCode verifies that a well-formed but unknown recovery code
// re-renders the form with 422 and the incorrect-code message, echoes the entered
// code back, preserves the requested destination, and issues no session.
//
// [Ja] TestCreate_WrongCode は、形式は正しいが未知のリカバリーコードがフォームを 422 と
// コード誤りのメッセージで再描画し、入力したコードと遷移先を保持し、セッションを発行しない
// ことを検証する。
func TestCreate_WrongCode(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler := newSignInTwoFactorRecoveryHandler(t, db)
	userID := seedUserWithRecoveryCodes(t, db, seededHandlerRecoveryCodes)

	// Well-formed (eight lowercase-alphanumeric) but not among the stored codes.
	//
	// [Ja] 形式は整う (8 文字の小文字英数字) が保存済みコードに含まれない。
	wrongCode := "zzzz9999"

	rec := httptest.NewRecorder()
	handler.Create(rec, postCreate(twoFactorPendingToken(t, userID), wrongCode, "/settings", model.LocaleJa))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "リカバリーコードが正しくありません") {
		t.Error("コード誤りのエラーメッセージが描画されていない")
	}
	if !strings.Contains(body, `value="`+wrongCode+`"`) {
		t.Error("入力したコードがエコーバックされていない")
	}
	if !strings.Contains(body, `name="return_to"`) {
		t.Error("再描画されたフォームに return_to フィールドが無い")
	}
	if !strings.Contains(body, `value="/settings"`) {
		t.Error("再描画されたフォームに return_to の値が保持されていない")
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

	db := testutil.SetupDB(t)

	handler := newSignInTwoFactorRecoveryHandler(t, db)
	userID := seedUserWithRecoveryCodes(t, db, seededHandlerRecoveryCodes)

	rec := httptest.NewRecorder()
	handler.Create(rec, postCreate(twoFactorPendingToken(t, userID), "abc", "", model.LocaleJa))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `aria-invalid="true"`) {
		t.Error("形式エラー時の入力欄に aria-invalid='true' が無い")
	}
	if !strings.Contains(body, "リカバリーコードは 8 文字の英数字で入力してください") {
		t.Error("形式エラーのメッセージが描画されていない")
	}
}

// TestCreate_NoEnabledTwoFactor verifies that a pending cookie whose user has no
// enabled 2FA (a stale or forged cookie) fails with 422 and the form-wide
// challenge-invalid message, issuing no session.
//
// [Ja] TestCreate_NoEnabledTwoFactor は、有効な 2FA を持たないユーザーの pending Cookie
// (失効・偽造した Cookie) が 422 とフォーム全体のチャレンジ無効メッセージで失敗し、
// セッションを発行しないことを検証する。
func TestCreate_NoEnabledTwoFactor(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler := newSignInTwoFactorRecoveryHandler(t, db)

	rec := httptest.NewRecorder()
	// A random user id that has no enabled 2FA setting: the challenge cannot succeed.
	// The code is well-formed so it reaches the enabled-2FA lookup rather than
	// failing on format first.
	//
	// [Ja] 有効な 2FA 設定を持たないランダムなユーザー id: チャレンジは成功しえない。
	// コードは形式が整っているため、先に形式で失敗せず有効な 2FA のルックアップまで到達する。
	handler.Create(rec, postCreate(twoFactorPendingToken(t, model.UserID(testutil.UnusedID)), "abcd1234", "", model.LocaleJa))

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

// TestCreate_NoCookieRedirectsToSignIn verifies that POST
// /sign_in/two_factor/recovery without the pending cookie redirects to sign-in,
// since there is no pending challenge to complete, and that the redirect keeps the
// destination when the form carried one.
//
// [Ja] TestCreate_NoCookieRedirectsToSignIn は、pending Cookie の無い
// POST /sign_in/two_factor/recovery がサインインへリダイレクトすること (完了すべき保留中の
// チャレンジが無いため)、そしてフォームが遷移先を運んでいたときはリダイレクトがそれを保つ
// ことを検証する。
func TestCreate_NoCookieRedirectsToSignIn(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler := newSignInTwoFactorRecoveryHandler(t, db)

	tests := []struct {
		name         string
		returnTo     string
		wantLocation string
	}{
		{
			name:         "遷移先なし",
			returnTo:     "",
			wantLocation: "/sign_in",
		},
		// The visitor's destination has not changed just because the challenge was
		// lost, so the restart carries it instead of dropping them on the home page.
		//
		// [Ja] チャレンジが失われても訪問者の目的の画面は変わらないため、やり直しでも遷移先を
		// 運び、ホームに着地させない。
		{
			name:         "遷移先あり",
			returnTo:     "/settings",
			wantLocation: "/sign_in?return_to=%2Fsettings",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			form := url.Values{"code": {"abcd1234"}}
			if tt.returnTo != "" {
				form.Set("return_to", tt.returnTo)
			}
			req := httptest.NewRequest(http.MethodPost, "/sign_in/two_factor/recovery", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req = req.WithContext(i18n.SetLocale(req.Context(), model.LocaleJa))
			rec := httptest.NewRecorder()

			handler.Create(rec, req)

			if rec.Code != http.StatusSeeOther {
				t.Fatalf("status code = %d, want %d", rec.Code, http.StatusSeeOther)
			}
			if loc := rec.Header().Get("Location"); loc != tt.wantLocation {
				t.Errorf("Location = %q, want %q", loc, tt.wantLocation)
			}
		})
	}
}

// TestCreate_ReturnTo verifies that completing the recovery-code challenge lands
// the user on the destination the flow carried since the password step, and falls
// back to the home page for a destination naming another origin.
//
// [Ja] TestCreate_ReturnTo は、リカバリーコードチャレンジの完了がパスワードのステップから
// 運ばれてきた遷移先へユーザーを着地させること、そして別オリジンを指す遷移先では
// ホームへフォールバックすることを検証する。
func TestCreate_ReturnTo(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	tests := []struct {
		name         string
		returnTo     string
		wantLocation string
	}{
		{name: "同一オリジンの相対パスへ戻す", returnTo: "/settings", wantLocation: "/settings"},
		{name: "別オリジンを指す値はホームへフォールバックする", returnTo: "https://evil.example.com", wantLocation: "/home"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler := newSignInTwoFactorRecoveryHandler(t, db)
			userID := seedUserWithRecoveryCodes(t, db, seededHandlerRecoveryCodes)

			form := url.Values{"code": {"abcd1234"}, "return_to": {tt.returnTo}}
			req := httptest.NewRequest(http.MethodPost, "/sign_in/two_factor/recovery", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.AddCookie(&http.Cookie{Name: session.TwoFactorPendingCookieName, Value: twoFactorPendingToken(t, userID)})
			req = req.WithContext(i18n.SetLocale(req.Context(), model.LocaleJa))
			rec := httptest.NewRecorder()

			handler.Create(rec, req)

			if rec.Code != http.StatusSeeOther {
				t.Fatalf("status code = %d, want %d", rec.Code, http.StatusSeeOther)
			}
			if loc := rec.Header().Get("Location"); loc != tt.wantLocation {
				t.Errorf("Location = %q, want %q", loc, tt.wantLocation)
			}
			if sessionCookie := findCookie(rec, session.CookieName); sessionCookie == nil || sessionCookie.Value == "" {
				t.Error("サインイン完了後にセッション Cookie が設定されていない")
			}
		})
	}
}
