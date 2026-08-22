package sign_in_two_factor_recovery_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/handler/sign_in_two_factor_recovery"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/validator"
)

// seededHandlerRecoveryCodes are the known recovery codes a handler test enrolls,
// each in the eight-lowercase-alphanumeric format the validator accepts.
//
// [Ja] seededHandlerRecoveryCodes はハンドラーテストが登録する既知のリカバリーコードで、
// それぞれバリデーターが受理する 8 文字の小文字英数字の形式です。
var seededHandlerRecoveryCodes = []string{"abcd1234", "efgh5678"}

// newSignInTwoFactorRecoveryHandler wires a sign_in_two_factor_recovery Handler
// over the test database's repositories, so a handler test exercises the full
// request path (pending cookie, validator, UseCase, session cookie) against a
// real database.
//
// [Ja] newSignInTwoFactorRecoveryHandler はテスト用データベースのリポジトリで
// sign_in_two_factor_recovery Handler を組み立てる。ハンドラーテストがリクエスト経路全体
// (pending Cookie・バリデーター・UseCase・セッション Cookie) を実 DB に対して通せるように
// するためである。
func newSignInTwoFactorRecoveryHandler(t *testing.T, db *database.DB) *sign_in_two_factor_recovery.Handler {
	t.Helper()

	cfg := testutil.NewTestConfig(t)
	userRepo := repository.NewUserRepository(db)
	userTwoFactorAuthRepo := repository.NewUserTwoFactorAuthRepository(db)
	userSessionRepo := repository.NewUserSessionRepository(db)

	sessionMgr := session.NewManager(userRepo, cfg)
	createUC := usecase.NewCreateSignInTwoFactorRecoveryUsecase(
		db.Writer,
		validator.NewSignInTwoFactorRecoveryCreateValidator(userTwoFactorAuthRepo),
		userTwoFactorAuthRepo,
		userSessionRepo,
	)
	return sign_in_two_factor_recovery.NewHandler(cfg, sessionMgr, createUC)
}

// twoFactorPendingToken issues the same signed handoff token as the password
// sign-in step, so recovery handler tests use the production handoff contract.
//
// [Ja] twoFactorPendingToken はパスワードサインインのステップと同じ署名付き受け渡し token を
// 発行し、リカバリーハンドラーテストでも本番の受け渡し契約を使う。
func twoFactorPendingToken(t *testing.T, id model.UserID) string {
	t.Helper()

	mgr := session.NewManager(nil, testutil.NewTestConfig(t))
	rec := httptest.NewRecorder()
	mgr.SetTwoFactorPendingUserID(rec, id)
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == session.TwoFactorPendingCookieName && cookie.Value != "" {
			return cookie.Value
		}
	}
	t.Fatalf("2 段階認証 pending Cookie %q の署名 token が発行されていない", session.TwoFactorPendingCookieName)
	return ""
}

// seedUserWithRecoveryCodes creates a committed user with an enabled 2FA setting
// holding the given recovery codes, returning the user id so a handler test can put
// it in the pending cookie and submit a matching code.
//
// [Ja] seedUserWithRecoveryCodes は指定のリカバリーコードを持つ有効な 2FA 設定付きの
// コミット済みユーザーを作成し、ハンドラーテストがその id を pending Cookie に入れ、一致する
// コードを送信できるよう user id を返す。
func seedUserWithRecoveryCodes(t *testing.T, db *database.DB, recoveryCodes []string) model.UserID {
	t.Helper()

	ctx := context.Background()

	user, err := repository.NewUserRepository(db).Create(ctx, repository.CreateUserInput{
		Email:    testutil.UniqueEmail(db, "2fa-rc"),
		Atname:   testutil.UniqueAtname(db),
		Locale:   i18n.LangJa,
		TimeZone: "Asia/Tokyo",
	})
	if err != nil {
		t.Fatalf("ユーザーの作成に失敗: %v", err)
	}

	twoFactorRepo := repository.NewUserTwoFactorAuthRepository(db)
	if _, err := twoFactorRepo.Create(ctx, repository.CreateUserTwoFactorAuthInput{
		UserID: user.ID,
		Secret: testutil.DefaultBuilderTOTPSecret,
	}); err != nil {
		t.Fatalf("2 段階認証設定の作成に失敗: %v", err)
	}
	enabled, err := twoFactorRepo.Enable(ctx, user.ID, recoveryCodes)
	if err != nil {
		t.Fatalf("2 段階認証の有効化に失敗: %v", err)
	}
	if !enabled {
		t.Fatal("2 段階認証を有効化できなかった (未有効化の行が見つからない)")
	}
	return user.ID
}

