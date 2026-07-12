package sign_in_two_factor_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
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

// getNew builds a GET /sign_in/two_factor/new request, attaching the pending cookie
// when pendingUserID is non-empty, with the locale set in its context.
//
// [Ja] getNew は GET /sign_in/two_factor/new リクエストを組み立て、pendingUserID が空でなければ
// pending Cookie を付け、context にロケールを設定する。
func getNew(pendingUserID, locale string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/sign_in/two_factor/new", nil)
	if pendingUserID != "" {
		req.AddCookie(&http.Cookie{Name: session.TwoFactorPendingCookieName, Value: pendingUserID})
	}
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
// challenge to complete.
//
// [Ja] TestNew_NoCookieRedirectsToSignIn は、pending Cookie の無い
// GET /sign_in/two_factor/new がサインインへリダイレクトすることを検証する。完了すべき
// 保留中のチャレンジが無いためである。
func TestNew_NoCookieRedirectsToSignIn(t *testing.T) {
	t.Parallel()

	handler := newSignInTwoFactorHandler(t)

	rec := httptest.NewRecorder()
	handler.New(rec, getNew("", i18n.LangJa))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/sign_in" {
		t.Errorf("Location = %q, want %q", loc, "/sign_in")
	}
}
