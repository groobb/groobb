package sign_in_two_factor_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/handler/sign_in_two_factor"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/query"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/validator"
)

// newSignInTwoFactorHandler wires a sign_in_two_factor Handler over the shared pool
// (not a rolled-back transaction) because completing the challenge creates a
// session row through the pool; an outer transaction's seed rows would be invisible
// to it. Its tests seed committed users with unique emails (the test database is
// reset by make test). This exercises the full request path (pending cookie,
// validator, UseCase, session cookie) against a real database.
//
// [Ja] newSignInTwoFactorHandler は共有プール (ロールバックされるトランザクションではなく)
// で sign_in_two_factor Handler を組み立てる。チャレンジの完了はプール経由でセッション行を
// 作るため、外側トランザクションで仕込んだ行はそこから見えないからである。テストはユニークな
// email でコミット済みユーザーを仕込む (テスト DB は make test がリセットする)。これにより
// リクエスト経路全体 (pending Cookie・バリデーター・UseCase・セッション Cookie) を実 DB に
// 対して通す。
func newSignInTwoFactorHandler(t *testing.T) *sign_in_two_factor.Handler {
	t.Helper()

	cfg := &config.Config{Env: "test"}
	queries := query.New(testutil.GetTestDB())
	userRepo := repository.NewUserRepository(queries)
	userTwoFactorAuthRepo := repository.NewUserTwoFactorAuthRepository(queries)
	userSessionRepo := repository.NewUserSessionRepository(queries)

	sessionMgr := session.NewManager(userRepo, cfg)
	createSignInTwoFactorUC := usecase.NewCreateSignInTwoFactorUsecase(validator.NewSignInTwoFactorCreateValidator(userTwoFactorAuthRepo))
	createSessionUC := usecase.NewCreateSessionUsecase(userSessionRepo)
	return sign_in_two_factor.NewHandler(cfg, sessionMgr, createSignInTwoFactorUC, createSessionUC)
}

// seedUserWithEnabledTwoFactor creates a committed user with an enabled 2FA setting
// keyed to the default TOTP secret, returning the user id so a handler test can put
// it in the pending cookie and generate matching codes.
//
// [Ja] seedUserWithEnabledTwoFactor は既定の TOTP secret に紐づく有効な 2FA 設定を持つ
// ユーザーをコミットして作成し、ハンドラーテストがその id を pending Cookie に入れ、一致する
// コードを生成できるよう user id を返す。
func seedUserWithEnabledTwoFactor(t *testing.T) model.UserID {
	t.Helper()

	ctx := context.Background()
	queries := query.New(testutil.GetTestDB())

	user, err := repository.NewUserRepository(queries).Create(ctx, repository.CreateUserInput{
		Email:    fmt.Sprintf("2fa-ch-%s@example.com", uuid.NewString()),
		Atname:   testutil.UniqueAtname(),
		Locale:   i18n.LangJa,
		TimeZone: "Asia/Tokyo",
	})
	if err != nil {
		t.Fatalf("ユーザーの作成に失敗: %v", err)
	}

	twoFactorRepo := repository.NewUserTwoFactorAuthRepository(queries)
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
	return user.ID
}

// getNew builds a GET /sign_in/two_factor/new request carrying pendingUserID in
// the pending cookie, with the locale set in its context.
//
// [Ja] getNew は pendingUserID を pending Cookie に載せた GET /sign_in/two_factor/new
// リクエストを組み立て、context にロケールを設定する。
func getNew(pendingUserID, locale string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/sign_in/two_factor/new", nil)
	req.AddCookie(&http.Cookie{Name: session.TwoFactorPendingCookieName, Value: pendingUserID})
	return req.WithContext(i18n.SetLocale(req.Context(), locale))
}