// getNew builds a GET /sign_in/two_factor/recovery/new request carrying
// pendingUserID in the pending cookie, with the locale set in its context.
//
// [Ja] getNew は pendingUserID を pending Cookie に載せた
// GET /sign_in/two_factor/recovery/new リクエストを組み立て、context にロケールを設定する。
func getNew(pendingUserID, locale string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/sign_in/two_factor/recovery/new", nil)
	req.AddCookie(&http.Cookie{Name: session.TwoFactorPendingCookieName, Value: pendingUserID})
	return req.WithContext(i18n.SetLocale(req.Context(), locale))
}

// TestNew verifies that GET /sign_in/two_factor/recovery/new returns HTTP 200 with
// the recovery-code entry form (code field and CSRF hidden field), the localized
// heading, and the link back to the authenticator challenge, when the pending
// cookie carries a valid signed continuation token.
//
// [Ja] TestNew は、pending Cookie がある場合に GET /sign_in/two_factor/recovery/new が
// HTTP 200 と、リカバリーコード入力フォーム (code フィールド・CSRF hidden フィールド)、
// ローカライズ済み見出し、認証アプリチャレンジへ戻るリンクを返すことを検証する。Cookie は
// 有効な署名付き continuation token を運ぶ。
func TestNew(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler := newSignInTwoFactorRecoveryHandler(t, db)

	tests := []struct {
		name        string
		locale      string
		wantHeading string
	}{
		{name: "Japanese", locale: i18n.LangJa, wantHeading: "リカバリーコード"},
		{name: "English", locale: i18n.LangEn, wantHeading: "Recovery code"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			handler.New(rec, getNew(twoFactorPendingToken(t, model.UserID(testutil.UnusedID)), tt.locale))

			if rec.Code != http.StatusOK {
				t.Errorf("status code = %d, want %d", rec.Code, http.StatusOK)
			}
			if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
				t.Errorf("Content-Type = %q, want prefix %q", got, "text/html")
			}

			body := rec.Body.String()
			wants := []string{
				tt.wantHeading,
				`action="/sign_in/two_factor/recovery"`,
				`method="POST"`,
				`name="csrf_token"`,
				`name="code"`,
				`<label for="code"`,
				// The link back to the authenticator-code challenge.
				//
				// [Ja] 認証アプリのコードチャレンジへ戻るリンク。
				`href="/sign_in/two_factor/new"`,
				// A transient, non-public auth interstitial is kept out of search results.
				//
				// [Ja] 一時的で非公開の認証中間ページは検索結果に出さない。
				`name="robots" content="noindex"`,
			}
			for _, want := range wants {
				if !strings.Contains(body, want) {
					t.Errorf("response body does not contain %q", want)
				}
			}
		})
	}
}

