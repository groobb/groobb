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
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/validator"
)

// newSignInHandler wires a sign-in Handler over the test database's repositories,
// with a Turnstile verifier that passes by default, so a handler test exercises
// the full request path (Turnstile gate, validator, UseCase, session cookie)
// against a real database. The verifier is returned so a test can make Turnstile
// verification fail.
//
// [Ja] newSignInHandler はテスト用データベースのリポジトリで、既定で通過する Turnstile
// 検証器を伴ってサインイン Handler を組み立て、ハンドラーテストが実 DB に対して
// リクエスト経路全体 (Turnstile ゲート・バリデーター・UseCase・セッション Cookie) を
// 通すようにします。Turnstile 検証を失敗させられるよう、検証器も併せて返します。
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

// seedUserWithPassword creates a committed user with the given email and a
// password credential, returning the email so a handler test can sign in as it.
//
// [Ja] seedUserWithPassword は指定 email のユーザーとパスワード資格情報をコミットして
// 作成し、ハンドラーテストがそれとしてサインインできるよう email を返す。
func seedUserWithPassword(t *testing.T, db *database.DB, password string) string {
	t.Helper()

	ctx := context.Background()
	email := testutil.UniqueEmail(db, "signin-h")

	user, err := repository.NewUserRepository(db).Create(ctx, repository.CreateUserInput{
		Email:    email,
		Atname:   testutil.UniqueAtname(db),
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
	if _, err := repository.NewUserPasswordRepository(db).Create(ctx, repository.CreateUserPasswordInput{
		UserID:         user.ID,
		PasswordDigest: digest,
	}); err != nil {
		t.Fatalf("パスワード資格情報の作成に失敗: %v", err)
	}
	return email
}

// seedUserWithTwoFactor creates a committed user with a password credential and
// an enabled 2FA setting, returning the email so a handler test can sign in as it
// and hit the two-factor challenge branch.
//
// [Ja] seedUserWithTwoFactor はパスワード資格情報と有効な 2FA 設定を持つユーザーを
// コミットして作成し、ハンドラーテストがそれとしてサインインして 2 段階認証チャレンジの
// 分岐へ到達できるよう email を返す。
func seedUserWithTwoFactor(t *testing.T, db *database.DB, password string) string {
	t.Helper()

	ctx := context.Background()
	email := testutil.UniqueEmail(db, "signin-h-2fa")

	user, err := repository.NewUserRepository(db).Create(ctx, repository.CreateUserInput{
		Email:    email,
		Atname:   testutil.UniqueAtname(db),
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
		t.Fatalf("2 段階認証設定の作成に失敗: %v", err)
	}
	enabled, err := twoFactorRepo.Enable(ctx, user.ID, []string{"recoverycode1"})
	if err != nil {
		t.Fatalf("2 段階認証の有効化に失敗: %v", err)
	}
	if !enabled {
		t.Fatal("2 段階認証を有効化できなかった (未有効化の行が見つからない)")
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
// cookie is set) and redirect to the home page.
//
// [Ja] TestCreate_Success は、有効な資格情報がユーザーをサインインさせ (セッション
// Cookie を設定)、ホームへリダイレクトすることを検証する。
func TestCreate_Success(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler, _ := newSignInHandler(t, db)
	email := seedUserWithPassword(t, db, "password123")

	rec := httptest.NewRecorder()
	handler.Create(rec, postSignIn(email, "password123", i18n.LangJa))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/home" {
		t.Errorf("Location = %q, want %q", loc, "/home")
	}
	if sessionCookie := findCookie(rec, session.CookieName); sessionCookie == nil || sessionCookie.Value == "" {
		t.Error("サインイン後にセッション Cookie が設定されていない")
	}
	// A user without 2FA is signed in outright, so no pending cookie is set.
	//
	// [Ja] 2FA 無しのユーザーはそのままサインインするため、pending Cookie は設定されない。
	if findCookie(rec, session.TwoFactorPendingCookieName) != nil {
		t.Error("2FA 無しのサインインで pending Cookie が設定されている")
	}
}

// TestCreate_TwoFactorEnabled verifies that a 2FA-enabled account is not signed
// in from the password alone: the password step passes but Create issues no
// session, sets the short-lived pending cookie, and redirects to the TOTP
// challenge instead of the home page.
//
// [Ja] TestCreate_TwoFactorEnabled は、2FA 有効なアカウントがパスワードだけでは
// サインインしないことを検証する。パスワードのステップは通るが Create はセッションを
// 発行せず、短命の pending Cookie を設定し、ホームではなく TOTP チャレンジへ
// リダイレクトする。
func TestCreate_TwoFactorEnabled(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler, _ := newSignInHandler(t, db)
	email := seedUserWithTwoFactor(t, db, "password123")

	rec := httptest.NewRecorder()
	handler.Create(rec, postSignIn(email, "password123", i18n.LangJa))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/sign_in/two_factor/new" {
		t.Errorf("Location = %q, want %q", loc, "/sign_in/two_factor/new")
	}
	// The password matched but 2FA is required, so no session is issued yet;
	// instead the pending user is held in the two-factor cookie for the challenge.
	//
	// [Ja] パスワードは一致したが 2FA が必要なため、この時点ではセッションを発行しない。
	// 代わりに保留中のユーザーをチャレンジ用の 2 段階認証 Cookie に保持する。
	if findCookie(rec, session.CookieName) != nil {
		t.Error("2FA 必須なのにセッション Cookie が設定されている (パスワードだけでサインインしてしまっている)")
	}
	pending := findCookie(rec, session.TwoFactorPendingCookieName)
	if pending == nil || pending.Value == "" {
		t.Fatal("2 段階認証の pending Cookie が設定されていない")
	}
}

// TestCreate_WrongPassword verifies that a wrong password re-renders the form
// with 422 and the generic credentials message, without setting a session cookie.
//
// [Ja] TestCreate_WrongPassword は、誤ったパスワードがフォームを 422 と汎用の資格情報
// メッセージで再描画し、セッション Cookie を設定しないことを検証する。
func TestCreate_WrongPassword(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler, _ := newSignInHandler(t, db)
	email := seedUserWithPassword(t, db, "password123")

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

	db := testutil.SetupDB(t)

	handler, _ := newSignInHandler(t, db)

	rec := httptest.NewRecorder()
	handler.Create(rec, postSignIn("", "password123", i18n.LangJa))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	if !strings.Contains(rec.Body.String(), `aria-invalid="true"`) {
		t.Error("エラー時の入力欄に aria-invalid='true' が無い")
	}
}

// TestCreate_TurnstileFailure verifies that when Turnstile verification does not
// pass — a non-pass or a siteverify error — Create stops the request at the bot
// gate: it re-renders the form with 422 and the form-wide Turnstile message,
// echoes the email and the destination back, forwards the submitted token to the
// verifier, and neither authenticates nor issues a session. Valid credentials are
// supplied on purpose, so a gate bypass would sign the user in (303 + session
// cookie); the 422 and the absent session cookie confirm the gate ran before
// authentication. The bot gate is a separate branch from the credential-check
// re-render covered by TestCreate_ReturnToSurvivesValidationError, so the
// destination is checked on this path too: losing it here would drop the visitor
// on home after a retry that succeeds.
//
// [Ja] TestCreate_TurnstileFailure は、Turnstile 検証が通過しないとき (非通過または
// siteverify エラー) に Create が Bot ゲートでリクエストを止めることを検証する。
// フォームを 422 とフォーム全体の Turnstile メッセージで再描画し、email と遷移先を
// エコーバックし、送信されたトークンを検証器へ渡し、認証もセッション発行もしないことを
// 確認する。有効な資格情報をあえて与えているため、ゲートが迂回されればユーザーは
// サインインしてしまう (303 + セッション Cookie)。422 とセッション Cookie の不在が、
// ゲートが認証の前で走ったことを裏付ける。Bot ゲートは
// TestCreate_ReturnToSurvivesValidationError が見る資格情報チェックの再描画とは別の分岐で
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
			req = req.WithContext(i18n.SetLocale(req.Context(), i18n.LangJa))

			rec := httptest.NewRecorder()
			handler.Create(rec, req)

			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status code = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
			}
			body := rec.Body.String()
			// The sign-in form is re-rendered carrying the form-wide Turnstile message.
			//
			// [Ja] サインインフォームが再描画され、フォーム全体の Turnstile メッセージを
			// 載せていること。
			if !strings.Contains(body, `action="/sign_in"`) {
				t.Error("サインインフォームが再描画されていない")
			}
			if !strings.Contains(body, "ロボットでないことの確認に失敗しました") {
				t.Error("Turnstile 失敗のフォーム全体メッセージが描画されていない")
			}
			if !strings.Contains(body, `role="alert"`) {
				t.Error("フォーム全体のエラーに role='alert' が無い")
			}
			// The email is echoed back so the user does not have to retype it.
			//
			// [Ja] ユーザーが再入力しなくて済むよう email はエコーバックされること。
			if !strings.Contains(body, email) {
				t.Error("入力した email が再描画フォームにエコーバックされていない")
			}
			// The destination survives the bot gate, so a successful retry still
			// lands the visitor where they were headed.
			//
			// [Ja] 遷移先は Bot ゲートを越えて残り、再試行に成功した訪問者は向かっていた
			// 先へ着地できること。
			if !strings.Contains(body, `value="/settings"`) {
				t.Error("再描画したフォームに return_to が残っていない")
			}
			// The submitted token reached the verifier, confirming the handler read
			// the correct cf-turnstile-response field.
			//
			// [Ja] 送信されたトークンが検証器へ到達しており、ハンドラーが正しい
			// cf-turnstile-response フィールドを読んでいることを確認する。
			if verifier.Token != "submitted-token" {
				t.Errorf("verifier に渡ったトークン = %q, want %q", verifier.Token, "submitted-token")
			}
			// The bot gate must stop the request before authentication, so even the
			// valid credentials supplied here do not issue a session.
			//
			// [Ja] Bot ゲートは認証の前でリクエストを止めるため、ここで与えた有効な
			// 資格情報でもセッションは発行されないこと。
			if findCookie(rec, session.CookieName) != nil {
				t.Error("Turnstile 失敗時にセッション Cookie が設定された (認証に進んでしまっている)")
			}
		})
	}
}

