package account_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/handler/account"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/query"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/validator"
)

// newAccountHandler wires an account Handler over the shared pool. The
// CreateAccountUsecase opens its own transaction, so its tests commit rows and
// use unique emails (the test database is reset by make test) rather than the
// rolled-back transaction pattern.
//
// [Ja] newAccountHandler は共有プールで account Handler を組み立てます。
// CreateAccountUsecase は自前のトランザクションを開くため、そのテストはロールバック
// されるトランザクションパターンではなく、行をコミットしユニークな email を使います
// (テスト DB は make test がリセットする)。
func newAccountHandler(t *testing.T) *account.Handler {
	t.Helper()

	cfg := &config.Config{Env: "test"}
	db := testutil.GetTestDB()
	queries := query.New(db)
	userRepo := repository.NewUserRepository(queries)
	userPasswordRepo := repository.NewUserPasswordRepository(queries)
	userSessionRepo := repository.NewUserSessionRepository(queries)
	emailConfirmationRepo := repository.NewEmailConfirmationRepository(queries)

	sessionMgr := session.NewManager(userRepo, cfg)
	createAccountUC := usecase.NewCreateAccountUsecase(
		db,
		validator.NewAccountCreateValidator(),
		emailConfirmationRepo,
		userRepo,
		userPasswordRepo,
	)
	createSessionUC := usecase.NewCreateSessionUsecase(userSessionRepo)
	return account.NewHandler(cfg, sessionMgr, createAccountUC, createSessionUC)
}

// getAccountNew builds a GET /account/new request, attaching the handoff cookie
// when confirmationID is non-empty, with the locale set in its context.
//
// [Ja] getAccountNew は GET /account/new リクエストを組み立て、confirmationID が空で
// なければ受け渡し Cookie を付け、context にロケールを設定する。
func getAccountNew(confirmationID, locale string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/account/new", nil)
	if confirmationID != "" {
		req.AddCookie(&http.Cookie{Name: session.EmailConfirmationCookieName, Value: confirmationID})
	}
	return req.WithContext(i18n.SetLocale(req.Context(), locale))
}

// TestNew verifies that GET /account/new returns HTTP 200 with the password-setup
// form (password and password-confirmation fields and a CSRF hidden field) and
// the localized heading for each supported locale, when the handoff cookie is
// present.
//
// [Ja] TestNew は、受け渡し Cookie がある場合に GET /account/new が HTTP 200 と、
// パスワード設定フォーム (password / password_confirmation フィールド・CSRF hidden
// フィールド) を、サポートする各ロケールのローカライズ済み見出しとともに返すことを
// 検証する。
func TestNew(t *testing.T) {
	t.Parallel()

	handler := newAccountHandler(t)

	tests := []struct {
		name        string
		locale      string
		wantHeading string
	}{
		{name: "Japanese", locale: i18n.LangJa, wantHeading: "パスワードを設定"},
		{name: "English", locale: i18n.LangEn, wantHeading: "Set your password"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			handler.New(rec, getAccountNew(uuid.NewString(), tt.locale))

			if rec.Code != http.StatusOK {
				t.Errorf("status code = %d, want %d", rec.Code, http.StatusOK)
			}
			if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
				t.Errorf("Content-Type = %q, want prefix %q", got, "text/html")
			}

			body := rec.Body.String()
			wants := []string{
				tt.wantHeading,
				`action="/account"`,
				`method="POST"`,
				`name="csrf_token"`,
				`name="password"`,
				`name="password_confirmation"`,
				`type="password"`,
			}
			for _, want := range wants {
				if !strings.Contains(body, want) {
					t.Errorf("response body does not contain %q", want)
				}
			}
		})
	}
}

// TestNew_NoCookieRedirectsToSignUp verifies that GET /account/new without the
// handoff cookie redirects to sign-up, since there is no in-progress sign-up to
// finish.
//
// [Ja] TestNew_NoCookieRedirectsToSignUp は、受け渡し Cookie の無い GET /account/new が
// サインアップへリダイレクトすることを検証する。完了すべき進行中のサインアップが無い
// ためである。
func TestNew_NoCookieRedirectsToSignUp(t *testing.T) {
	t.Parallel()

	handler := newAccountHandler(t)

	rec := httptest.NewRecorder()
	handler.New(rec, getAccountNew("", i18n.LangJa))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/sign_up" {
		t.Errorf("Location = %q, want %q", loc, "/sign_up")
	}
}
