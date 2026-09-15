package email_confirmation_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/handler/email_confirmation"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/validator"
)

// newEmailConfirmationHandlerはテスト用データベースのリポジトリでメール確認
// Handlerを組み立てる。ハンドラーテストがリクエスト経路全体 (バリデーター・UseCase・
// セッションCookie) を実DBに対して通せるようにするためである。
func newEmailConfirmationHandler(t *testing.T, db *database.DB) *email_confirmation.Handler {
	t.Helper()

	cfg := testutil.NewTestConfig(t)
	userRepo := repository.NewUserRepository(db)
	emailConfirmationRepo := repository.NewEmailConfirmationRepository(db)

	uc := usecase.NewVerifyEmailConfirmationUsecase(
		db.Writer,
		validator.NewEmailConfirmationCreateValidator(emailConfirmationRepo),
		emailConfirmationRepo,
	)
	sessionMgr := session.NewManager(userRepo, cfg)
	return email_confirmation.NewHandler(cfg, sessionMgr, uc)
}

// emailConfirmationTokenはサインアップフローがメール確認Cookieへ格納するものと
// 同じ署名付きcontinuation tokenを発行します。
func emailConfirmationToken(t *testing.T, id model.EmailConfirmationID) string {
	t.Helper()

	mgr := session.NewManager(nil, testutil.NewTestConfig(t))
	rec := httptest.NewRecorder()
	mgr.SetEmailConfirmationID(rec, id)
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == session.EmailConfirmationCookieName && cookie.Value != "" {
			return cookie.Value
		}
	}
	t.Fatalf("メール確認Cookie %q の署名tokenが発行されていない", session.EmailConfirmationCookieName)
	return ""
}

// seedActiveConfirmationは指定コード (とユニークなemail) のコミット済み・
// アクティブなサインアップ確認を作成し、そのidを返す。ハンドラーテストが実在の行に対して
// コード検証を駆動できるようにする。
func seedActiveConfirmation(t *testing.T, db *database.DB, code string) model.EmailConfirmationID {
	t.Helper()

	ctx := context.Background()
	repo := repository.NewEmailConfirmationRepository(db)
	confirmation, err := repo.Create(ctx, repository.CreateEmailConfirmationInput{
		Email: "ec@example.com",
		Event: model.EmailConfirmationEventSignUp,
		Code:  code,
	})
	if err != nil {
		t.Fatalf("確認の作成に失敗: %v", err)
	}
	return confirmation.ID
}

// getNewはGET /email_confirmation/newリクエストを組み立て、confirmationIDが
// 空でなければ受け渡しCookieを付け、contextにロケールを設定する。
func getNew(confirmationID string, locale model.Locale) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/email_confirmation/new", nil)
	if confirmationID != "" {
		req.AddCookie(&http.Cookie{Name: session.EmailConfirmationCookieName, Value: confirmationID})
	}
	return req.WithContext(i18n.SetLocale(req.Context(), locale))
}

// TestNewは、受け渡しCookieがある場合にGET /email_confirmation/newがHTTP 200
// と、コード入力フォーム (codeフィールド・CSRF hiddenフィールド) を、サポートする各
// ロケールのローカライズ済み見出しとともに返すことを検証する。
func TestNew(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler := newEmailConfirmationHandler(t, db)

	tests := []struct {
		name        string
		locale      model.Locale
		wantHeading string
	}{
		{name: "日本語", locale: model.LocaleJa, wantHeading: "確認コードを入力"},
		{name: "英語", locale: model.LocaleEn, wantHeading: "Enter your confirmation code"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			handler.New(rec, getNew(emailConfirmationToken(t, model.EmailConfirmationID(testutil.UnusedID)), tt.locale))

			if rec.Code != http.StatusOK {
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
			}
			if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
				t.Errorf("Content-Type = %q、期待値の接頭辞 = %q", got, "text/html")
			}

			body := rec.Body.String()
			wants := []string{
				tt.wantHeading,
				`action="/email_confirmation"`,
				`method="POST"`,
				`name="csrf_token"`,
				`name="code"`,
				`<label for="code"`,
			}
			for _, want := range wants {
				if !strings.Contains(body, want) {
					t.Errorf("レスポンスボディに %q が含まれていない", want)
				}
			}
		})
	}
}

// TestNew_NoCookieRedirectsToSignUpは、受け渡しCookieの無い
// GET /email_confirmation/newがサインアップへリダイレクトすることを検証する。コードを
// 入力すべき保留中の確認が無いためである。
func TestNew_NoCookieRedirectsToSignUp(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler := newEmailConfirmationHandler(t, db)

	rec := httptest.NewRecorder()
	handler.New(rec, getNew("", model.LocaleJa))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/sign_up" {
		t.Errorf("Location = %q、期待値 = %q", loc, "/sign_up")
	}
}
