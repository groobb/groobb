package sign_in_two_factor_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/testutil"
)

// postCreateはTOTPコードと任意のreturnToをフォームデータとして、pendingUserIDを
// pending Cookieに載せたPOST /sign_in/two_factorリクエストを組み立て、contextに
// ロケールを設定する。
func postCreate(pendingUserID, code, returnTo string, locale model.Locale) *http.Request {
	form := url.Values{"code": {code}}
	if returnTo != "" {
		form.Set("return_to", returnTo)
	}
	req := httptest.NewRequest(http.MethodPost, "/sign_in/two_factor", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: session.TwoFactorPendingCookieName, Value: pendingUserID})
	return req.WithContext(i18n.SetLocale(req.Context(), locale))
}

// findCookieはレスポンスから指定名のCookieを返す。無ければnil。
func findCookie(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// TestCreate_Successは、正しいTOTPコードがサインインを完了させることを検証する。
// セッションCookieが設定され、pending Cookieが消去され、レスポンスがホームへ
// リダイレクトする。
func TestCreate_Success(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler := newSignInTwoFactorHandler(t, db)
	userID := seedUserWithEnabledTwoFactor(t, db)

	code, err := totp.GenerateCode(testutil.DefaultBuilderTOTPSecret, time.Now())
	if err != nil {
		t.Fatalf("テスト用TOTPコードの生成に失敗: %v", err)
	}

	rec := httptest.NewRecorder()
	handler.Create(rec, postCreate(twoFactorPendingToken(t, userID), code, "", model.LocaleJa))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/home" {
		t.Errorf("Location = %q、期待値 = %q", loc, "/home")
	}
	if sessionCookie := findCookie(rec, session.CookieName); sessionCookie == nil || sessionCookie.Value == "" {
		t.Error("サインイン完了後にセッションCookieが設定されていない")
	}
	// 完了したチャレンジはpending Cookieを消去する (MaxAge < 0 = 削除)。
	pending := findCookie(rec, session.TwoFactorPendingCookieName)
	if pending == nil || pending.MaxAge >= 0 {
		t.Error("完了後にpending Cookieが消去されていない")
	}
}

// TestCreate_UnsignedNumericCookieCannotCompleteSignInは、ユーザーの連番idと現在の
// 正しいTOTPコードを知っていても、直前のパスワードステップが発行する署名付き受け渡し
// tokenが無ければサインインを完了できないことを検証する。
func TestCreate_UnsignedNumericCookieCannotCompleteSignIn(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler := newSignInTwoFactorHandler(t, db)
	userID := seedUserWithEnabledTwoFactor(t, db)

	code, err := totp.GenerateCode(testutil.DefaultBuilderTOTPSecret, time.Now())
	if err != nil {
		t.Fatalf("テスト用TOTPコードの生成に失敗: %v", err)
	}

	rec := httptest.NewRecorder()
	handler.Create(rec, postCreate(userID.String(), code, "", model.LocaleJa))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/sign_in" {
		t.Errorf("Location = %q、期待値 = %q", loc, "/sign_in")
	}
	if findCookie(rec, session.CookieName) != nil {
		t.Error("未署名の連番id CookieでセッションCookieが設定されている")
	}
}

// TestCreate_WrongCodeは、形式は正しいが一致しないコードがフォームを422とコード
// 誤りのメッセージで再描画し、入力したコードと遷移先を保持し、セッションを発行しないことを
// 検証する。
func TestCreate_WrongCode(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler := newSignInTwoFactorHandler(t, db)
	userID := seedUserWithEnabledTwoFactor(t, db)

	validCode, err := totp.GenerateCode(testutil.DefaultBuilderTOTPSecret, time.Now())
	if err != nil {
		t.Fatalf("テスト用TOTPコードの生成に失敗: %v", err)
	}
	wrongCode := "000000"
	if wrongCode == validCode {
		wrongCode = "111111"
	}

	rec := httptest.NewRecorder()
	handler.Create(rec, postCreate(twoFactorPendingToken(t, userID), wrongCode, "/settings", model.LocaleJa))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusUnprocessableEntity)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "認証コードが正しくありません") {
		t.Error("コード誤りのエラーメッセージが描画されていない")
	}
	if !strings.Contains(body, `value="`+wrongCode+`"`) {
		t.Error("入力したコードがエコーバックされていない")
	}
	if !strings.Contains(body, `name="return_to"`) {
		t.Error("再描画されたフォームにreturn_toフィールドが無い")
	}
	if !strings.Contains(body, `value="/settings"`) {
		t.Error("再描画されたフォームにreturn_toの値が保持されていない")
	}
	if findCookie(rec, session.CookieName) != nil {
		t.Error("コード誤りなのにセッションCookieが設定されている")
	}
}

