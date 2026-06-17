package password_reset_test

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
	"github.com/groobb/groobb/go/internal/dispatcher"
	"github.com/groobb/groobb/go/internal/handler/password_reset"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/query"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/validator"
)

// newPasswordResetHandler wires a password-reset Handler over the shared pool.
// The CreatePasswordResetTokenUsecase opens its own transaction, so its tests
// commit rows and use unique emails (the test database is reset by make test)
// rather than the rolled-back transaction pattern. The fake job inserter is
// returned so a test can assert whether the reset mail was enqueued.
//
// [Ja] newPasswordResetHandler は共有プールで password-reset Handler を組み立てる。
// CreatePasswordResetTokenUsecase は自前のトランザクションを開くため、そのテストは
// ロールバックされるトランザクションパターンではなく、行をコミットしユニークな email を
// 使う (テスト DB は make test がリセットする)。リセットメールが投入されたかをテストが
// 検証できるよう、フェイクのジョブインサーターを返す。
func newPasswordResetHandler(t *testing.T) (*password_reset.Handler, *testutil.FakeJobInserter) {
	t.Helper()

	cfg := &config.Config{Env: "test", AppURL: "https://groobb.example.dev"}
	db := testutil.GetTestDB()
	queries := query.New(db)
	userRepo := repository.NewUserRepository(queries)
	passwordResetTokenRepo := repository.NewPasswordResetTokenRepository(queries)

	inserter := &testutil.FakeJobInserter{}
	uc := usecase.NewCreatePasswordResetTokenUsecase(
		db,
		validator.NewPasswordResetCreateValidator(),
		userRepo,
		passwordResetTokenRepo,
		dispatcher.NewDispatcher(inserter),
		cfg,
	)
	return password_reset.NewHandler(cfg, uc), inserter
}

// seedUser creates a committed user with a unique email and returns the email.
//
// [Ja] seedUser はユニークな email を持つコミット済みユーザーを作成し、その email を返す。
func seedUser(t *testing.T) string {
	t.Helper()

	email := fmt.Sprintf("pwreset-h-%s@example.com", uuid.NewString())
	userRepo := repository.NewUserRepository(query.New(testutil.GetTestDB()))
	if _, err := userRepo.Create(context.Background(), repository.CreateUserInput{
		Email:    email,
		Locale:   "ja",
		TimeZone: "Asia/Tokyo",
	}); err != nil {
		t.Fatalf("テスト用ユーザーの作成に失敗: %v", err)
	}
	return email
}

// postPasswordReset builds a POST /password_reset request carrying the given
// email as form data, with the locale set in its context.
//
// [Ja] postPasswordReset は指定した email をフォームデータとして運ぶ
// POST /password_reset リクエストを組み立て、context にロケールを設定する。
func postPasswordReset(email, locale string) *http.Request {
	form := url.Values{"email": {email}}
	req := httptest.NewRequest(http.MethodPost, "/password_reset", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req.WithContext(i18n.SetLocale(req.Context(), locale))
}

// TestCreate_KnownEmail verifies that a registered email renders the sent
// confirmation (200) and enqueues the reset mail.
//
// [Ja] TestCreate_KnownEmail は、登録済みの email が送信済み確認 (200) を描画し、
// リセットメールを投入することを検証する。
func TestCreate_KnownEmail(t *testing.T) {
	t.Parallel()

	handler, inserter := newPasswordResetHandler(t)
	email := seedUser(t)

	rec := httptest.NewRecorder()
	handler.Create(rec, postPasswordReset(email, i18n.LangJa))

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), "メールをご確認ください") {
		t.Error("送信済み確認ページが描画されていない")
	}
	if !inserter.Called {
		t.Error("登録済み email ではリセットメールを投入すべき")
	}
	if _, ok := inserter.Args.(dispatcher.SendPasswordResetArgs); !ok {
		t.Errorf("投入ジョブの型 = %T, want SendPasswordResetArgs", inserter.Args)
	}
}

// TestCreate_UnknownEmail verifies the enumeration-safe path: an unregistered
// email renders the same sent confirmation (200) but enqueues no mail, so the
// response is indistinguishable from the known-email case.
//
// [Ja] TestCreate_UnknownEmail は列挙攻撃対策の経路を検証する。未登録の email は同じ
// 送信済み確認 (200) を描画するがメールは投入しないため、レスポンスは登録済みの場合と
// 区別できない。
func TestCreate_UnknownEmail(t *testing.T) {
	t.Parallel()

	handler, inserter := newPasswordResetHandler(t)
	email := fmt.Sprintf("nobody-h-%s@example.com", uuid.NewString())

	rec := httptest.NewRecorder()
	handler.Create(rec, postPasswordReset(email, i18n.LangJa))

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), "メールをご確認ください") {
		t.Error("送信済み確認ページが描画されていない (未登録でも同じ応答であるべき)")
	}
	if inserter.Called {
		t.Error("未登録 email ではメールを投入すべきでない")
	}
}

// TestCreate_InvalidEmail verifies that a malformed email re-renders the form
// (422) with the format error and enqueues no mail.
//
// [Ja] TestCreate_InvalidEmail は、形式不正の email がフォームを 422 で形式エラー付きに
// 再描画し、メールを投入しないことを検証する。
func TestCreate_InvalidEmail(t *testing.T) {
	t.Parallel()

	handler, inserter := newPasswordResetHandler(t)

	rec := httptest.NewRecorder()
	handler.Create(rec, postPasswordReset("not-an-email", i18n.LangJa))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `action="/password_reset"`) {
		t.Error("申請フォームが再描画されていない")
	}
	if !strings.Contains(body, "正しいメールアドレスを入力してください") {
		t.Error("email 形式エラーのメッセージが描画されていない")
	}
	if !strings.Contains(body, `aria-invalid="true"`) {
		t.Error("エラー時の入力欄に aria-invalid='true' が無い")
	}
	if inserter.Called {
		t.Error("形式不正の email ではメールを投入すべきでない")
	}
}
