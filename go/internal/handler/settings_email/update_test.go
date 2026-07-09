package settings_email_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/groobb/groobb/go/internal/auth"
	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/dispatcher"
	"github.com/groobb/groobb/go/internal/handler/settings_email"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/query"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/validator"
)

// newSettingsEmailHandler wires a settings_email Handler over the shared pool with
// a fake job inserter, returning the inserter too so a test can assert what was
// enqueued or force an enqueue failure. The CreateEmailChangeUsecase opens its own
// transaction, so its tests commit rows and use unique emails (the test database
// is reset by make test) rather than the rolled-back transaction pattern.
//
// [Ja] newSettingsEmailHandler は共有プール上に settings_email Handler をフェイクの
// ジョブインサーターで組み立て、投入内容を検証したり enqueue 失敗を強制したりできるよう
// インサーターも返します。CreateEmailChangeUsecase は自前のトランザクションを開くため、
// そのテストはロールバックされるトランザクションパターンではなく、行をコミットしユニークな
// email を使います (テスト DB は make test がリセットする)。
func newSettingsEmailHandler(t *testing.T) (*settings_email.Handler, *testutil.FakeJobInserter) {
	t.Helper()

	cfg := &config.Config{Env: "test"}
	queries := query.New(testutil.GetTestDB())
	userRepo := repository.NewUserRepository(queries)
	userPasswordRepo := repository.NewUserPasswordRepository(queries)
	emailConfirmationRepo := repository.NewEmailConfirmationRepository(queries)

	inserter := &testutil.FakeJobInserter{}
	uc := usecase.NewCreateEmailChangeUsecase(
		testutil.GetTestDB(),
		validator.NewSettingsEmailUpdateValidator(userRepo, userPasswordRepo),
		emailConfirmationRepo,
		dispatcher.NewDispatcher(inserter),
	)
	return settings_email.NewHandler(cfg, uc), inserter
}

// seedUserWithPassword creates a committed user with the given email and the
// password "password123", returning the user model so an Update test can place it
// in the request context (as RequireAuth would) and authenticate against it.
//
// [Ja] seedUserWithPassword は指定 email とパスワード "password123" を持つコミット済み
// ユーザーを作成し、ユーザーモデルを返す。Update テストが (RequireAuth がするように) それを
// リクエスト context に載せ、それに対して認証できるようにする。
func seedUserWithPassword(t *testing.T, email string) *model.User {
	t.Helper()

	ctx := context.Background()
	queries := query.New(testutil.GetTestDB())
	userRepo := repository.NewUserRepository(queries)
	userPasswordRepo := repository.NewUserPasswordRepository(queries)

	user, err := userRepo.Create(ctx, repository.CreateUserInput{
		Email:    email,
		Atname:   testutil.UniqueAtname(),
		Locale:   "ja",
		TimeZone: "Asia/Tokyo",
	})
	if err != nil {
		t.Fatalf("テスト用ユーザーの作成に失敗: %v", err)
	}
	digest, err := auth.HashPassword("password123")
	if err != nil {
		t.Fatalf("パスワードのハッシュ化に失敗: %v", err)
	}
	if _, err := userPasswordRepo.Create(ctx, repository.CreateUserPasswordInput{
		UserID:         user.ID,
		PasswordDigest: digest,
	}); err != nil {
		t.Fatalf("テスト用パスワードの作成に失敗: %v", err)
	}
	return user
}

// patchSettingsEmail builds a PATCH /settings/email request carrying the new email
// and current password as form data, with the user in the context (as RequireAuth
// would place it) and the locale set.
//
// [Ja] patchSettingsEmail は新しい email と現在のパスワードをフォームデータとして運ぶ
// PATCH /settings/email リクエストを組み立て、(RequireAuth が置くように) ユーザーを context に
// 載せ、ロケールを設定する。
func patchSettingsEmail(user *model.User, newEmail, currentPassword, locale string) *http.Request {
	form := url.Values{
		"email":            {newEmail},
		"current_password": {currentPassword},
	}
	req := httptest.NewRequest(http.MethodPatch, "/settings/email", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	ctx := i18n.SetLocale(req.Context(), locale)
	ctx = middleware.SetUserToContext(ctx, user)
	return req.WithContext(ctx)
}

// TestUpdate_Success verifies that a valid new email plus the correct current
// password issues a confirmation for the new address, enqueues the mail, and
// redirects to the code-entry step.
//
// [Ja] TestUpdate_Success は、有効な新しい email と正しい現在のパスワードが、新しい
// アドレスの確認を発行し、メールを投入し、コード入力ステップへリダイレクトすることを
// 検証する。
func TestUpdate_Success(t *testing.T) {
	t.Parallel()

	handler, inserter := newSettingsEmailHandler(t)
	user := seedUserWithPassword(t, fmt.Sprintf("ec-h-cur-%s@example.com", uuid.NewString()))
	newEmail := fmt.Sprintf("ec-h-new-%s@example.com", uuid.NewString())

	rec := httptest.NewRecorder()
	handler.Update(rec, patchSettingsEmail(user, newEmail, "password123", i18n.LangJa))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/settings/email/confirmation/new" {
		t.Errorf("Location = %q, want %q", loc, "/settings/email/confirmation/new")
	}
	if !inserter.Called {
		t.Error("確認メールが投入されていない")
	}

	active, err := repository.NewEmailConfirmationRepository(query.New(testutil.GetTestDB())).FindActiveEmailChangeByUserID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("FindActiveEmailChangeByUserID() error = %v", err)
	}
	if active == nil || active.Email != newEmail {
		t.Errorf("保留中のメール変更確認 = %v, want アドレス %q", active, newEmail)
	}
}