// TestCreate_InvalidFormatは、形式が不正なコードがフォームを422と、code入力欄の
// アクセシブルなフィールドエラー (aria-invalid) 付きで再描画することを検証する。
func TestCreate_InvalidFormat(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler := newSignInTwoFactorHandler(t, db)
	userID := seedUserWithEnabledTwoFactor(t, db)

	rec := httptest.NewRecorder()
	handler.Create(rec, postCreate(twoFactorPendingToken(t, userID), "abc", "", model.LocaleJa))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusUnprocessableEntity)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `aria-invalid="true"`) {
		t.Error("形式エラー時の入力欄にaria-invalid='true' が無い")
	}
	if !strings.Contains(body, "認証コードは6桁の数字で入力してください") {
		t.Error("形式エラーのメッセージが描画されていない")
	}
}

// TestCreate_NoEnabledTwoFactorは、有効な2FAを持たないユーザーのpending Cookie
// (失効・偽造したCookie) が422とフォーム全体のチャレンジ無効メッセージで失敗し、セッションを
// 発行しないことを検証する。
func TestCreate_NoEnabledTwoFactor(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler := newSignInTwoFactorHandler(t, db)

	rec := httptest.NewRecorder()
	// 有効な2FA設定を持たないランダムなユーザーid: チャレンジは成功しえない。
	handler.Create(rec, postCreate(twoFactorPendingToken(t, model.UserID(testutil.UnusedID)), "123456", "", model.LocaleJa))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusUnprocessableEntity)
	}
	if !strings.Contains(rec.Body.String(), "2段階認証を完了できませんでした") {
		t.Error("チャレンジ無効のフォーム全体メッセージが描画されていない")
	}
	if findCookie(rec, session.CookieName) != nil {
		t.Error("有効な2FAが無いのにセッションCookieが設定されている")
	}
}

// TestCreate_NoCookieRedirectsToSignInは、pending Cookieの無い
// POST /sign_in/two_factorがサインインへリダイレクトすること (完了すべき保留中の
// チャレンジが無いため)、そしてフォームが遷移先を運んでいたときはリダイレクトがそれを保つ
// ことを検証する。
func TestCreate_NoCookieRedirectsToSignIn(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler := newSignInTwoFactorHandler(t, db)

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
		// チャレンジが失われても訪問者の目的の画面は変わらないため、やり直しでも遷移先を
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

			form := url.Values{"code": {"123456"}}
			if tt.returnTo != "" {
				form.Set("return_to", tt.returnTo)
			}
			req := httptest.NewRequest(http.MethodPost, "/sign_in/two_factor", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req = req.WithContext(i18n.SetLocale(req.Context(), model.LocaleJa))
			rec := httptest.NewRecorder()

			handler.Create(rec, req)

			if rec.Code != http.StatusSeeOther {
				t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusSeeOther)
			}
			if loc := rec.Header().Get("Location"); loc != tt.wantLocation {
				t.Errorf("Location = %q、期待値 = %q", loc, tt.wantLocation)
			}
		})
	}
}

// TestCreate_ReturnToは、TOTPチャレンジの完了がパスワードのステップから運ばれてきた
// 遷移先へユーザーを着地させること、そして別オリジンを指す遷移先ではホームへ
// フォールバックすることを検証する。
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

			handler := newSignInTwoFactorHandler(t, db)
			userID := seedUserWithEnabledTwoFactor(t, db)

			code, err := totp.GenerateCode(testutil.DefaultBuilderTOTPSecret, time.Now())
			if err != nil {
				t.Fatalf("テスト用TOTPコードの生成に失敗: %v", err)
			}

			form := url.Values{"code": {code}, "return_to": {tt.returnTo}}
			req := httptest.NewRequest(http.MethodPost, "/sign_in/two_factor", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.AddCookie(&http.Cookie{Name: session.TwoFactorPendingCookieName, Value: twoFactorPendingToken(t, userID)})
			req = req.WithContext(i18n.SetLocale(req.Context(), model.LocaleJa))
			rec := httptest.NewRecorder()

			handler.Create(rec, req)

			if rec.Code != http.StatusSeeOther {
				t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusSeeOther)
			}
			if loc := rec.Header().Get("Location"); loc != tt.wantLocation {
				t.Errorf("Location = %q、期待値 = %q", loc, tt.wantLocation)
			}
			if sessionCookie := findCookie(rec, session.CookieName); sessionCookie == nil || sessionCookie.Value == "" {
				t.Error("サインイン完了後にセッションCookieが設定されていない")
			}
		})
	}
}