// postSignInWithReturnTo builds a POST /sign_in request that also carries a
// return_to destination, as the form does when the visitor arrived from a route
// that turned them away.
//
// [Ja] postSignInWithReturnTo は return_to の遷移先も併せて運ぶ POST /sign_in リクエストを
// 組み立てる。訪問者が追い返されたルートから来たときにフォームが送る形と同じである。
func postSignInWithReturnTo(email, password, returnTo, locale string) *http.Request {
	form := url.Values{"email": {email}, "password": {password}, "return_to": {returnTo}}
	req := httptest.NewRequest(http.MethodPost, "/sign_in", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req.WithContext(i18n.SetLocale(req.Context(), locale))
}

// TestCreate_ReturnTo verifies that a sign-in carrying a destination lands the
// user there instead of on the home page, and that a destination naming another
// origin is discarded so the sign-in flow cannot be used as an open redirect.
//
// [Ja] TestCreate_ReturnTo は、遷移先を伴うサインインがユーザーをホームではなく
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
			handler.Create(rec, postSignInWithReturnTo(email, "password123", tt.returnTo, i18n.LangJa))

			if rec.Code != http.StatusSeeOther {
				t.Fatalf("status code = %d, want %d", rec.Code, http.StatusSeeOther)
			}
			if loc := rec.Header().Get("Location"); loc != tt.wantLocation {
				t.Errorf("Location = %q, want %q", loc, tt.wantLocation)
			}
			if sessionCookie := findCookie(rec, session.CookieName); sessionCookie == nil || sessionCookie.Value == "" {
				t.Error("サインイン後にセッション Cookie が設定されていない")
			}
		})
	}
}

