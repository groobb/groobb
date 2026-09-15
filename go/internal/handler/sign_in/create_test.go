package sign_in_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/auth"
	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/handler/sign_in"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/validator"
)

// newSignInHandlerはテスト用データベースのリポジトリで、既定で通過するTurnstile
// 検証器を伴ってサインインHandlerを組み立て、ハンドラーテストが実DBに対して
// リクエスト経路全体 (Turnstileゲート・バリデーター・UseCase・セッションCookie) を
// 通すようにします。Turnstile検証を失敗させられるよう、検証器も併せて返します。
func newSignInHandler(t *testing.T, db *database.DB) (*sign_in.Handler, *testutil.FakeTurnstileVerifier) {
	t.Helper()

	cfg := testutil.NewTestConfig(t)
	userRepo := repository.NewUserRepository(db)
	userPasswordRepo := repository.NewUserPasswordRepository(db)
	userTwoFactorAuthRepo := repository.NewUserTwoFactorAuthRepository(db)
	userSessionRepo := repository.NewUserSessionRepository(db)

	sessionMgr := session.NewManager(userRepo, cfg)
	createSignInUC := usecase.NewCreateSignInUsecase(validator.NewSignInCreateValidator(userRepo, userPasswordRepo, userTwoFactorAuthRepo))
	createSessionUC := usecase.NewCreateSessionUsecase(userSessionRepo)
	verifier := &testutil.FakeTurnstileVerifier{Passed: true}
	return sign_in.NewHandler(cfg, sessionMgr, createSignInUC, createSessionUC, verifier), verifier
}

// seedUserWithPasswordは指定emailのユーザーとパスワード資格情報をコミットして
// 作成し、ハンドラーテストがそれとしてサインインできるようemailを返す。
func seedUserWithPassword(t *testing.T, db *database.DB, password string) string {
	t.Helper()

	ctx := context.Background()
	email := testutil.UniqueEmail(db, "signin-h")

	user, err := repository.NewUserRepository(db).Create(ctx, repository.CreateUserInput{
		Email:    email,
		Atname:   testutil.UniqueAtname(db),
		Locale:   model.LocaleJa,
		TimeZone: "Asia/Tokyo",
	})
	if err != nil {
		t.Fatalf("ユーザーの作成に失敗: %v", err)
	}

	digest, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("パスワードのハッシュ化に失敗: %v", err)
	}
	if _, err := repository.NewUserPasswordRepository(db).Create(ctx, repository.CreateUserPasswordInput{
		UserID:         user.ID,
		PasswordDigest: digest,
	}); err != nil {
		t.Fatalf("パスワード資格情報の作成に失敗: %v", err)
	}
	return email
}

// seedUserWithTwoFactorはパスワード資格情報と有効な2FA設定を持つユーザーを
// コミットして作成し、ハンドラーテストがそれとしてサインインして2段階認証チャレンジの
// 分岐へ到達できるようemailを返す。
func seedUserWithTwoFactor(t *testing.T, db *database.DB, password string) string {
	t.Helper()

	ctx := context.Background()
	email := testutil.UniqueEmail(db, "signin-h-2fa")

	user, err := repository.NewUserRepository(db).Create(ctx, repository.CreateUserInput{
		Email:    email,
		Atname:   testutil.UniqueAtname(db),
		Locale:   model.LocaleJa,
		TimeZone: "Asia/Tokyo",
	})
	if err != nil {
		t.Fatalf("ユーザーの作成に失敗: %v", err)
	}

	digest, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("パスワードのハッシュ化に失敗: %v", err)
	}
	if _, err := repository.NewUserPasswordRepository(db).Create(ctx, repository.CreateUserPasswordInput{
		UserID:         user.ID,
		PasswordDigest: digest,
	}); err != nil {
		t.Fatalf("パスワード資格情報の作成に失敗: %v", err)
	}

	twoFactorRepo := repository.NewUserTwoFactorAuthRepository(db)
	if _, err := twoFactorRepo.Create(ctx, repository.CreateUserTwoFactorAuthInput{
		UserID: user.ID,
		Secret: testutil.DefaultBuilderTOTPSecret,
	}); err != nil {
		t.Fatalf("2段階認証設定の作成に失敗: %v", err)
	}
	enabled, err := twoFactorRepo.Enable(ctx, user.ID, []string{"recoverycode1"})
	if err != nil {
		t.Fatalf("2段階認証の有効化に失敗: %v", err)
	}
	if !enabled {
		t.Fatal("2段階認証を有効化できなかった (未有効化の行が見つからない)")
	}
	return email
}

