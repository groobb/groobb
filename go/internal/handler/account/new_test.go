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

// newAccountHandlerはテスト用データベースのリポジトリでaccount Handlerを
// 組み立てます。ハンドラーテストがCreateAccountUsecaseと、それが開くトランザクションを
// 実DBに対して駆動できるようにするためです。
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

// emailConfirmationTokenはサインアップフローが受け渡しCookieに設定するものと
// 同じ署名付きcontinuation tokenを発行します。
func emailConfirmationToken(t *testing.T, id model.EmailConfirmationID) string {
	t.Helper()

	mgr := session.NewManager(nil, testutil.NewTestConfig(t))
	rec := httptest.NewRecorder()
	mgr.SetEmailConfirmationID(rec, id)
	cookie := findCookie(rec, session.EmailConfirmationCookieName)
	if cookie == nil || cookie.Value == "" {
		t.Fatalf("メール確認Cookie %q の署名tokenが発行されていない", session.EmailConfirmationCookieName)
	}
	return cookie.Value
}

// getAccountNewはGET /account/newリクエストを組み立て、confirmationIDが空で
// なければ受け渡しCookieを付け、contextにロケールを設定する。
func getAccountNew(confirmationID string, locale model.Locale) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/account/new", nil)
	if confirmationID != "" {
		req.AddCookie(&http.Cookie{Name: session.EmailConfirmationCookieName, Value: confirmationID})
	}
	return req.WithContext(i18n.SetLocale(req.Context(), locale))
}

// TestNewは、受け渡しCookieがある場合にGET /account/newがHTTP 200と、
// アカウント作成フォーム (atname / password / password_confirmationフィールド・
// CSRF hiddenフィールド) を、サポートする各ロケールのローカライズ済み見出しと
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
		{name: "日本語", locale: model.LocaleJa, wantHeading: "アカウントを作成", wantLabel: "アットネーム"},
		{name: "英語", locale: model.LocaleEn, wantHeading: "Create your account", wantLabel: "Atname"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			handler.New(rec, getAccountNew(emailConfirmationToken(t, model.EmailConfirmationID(testutil.UnusedID)), tt.locale))

			if rec.Code != http.StatusOK {
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
			}
			if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
				t.Errorf("Content-Type = %q、期待値の接頭辞 = %q", got, "text/html")
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
					t.Errorf("レスポンスボディに %q が含まれていない", want)
				}
			}
		})
	}
}

// TestNew_NoCookieRedirectsToSignUpは、受け渡しCookieの無いGET /account/newが
// サインアップへリダイレクトすることを検証する。完了すべき進行中のサインアップが無い
// ためである。
func TestNew_NoCookieRedirectsToSignUp(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler := newAccountHandler(t, db)

	rec := httptest.NewRecorder()
	handler.New(rec, getAccountNew("", model.LocaleJa))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/sign_up" {
		t.Errorf("Location = %q、期待値 = %q", loc, "/sign_up")
	}
}
