package email_confirmation_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/handler/email_confirmation"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/query"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/validator"
)

// newEmailConfirmationHandler wires an email-confirmation Handler over the shared
// pool (not a rolled-back transaction) because the verify UseCase opens its own
// transaction internally; an outer transaction's seed rows would be invisible to
// that inner transaction. Handler tests seed committed confirmations with unique
// emails (the test database is reset by make test). This exercises the full
// request path (validator, UseCase, session cookie) against a real database.
//
// [Ja] newEmailConfirmationHandler は共有プール (ロールバックされるトランザクションでは
// なく) でメール確認 Handler を組み立てる。verify UseCase は内部で自前のトランザクションを
// 開くため、外側トランザクションで仕込んだ行はその内側トランザクションから見えないからで
// ある。ハンドラーテストはユニークな email でコミット済みの確認を仕込む (テスト DB は
// make test がリセットする)。これによりリクエスト経路全体 (バリデーター・UseCase・
// セッション Cookie) を実 DB に対して通す。
func newEmailConfirmationHandler(t *testing.T) *email_confirmation.Handler {
	t.Helper()

	cfg := &config.Config{Env: "test"}
	db := testutil.GetTestDB()
	queries := query.New(db)
	userRepo := repository.NewUserRepository(queries)
	emailConfirmationRepo := repository.NewEmailConfirmationRepository(queries)

	uc := usecase.NewVerifyEmailConfirmationUsecase(
		db,
		validator.NewEmailConfirmationCreateValidator(emailConfirmationRepo),
		emailConfirmationRepo,
	)
	sessionMgr := session.NewManager(userRepo, cfg)
	return email_confirmation.NewHandler(cfg, sessionMgr, uc)
}

// seedActiveConfirmation creates a committed, active sign-up confirmation with
// the given code (and a unique email) and returns its id, so a handler test can
// drive code verification against a real row.
//
// [Ja] seedActiveConfirmation は指定コード (とユニークな email) のコミット済み・
// アクティブなサインアップ確認を作成し、その id を返す。ハンドラーテストが実在の行に対して
// コード検証を駆動できるようにする。
func seedActiveConfirmation(t *testing.T, code string) model.EmailConfirmationID {
	t.Helper()

	ctx := context.Background()
	repo := repository.NewEmailConfirmationRepository(query.New(testutil.GetTestDB()))
	confirmation, err := repo.Create(ctx, repository.CreateEmailConfirmationInput{
		Email: fmt.Sprintf("ec-%s@example.com", uuid.NewString()),
		Event: model.EmailConfirmationEventSignUp,
		Code:  code,
	})
	if err != nil {
		t.Fatalf("確認の作成に失敗: %v", err)
	}
	return confirmation.ID
}

// getNew builds a GET /email_confirmation/new request, attaching the handoff
// cookie when confirmationID is non-empty, with the locale set in its context.
//
// [Ja] getNew は GET /email_confirmation/new リクエストを組み立て、confirmationID が
// 空でなければ受け渡し Cookie を付け、context にロケールを設定する。
func getNew(confirmationID, locale string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/email_confirmation/new", nil)
	if confirmationID != "" {
		req.AddCookie(&http.Cookie{Name: session.EmailConfirmationCookieName, Value: confirmationID})
	}
	return req.WithContext(i18n.SetLocale(req.Context(), locale))
}

// TestNew verifies that GET /email_confirmation/new returns HTTP 200 with the
// code-entry form (code field and CSRF hidden field) and the localized heading
// for each supported locale, when the handoff cookie is present.
//
// [Ja] TestNew は、受け渡し Cookie がある場合に GET /email_confirmation/new が HTTP 200
// と、コード入力フォーム (code フィールド・CSRF hidden フィールド) を、サポートする各
// ロケールのローカライズ済み見出しとともに返すことを検証する。
func TestNew(t *testing.T) {
	t.Parallel()

	handler := newEmailConfirmationHandler(t)

	tests := []struct {
		name        string
		locale      string
		wantHeading string
	}{
		{name: "Japanese", locale: i18n.LangJa, wantHeading: "確認コードを入力"},
		{name: "English", locale: i18n.LangEn, wantHeading: "Enter your confirmation code"},
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
				`action="/email_confirmation"`,
				`method="POST"`,
				`name="csrf_token"`,
				`name="code"`,
				`<label for="code"`,
			}
			for _, want := range wants {
				if !strings.Contains(body, want) {
					t.Errorf("response body does not contain %q", want)
				}
			}
		})
	}
}

// TestNew_NoCookieRedirectsToSignUp verifies that GET /email_confirmation/new
// without the handoff cookie redirects to sign-up, since there is no pending
// confirmation to enter a code for.
//
// [Ja] TestNew_NoCookieRedirectsToSignUp は、受け渡し Cookie の無い
// GET /email_confirmation/new がサインアップへリダイレクトすることを検証する。コードを
// 入力すべき保留中の確認が無いためである。
func TestNew_NoCookieRedirectsToSignUp(t *testing.T) {
	t.Parallel()

	handler := newEmailConfirmationHandler(t)

	rec := httptest.NewRecorder()
	handler.New(rec, getNew("", i18n.LangJa))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/sign_up" {
		t.Errorf("Location = %q, want %q", loc, "/sign_up")
	}
}