// postSignInはemailとpasswordをフォームデータとして運ぶPOST /sign_in
// リクエストを組み立て、contextにロケールを設定する。
func postSignIn(email, password string, locale model.Locale) *http.Request {
	form := url.Values{"email": {email}, "password": {password}}
	req := httptest.NewRequest(http.MethodPost, "/sign_in", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
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

// TestCreate_Successは、有効な資格情報がユーザーをサインインさせ (セッション
// Cookieを設定)、ホームへリダイレクトすることを検証する。
func TestCreate_Success(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler, _ := newSignInHandler(t, db)
	email := seedUserWithPassword(t, db, "password123")

	rec := httptest.NewRecorder()
	handler.Create(rec, postSignIn(email, "password123", model.LocaleJa))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/home" {
		t.Errorf("Location = %q、期待値 = %q", loc, "/home")
	}
	if sessionCookie := findCookie(rec, session.CookieName); sessionCookie == nil || sessionCookie.Value == "" {
		t.Error("サインイン後にセッションCookieが設定されていない")
	}
	// 2FA無しのユーザーはそのままサインインするため、pending Cookieは設定されない。
	if findCookie(rec, session.TwoFactorPendingCookieName) != nil {
		t.Error("2FA無しのサインインでpending Cookieが設定されている")
	}
}

// TestCreate_TwoFactorEnabledは、2FA有効なアカウントがパスワードだけでは
// サインインしないことを検証する。パスワードのステップは通るがCreateはセッションを
// 発行せず、短命のpending Cookieを設定し、ホームではなくTOTPチャレンジへ
// リダイレクトする。
func TestCreate_TwoFactorEnabled(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler, _ := newSignInHandler(t, db)
	email := seedUserWithTwoFactor(t, db, "password123")

	rec := httptest.NewRecorder()
	handler.Create(rec, postSignIn(email, "password123", model.LocaleJa))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/sign_in/two_factor/new" {
		t.Errorf("Location = %q、期待値 = %q", loc, "/sign_in/two_factor/new")
	}
	// パスワードは一致したが2FAが必要なため、この時点ではセッションを発行しない。
	// 代わりに保留中のユーザーをチャレンジ用の2段階認証Cookieに保持する。
	if findCookie(rec, session.CookieName) != nil {
		t.Error("2FA必須なのにセッションCookieが設定されている (パスワードだけでサインインしてしまっている)")
	}
	pending := findCookie(rec, session.TwoFactorPendingCookieName)
	if pending == nil || pending.Value == "" {
		t.Fatal("2段階認証のpending Cookieが設定されていない")
	}
}

// TestCreate_WrongPasswordは、誤ったパスワードがフォームを422と汎用の資格情報
// メッセージで再描画し、セッションCookieを設定しないことを検証する。
func TestCreate_WrongPassword(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler, _ := newSignInHandler(t, db)
	email := seedUserWithPassword(t, db, "password123")

	rec := httptest.NewRecorder()
	handler.Create(rec, postSignIn(email, "wrongpassword", model.LocaleJa))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusUnprocessableEntity)
	}
	if !strings.Contains(rec.Body.String(), "メールアドレスかパスワードが正しくありません") {
		t.Error("資格情報エラーのメッセージが描画されていない")
	}
	// 送信されたemailはユーザーが再入力せずに済むようエコーバックされる。パスワードは
	// されない。
	if !strings.Contains(rec.Body.String(), email) {
		t.Error("入力したemailが再描画フォームにエコーバックされていない")
	}
	if findCookie(rec, session.CookieName) != nil {
		t.Error("資格情報エラーなのにセッションCookieが設定されている")
	}
}