// TestNew verifies that GET /sign_in/two_factor/new returns HTTP 200 with the
// code-entry form (code field and CSRF hidden field) and the localized heading for
// each supported locale, when the pending cookie is present. The cookie id is not
// resolved here, so any value is enough to render the thin form.
//
// [Ja] TestNew は、pending Cookie がある場合に GET /sign_in/two_factor/new が HTTP 200 と、
// コード入力フォーム (code フィールド・CSRF hidden フィールド) を、サポートする各ロケールの
// ローカライズ済み見出しとともに返すことを検証する。ここでは Cookie の id を解決しないため、
// 薄いフォームの描画には任意の値で足りる。
func TestNew(t *testing.T) {
	t.Parallel()

	handler := newSignInTwoFactorHandler(t)

	tests := []struct {
		name        string
		locale      string
		wantHeading string
	}{
		{name: "Japanese", locale: i18n.LangJa, wantHeading: "2 段階認証"},
		{name: "English", locale: i18n.LangEn, wantHeading: "Two-factor authentication"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			handler.New(rec, getNew(uuid.NewString(), tt.locale))

			if rec.Code != http.StatusOK {
				t.Errorf("status code = %d, want %d", rec.Code, http.StatusOK)
			}
			if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
				t.Errorf("Content-Type = %q, want prefix %q", got, "text/html")
			}

			body := rec.Body.String()
			wants := []string{
				tt.wantHeading,
				`action="/sign_in/two_factor"`,
				`method="POST"`,
				`name="csrf_token"`,
				`name="code"`,
				`<label for="code"`,
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

// TestNew_NoCookieRedirectsToSignIn verifies that GET /sign_in/two_factor/new
// without the pending cookie redirects to sign-in, since there is no pending
// challenge to complete, and that the redirect keeps the destination when the
// request carried one.
//
// [Ja] TestNew_NoCookieRedirectsToSignIn は、pending Cookie の無い
// GET /sign_in/two_factor/new がサインインへリダイレクトすること (完了すべき保留中の
// チャレンジが無いため)、そしてリクエストが遷移先を運んでいたときはリダイレクトがそれを
// 保つことを検証する。
func TestNew_NoCookieRedirectsToSignIn(t *testing.T) {
	t.Parallel()

	handler := newSignInTwoFactorHandler(t)

	tests := []struct {
		name         string
		target       string
		wantLocation string
	}{
		{
			name:         "遷移先なし",
			target:       "/sign_in/two_factor/new",
			wantLocation: "/sign_in",
		},
		// The visitor's destination has not changed just because the challenge was
		// lost, so the restart carries it instead of dropping them on the home page.
		//
		// [Ja] チャレンジが失われても訪問者の目的の画面は変わらないため、やり直しでも遷移先を
		// 運び、ホームに着地させない。
		{
			name:         "遷移先あり",
			target:       "/sign_in/two_factor/new?return_to=%2Fc%2Fgroobb",
			wantLocation: "/sign_in?return_to=%2Fc%2Fgroobb",
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

// TestNew_ReturnTo verifies that the TOTP challenge page keeps the destination the
// password step handed over: it rides along in the form's hidden field and in the
// link to the recovery-code challenge, so switching to recovery codes does not lose
// where the visitor was headed. A destination naming another origin is dropped from
// both, so neither can hand an open redirect on to the step that issues the session.
//
// [Ja] TestNew_ReturnTo は、TOTP チャレンジページがパスワードのステップから引き渡された
// 遷移先を保つことを検証する。遷移先はフォームの hidden フィールドと、リカバリーコード
// チャレンジへのリンクの両方に載るため、リカバリーコードへ切り替えても訪問者の向かっていた
// 先を失わない。別オリジンを指す遷移先は両方から落ちるため、どちらもセッションを発行する
// ステップへオープンリダイレクトを引き渡せない。
func TestNew_ReturnTo(t *testing.T) {
	t.Parallel()

	handler := newSignInTwoFactorHandler(t)

	tests := []struct {
		name       string
		returnTo   string
		wantHidden bool
		wantLink   string
	}{
		{
			name:       "同一オリジンの相対パスは引き継ぐ",
			returnTo:   "/c/groobb",
			wantHidden: true,
			wantLink:   `href="/sign_in/two_factor/recovery/new?return_to=%2Fc%2Fgroobb"`,
		},
		{
			name:       "別オリジンを指す値は引き継がない",
			returnTo:   "https://evil.example.com/c/groobb",
			wantHidden: false,
			wantLink:   `href="/sign_in/two_factor/recovery/new"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			target := "/sign_in/two_factor/new?return_to=" + url.QueryEscape(tt.returnTo)
			req := httptest.NewRequest(http.MethodGet, target, nil)
			req.AddCookie(&http.Cookie{Name: session.TwoFactorPendingCookieName, Value: uuid.NewString()})
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
			if tt.wantHidden && !strings.Contains(body, `value="/c/groobb"`) {
				t.Error(`body does not contain value="/c/groobb"`)
			}
			if !strings.Contains(body, tt.wantLink) {
				t.Errorf("body does not contain %q", tt.wantLink)
			}
		})
	}
}