// TestUpdate_ValidationError verifies that a wrong current password re-renders the
// form with 422 and the incorrect-password message, does not enqueue the mail, and
// creates no confirmation.
//
// [Ja] TestUpdate_ValidationError は、誤った現在のパスワードがフォームを 422 と
// パスワード誤りのメッセージで再描画し、メールを投入せず、確認を作成しないことを
// 検証する。
func TestUpdate_ValidationError(t *testing.T) {
	t.Parallel()

	handler, inserter := newSettingsEmailHandler(t)
	user := seedUserWithPassword(t, fmt.Sprintf("ec-h-ve-%s@example.com", uuid.NewString()))
	newEmail := fmt.Sprintf("ec-h-ve-new-%s@example.com", uuid.NewString())

	rec := httptest.NewRecorder()
	handler.Update(rec, patchSettingsEmail(user, newEmail, "wrongpassword", i18n.LangJa))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "現在のパスワードが正しくありません") {
		t.Error("現在パスワード誤りのエラーメッセージが描画されていない")
	}
	// The accessible-error markup must accompany the message so screen readers
	// announce it and associate it with the input.
	//
	// [Ja] スクリーンリーダーがメッセージを読み上げ、入力欄に関連付けられるよう、
	// アクセシブルなエラーマークアップがメッセージに伴っていること。
	if !strings.Contains(body, `aria-invalid="true"`) {
		t.Error("エラー時の入力欄に aria-invalid='true' が無い")
	}
	// The attempted new email is echoed back so the user does not retype it.
	//
	// [Ja] 試した新しい email はエコーバックされること。
	if !strings.Contains(body, `value="`+newEmail+`"`) {
		t.Error("入力した新しい email がエコーバックされていない")
	}
	if inserter.Called {
		t.Error("バリデーション失敗時に確認メールが投入された")
	}

	active, err := repository.NewEmailConfirmationRepository(query.New(testutil.GetTestDB())).FindActiveEmailChangeByUserID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("FindActiveEmailChangeByUserID() error = %v", err)
	}
	if active != nil {
		t.Error("バリデーション失敗時にメール変更確認が作成された")
	}
}

// TestUpdate_EnqueueFailure verifies that when the confirmation mail cannot be
// enqueued, Update re-renders the form with 500 and a form-wide error (the retry
// path) and echoes the attempted new email back.
//
// [Ja] TestUpdate_EnqueueFailure は、確認メールを投入できないとき、Update がフォームを
// 500 とフォーム全体のエラー (再申請導線) で再描画し、試した新しい email をエコーバック
// することを検証する。
func TestUpdate_EnqueueFailure(t *testing.T) {
	t.Parallel()

	handler, inserter := newSettingsEmailHandler(t)
	inserter.Err = errors.New("queue unavailable")
	user := seedUserWithPassword(t, fmt.Sprintf("ec-h-enq-%s@example.com", uuid.NewString()))
	newEmail := fmt.Sprintf("ec-h-enq-new-%s@example.com", uuid.NewString())

	rec := httptest.NewRecorder()
	handler.Update(rec, patchSettingsEmail(user, newEmail, "password123", i18n.LangJa))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `action="/settings/email"`) {
		t.Error("再申請フォームが再描画されていない")
	}
	if !strings.Contains(body, `role="alert"`) {
		t.Error("フォーム全体のエラーに role='alert' が無い")
	}
	if !strings.Contains(body, "確認コードの送信に失敗しました") {
		t.Error("フォーム全体のエラーメッセージが描画されていない")
	}
	if !strings.Contains(body, `value="`+newEmail+`"`) {
		t.Error("入力した新しい email がエコーバックされていない")
	}
}