// TestCreate_MissingEmailは、空のemailがフォームを422とフィールド単位の必須
// エラーで再描画することを検証する。
func TestCreate_MissingEmail(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler, _ := newSignInHandler(t, db)

	rec := httptest.NewRecorder()
	handler.Create(rec, postSignIn("", "password123", model.LocaleJa))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusUnprocessableEntity)
	}
	if !strings.Contains(rec.Body.String(), `aria-invalid="true"`) {
		t.Error("エラー時の入力欄にaria-invalid='true' が無い")
	}
}

// TestCreate_TurnstileFailureは、Turnstile検証が通過しないとき (非通過または
// siteverifyエラー) にCreateがBotゲートでリクエストを止めることを検証する。
// フォームを422とフォーム全体のTurnstileメッセージで再描画し、emailと遷移先を
// エコーバックし、送信されたトークンを検証器へ渡し、認証もセッション発行もしないことを
// 確認する。有効な資格情報をあえて与えているため、ゲートが迂回されればユーザーは
// サインインしてしまう (303 + セッションCookie)。422とセッションCookieの不在が、
// ゲートが認証の前で走ったことを裏付ける。Botゲートは
// TestCreate_ReturnToSurvivesValidationErrorが見る資格情報チェックの再描画とは別の分岐で
// あるため、遷移先の残存もこの経路で確認する。ここで遷移先を落とすと、再試行に成功しても
// 訪問者はホームに着地してしまう。
func TestCreate_TurnstileFailure(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	tests := []struct {
		name   string
		passed bool
		err    error
	}{
		{name: "非通過", passed: false, err: nil},
		{name: "検証エラー", passed: false, err: errors.New("siteverify unavailable")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler, verifier := newSignInHandler(t, db)
			verifier.Passed = tt.passed
			verifier.Err = tt.err
			email := seedUserWithPassword(t, db, "password123")

			form := url.Values{
				"email":                 {email},
				"password":              {"password123"},
				"cf-turnstile-response": {"submitted-token"},
				"return_to":             {"/settings"},
			}
			req := httptest.NewRequest(http.MethodPost, "/sign_in", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req = req.WithContext(i18n.SetLocale(req.Context(), model.LocaleJa))

			rec := httptest.NewRecorder()
			handler.Create(rec, req)

			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusUnprocessableEntity)
			}
			body := rec.Body.String()
			// サインインフォームが再描画され、フォーム全体のTurnstileメッセージを
			// 載せていること。
			if !strings.Contains(body, `action="/sign_in"`) {
				t.Error("サインインフォームが再描画されていない")
			}
			if !strings.Contains(body, "ロボットでないことの確認に失敗しました") {
				t.Error("Turnstile失敗のフォーム全体メッセージが描画されていない")
			}
			if !strings.Contains(body, `role="alert"`) {
				t.Error("フォーム全体のエラーにrole='alert' が無い")
			}
			// ユーザーが再入力しなくて済むようemailはエコーバックされること。
			if !strings.Contains(body, email) {
				t.Error("入力したemailが再描画フォームにエコーバックされていない")
			}
			// 遷移先はBotゲートを越えて残り、再試行に成功した訪問者は向かっていた
			// 先へ着地できること。
			if !strings.Contains(body, `value="/settings"`) {
				t.Error("再描画したフォームにreturn_toが残っていない")
			}
			// 送信されたトークンが検証器へ到達しており、ハンドラーが正しい
			// cf-turnstile-responseフィールドを読んでいることを確認する。
			if verifier.Token != "submitted-token" {
				t.Errorf("verifierに渡ったトークン = %q、期待値 = %q", verifier.Token, "submitted-token")
			}
			// Botゲートは認証の前でリクエストを止めるため、ここで与えた有効な
			// 資格情報でもセッションは発行されないこと。
			if findCookie(rec, session.CookieName) != nil {
				t.Error("Turnstile失敗時にセッションCookieが設定された (認証に進んでしまっている)")
			}
		})
	}
}

