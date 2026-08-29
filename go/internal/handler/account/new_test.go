package account_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/handler/account"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/validator"
)

// newAccountHandler wires an account Handler over the test database's
// repositories, so a handler test drives the CreateAccountUsecase and the
// transaction it opens against a real database.
//
// [Ja] newAccountHandler はテスト用データベースのリポジトリで account Handler を
// 組み立てます。ハンドラーテストが CreateAccountUsecase と、それが開くトランザクションを
// 実 DB に対して駆動できるようにするためです。
func newAccountHandler(t *testing.T, db *database.DB) *account.Handler {
	t.Helper()

	cfg := testutil.NewTestConfig(t)
	userRepo := repository.NewUserRepository(db)
	userPasswordRepo := repository.NewUserPasswordRepository(db)
	userSessionRepo := repository.NewUserSessionRepository(db)
	emailConfirmationRepo := repository.NewEmailConfirmationRepository(db)

	sessionMgr := session.NewManager(userRepo, cfg)
	createAccountUC := usecase.NewCreateAccountUsecase(
		db.Writer,
		validator.NewAccountCreateValidator(userRepo),
		emailConfirmationRepo,
		userRepo,
		userPasswordRepo,
	)
	createSessionUC := usecase.NewCreateSessionUsecase(userSessionRepo)
	return account.NewHandler(cfg, sessionMgr, createAccountUC, createSessionUC)
}

// emailConfirmationToken issues the same signed continuation token the
// sign-up flow would place in the handoff Cookie.
//
// [Ja] emailConfirmationToken はサインアップフローが受け渡し Cookie に設定するものと
// 同じ署名付き continuation token を発行します。
func emailConfirmationToken(t *testing.T, id model.EmailConfirmationID) string {
	t.Helper()

	mgr := session.NewManager(nil, testutil.NewTestConfig(t))
	rec := httptest.NewRecorder()
	mgr.SetEmailConfirmationID(rec, id)
	cookie := findCookie(rec, session.EmailConfirmationCookieName)
	if cookie == nil || cookie.Value == "" {
		t.Fatalf("メール確認 Cookie %q の署名 token が発行されていない", session.EmailConfirmationCookieName)
	}
	return cookie.Value
}

// getAccountNew builds a GET /account/new request, attaching the handoff cookie
// when confirmationID is non-empty, with the locale set in its context.
//
// [Ja] getAccountNew は GET /account/new リクエストを組み立て、confirmationID が空で
// なければ受け渡し Cookie を付け、context にロケールを設定する。
func getAccountNew(confirmationID string, locale model.Locale) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/account/new", nil)
	if confirmationID != "" {
		req.AddCookie(&http.Cookie{Name: session.EmailConfirmationCookieName, Value: confirmationID})
	}
	return req.WithContext(i18n.SetLocale(req.Context(), locale))
}

// TestNew verifies that GET /account/new returns HTTP 200 with the account-creation
// form (atname, password, and password-confirmation fields and a CSRF hidden field)
// and the localized heading and atname label for each supported locale, when the
// handoff cookie is present.
//
// [Ja] TestNew は、受け渡し Cookie がある場合に GET /account/new が HTTP 200 と、
// アカウント作成フォーム (atname / password / password_confirmation フィールド・
// CSRF hidden フィールド) を、サポートする各ロケールのローカライズ済み見出しと
// アットネームのラベルとともに返すことを検証する。
func TestNew(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler := newAccountHandler(t, db)

	tests := []struct {
		name        string
		locale      model.Locale
		wantHeading string
		wantLabel   string
	}{
		{name: "Japanese", locale: model.LocaleJa, wantHeading: "アカウントを作成", wantLabel: "アットネーム"},
		{name: "English", locale: model.LocaleEn, wantHeading: "Create your account", wantLabel: "Atname"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			handler.New(rec, getAccountNew(emailConfirmationToken(t, model.EmailConfirmationID(testutil.UnusedID)), tt.locale))

			if rec.Code != http.StatusOK {
				t.Errorf("status code = %d, want %d", rec.Code, http.StatusOK)
			}
			if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
				t.Errorf("Content-Type = %q, want prefix %q", got, "text/html")
			}

			body := rec.Body.String()
			wants := []string{
				tt.wantHeading,
				tt.wantLabel,
				`action="/account"`,
				`method="POST"`,
				`name="csrf_token"`,
				`name="atname"`,
				`pattern="[A-Za-z0-9_]+"`,
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

	db := testutil.SetupDB(t)

	handler := newAccountHandler(t, db)

	rec := httptest.NewRecorder()
	handler.New(rec, getAccountNew("", model.LocaleJa))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/sign_up" {
		t.Errorf("Location = %q, want %q", loc, "/sign_up")
	}
}