// TestNew_NoCookieRedirectsToSignIn verifies that GET
// /sign_in/two_factor/recovery/new without the pending cookie redirects to
// sign-in, since there is no pending challenge to complete, and that the redirect
// keeps the destination when the request carried one.
//
// [Ja] TestNew_NoCookieRedirectsToSignIn は、pending Cookie の無い
// GET /sign_in/two_factor/recovery/new がサインインへリダイレクトすること (完了すべき
// 保留中のチャレンジが無いため)、そしてリクエストが遷移先を運んでいたときはリダイレクトが
// それを保つことを検証する。
func TestNew_NoCookieRedirectsToSignIn(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler := newSignInTwoFactorRecoveryHandler(t, db)

	tests := []struct {
		name         string
		target       string
		wantLocation string
	}{
		{
			name:         "遷移先なし",
			target:       "/sign_in/two_factor/recovery/new",
			wantLocation: "/sign_in",
		},
		// The visitor's destination has not changed just because the challenge was
		// lost, so the restart carries it instead of dropping them on the home page.
		//
		// [Ja] チャレンジが失われても訪問者の目的の画面は変わらないため、やり直しでも遷移先を
		// 運び、ホームに着地させない。
		{
			name:         "遷移先あり",
			target:       "/sign_in/two_factor/recovery/new?return_to=%2Fsettings",
			wantLocation: "/sign_in?return_to=%2Fsettings",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, tt.target, nil)
			req = req.WithContext(i18n.SetLocale(req.Context(), i18n.LangJa))
			rec := httptest.NewRecorder()

			handler.New(rec, req)

			if rec.Code != http.StatusSeeOther {
				t.Fatalf("status code = %d, want %d", rec.Code, http.StatusSeeOther)
			}
			if loc := rec.Header().Get("Location"); loc != tt.wantLocation {
				t.Errorf("Location = %q, want %q", loc, tt.wantLocation)
			}
		})
	}
}

// TestNew_ReturnTo verifies that the recovery-code challenge page keeps the
// destination the flow carried: it rides along in the form's hidden field and in
// the link back to the authenticator challenge, so switching between the two
// challenges does not lose where the visitor was headed. A destination naming
// another origin is dropped from both, so neither can hand an open redirect on to
// the step that issues the session.
//
// [Ja] TestNew_ReturnTo は、リカバリーコードチャレンジページがフローの運んできた遷移先を
// 保つことを検証する。遷移先はフォームの hidden フィールドと、認証アプリのチャレンジへ戻る
// リンクの両方に載るため、2 つのチャレンジを行き来しても訪問者の向かっていた先を失わない。
// 別オリジンを指す遷移先は両方から落ちるため、どちらもセッションを発行するステップへ
// オープンリダイレクトを引き渡せない。
func TestNew_ReturnTo(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler := newSignInTwoFactorRecoveryHandler(t, db)

	tests := []struct {
		name       string
		returnTo   string
		wantHidden bool
		wantLink   string
	}{
		{
			name:       "同一オリジンの相対パスは引き継ぐ",
			returnTo:   "/settings",
			wantHidden: true,
			wantLink:   `href="/sign_in/two_factor/new?return_to=%2Fsettings"`,
		},
		{
			name:       "別オリジンを指す値は引き継がない",
			returnTo:   "https://evil.example.com/settings",
			wantHidden: false,
			wantLink:   `href="/sign_in/two_factor/new"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			target := "/sign_in/two_factor/recovery/new?return_to=" + url.QueryEscape(tt.returnTo)
			req := httptest.NewRequest(http.MethodGet, target, nil)
			req.AddCookie(&http.Cookie{
				Name:  session.TwoFactorPendingCookieName,
				Value: twoFactorPendingToken(t, model.UserID(testutil.UnusedID)),
			})
			req = req.WithContext(i18n.SetLocale(req.Context(), i18n.LangJa))
			rec := httptest.NewRecorder()

			handler.New(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
			}
			body := rec.Body.String()
			if got := strings.Contains(body, `name="return_to"`); got != tt.wantHidden {
				t.Errorf("return_to の hidden フィールドの有無 = %v, want %v", got, tt.wantHidden)
			}
			if tt.wantHidden && !strings.Contains(body, `value="/settings"`) {
				t.Error(`body does not contain value="/settings"`)
			}
			if !strings.Contains(body, tt.wantLink) {
				t.Errorf("body does not contain %q", tt.wantLink)
			}
		})
	}
}