// postSignInWithReturnToはreturn_toの遷移先も併せて運ぶPOST /sign_inリクエストを
// 組み立てる。訪問者が追い返されたルートから来たときにフォームが送る形と同じである。
func postSignInWithReturnTo(email, password, returnTo string, locale model.Locale) *http.Request {
	form := url.Values{"email": {email}, "password": {password}, "return_to": {returnTo}}
	req := httptest.NewRequest(http.MethodPost, "/sign_in", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req.WithContext(i18n.SetLocale(req.Context(), locale))
}

// TestCreate_ReturnToは、遷移先を伴うサインインがユーザーをホームではなく
// その遷移先へ着地させること、そして別オリジンを指す遷移先は破棄され、サインインフローが
// オープンリダイレクトとして使えないことを検証する。
func TestCreate_ReturnTo(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	tests := []struct {
		name         string
		returnTo     string
		wantLocation string
	}{
		{name: "同一オリジンの相対パスへ戻す", returnTo: "/settings", wantLocation: "/settings"},
		{name: "別オリジンを指す値はホームへフォールバックする", returnTo: "//evil.example.com/settings", wantLocation: "/home"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler, _ := newSignInHandler(t, db)
			email := seedUserWithPassword(t, db, "password123")

			rec := httptest.NewRecorder()
			handler.Create(rec, postSignInWithReturnTo(email, "password123", tt.returnTo, model.LocaleJa))

			if rec.Code != http.StatusSeeOther {
				t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusSeeOther)
			}
			if loc := rec.Header().Get("Location"); loc != tt.wantLocation {
				t.Errorf("Location = %q、期待値 = %q", loc, tt.wantLocation)
			}
			if sessionCookie := findCookie(rec, session.CookieName); sessionCookie == nil || sessionCookie.Value == "" {
				t.Error("サインイン後にセッションCookieが設定されていない")
			}
		})
	}
}

// TestCreate_TwoFactorEnabledForwardsReturnToは、2FA有効なアカウントがチャレンジの
// ホップを跨いで遷移先を保つことを検証する。TOTPフォームへのリダイレクトがreturn_toを運ぶ
// ため、コード入力フォームは最終的にセッションを発行するステップへそれを引き渡せる。
func TestCreate_TwoFactorEnabledForwardsReturnTo(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler, _ := newSignInHandler(t, db)
	email := seedUserWithTwoFactor(t, db, "password123")

	rec := httptest.NewRecorder()
	handler.Create(rec, postSignInWithReturnTo(email, "password123", "/settings", model.LocaleJa))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusSeeOther)
	}
	want := "/sign_in/two_factor/new?return_to=%2Fsettings"
	if loc := rec.Header().Get("Location"); loc != want {
		t.Errorf("Location = %q、期待値 = %q", loc, want)
	}
}

// TestCreate_ReturnToSurvivesValidationErrorは、サインイン失敗時の再描画でも遷移先が
// フォームに残ることを検証する。これにより続く再試行でもユーザーは向かっていた先へ着地できる。
func TestCreate_ReturnToSurvivesValidationError(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler, _ := newSignInHandler(t, db)
	email := seedUserWithPassword(t, db, "password123")

	rec := httptest.NewRecorder()
	handler.Create(rec, postSignInWithReturnTo(email, "wrongpassword", "/settings", model.LocaleJa))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusUnprocessableEntity)
	}
	if !strings.Contains(rec.Body.String(), `value="/settings"`) {
		t.Error("再描画したフォームにreturn_toが残っていない")
	}
}