// TestCreate_TwoFactorEnabledForwardsReturnTo verifies that a 2FA-enabled account
// keeps its destination across the challenge hop: the redirect to the TOTP form
// carries return_to, so the code-entry form can hand it on to the step that
// finally issues the session.
//
// [Ja] TestCreate_TwoFactorEnabledForwardsReturnTo は、2FA 有効なアカウントがチャレンジの
// ホップを跨いで遷移先を保つことを検証する。TOTP フォームへのリダイレクトが return_to を運ぶ
// ため、コード入力フォームは最終的にセッションを発行するステップへそれを引き渡せる。
func TestCreate_TwoFactorEnabledForwardsReturnTo(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler, _ := newSignInHandler(t, db)
	email := seedUserWithTwoFactor(t, db, "password123")

	rec := httptest.NewRecorder()
	handler.Create(rec, postSignInWithReturnTo(email, "password123", "/settings", i18n.LangJa))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	want := "/sign_in/two_factor/new?return_to=%2Fsettings"
	if loc := rec.Header().Get("Location"); loc != want {
		t.Errorf("Location = %q, want %q", loc, want)
	}
}

// TestCreate_ReturnToSurvivesValidationError verifies that a failed sign-in
// re-renders the form with the destination still in it, so the retry that follows
// still lands the user where they were headed.
//
// [Ja] TestCreate_ReturnToSurvivesValidationError は、サインイン失敗時の再描画でも遷移先が
// フォームに残ることを検証する。これにより続く再試行でもユーザーは向かっていた先へ着地できる。
func TestCreate_ReturnToSurvivesValidationError(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler, _ := newSignInHandler(t, db)
	email := seedUserWithPassword(t, db, "password123")

	rec := httptest.NewRecorder()
	handler.Create(rec, postSignInWithReturnTo(email, "wrongpassword", "/settings", i18n.LangJa))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	if !strings.Contains(rec.Body.String(), `value="/settings"`) {
		t.Error("再描画したフォームに return_to が残っていない")
	}
}
