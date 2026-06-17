package sign_up_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/dispatcher"
	"github.com/groobb/groobb/go/internal/handler/sign_up"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/query"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/validator"
)

// newSignUpHandler wires a sign-up Handler over transaction-bound repositories
// and a fake job inserter, so a handler test exercises the full request path
// (validator, UseCase, session cookie) against a real database. The inserter is
// returned too so a test can make enqueue fail.
//
// [Ja] newSignUpHandler はトランザクション束縛のリポジトリとフェイクのジョブ
// インサーターでサインアップ Handler を組み立て、ハンドラーテストが実 DB に対して
// リクエスト経路全体 (バリデーター・UseCase・セッション Cookie) を通すようにします。
// テストが enqueue を失敗させられるよう、インサーターも併せて返します。
func newSignUpHandler(t *testing.T, tx pgx.Tx) (*sign_up.Handler, *testutil.FakeJobInserter) {
	t.Helper()

	cfg := &config.Config{Env: "test"}
	queries := query.New(testutil.GetTestDB())
	userRepo := repository.NewUserRepository(queries).WithTx(tx)
	emailConfirmationRepo := repository.NewEmailConfirmationRepository(queries).WithTx(tx)

	inserter := &testutil.FakeJobInserter{}
	uc := usecase.NewCreateSignUpUsecase(
		validator.NewSignUpCreateValidator(userRepo),
		emailConfirmationRepo,
		dispatcher.NewDispatcher(inserter),
	)
	sessionMgr := session.NewManager(userRepo, cfg)
	return sign_up.NewHandler(cfg, sessionMgr, uc), inserter
}

// postSignUp builds a POST /sign_up request carrying the given email as form
// data, with the locale set in its context.
//
// [Ja] postSignUp は指定した email をフォームデータとして運ぶ POST /sign_up リクエストを
// 組み立て、context にロケールを設定します。
func postSignUp(email, locale string) *http.Request {
	form := url.Values{"email": {email}}
	req := httptest.NewRequest(http.MethodPost, "/sign_up", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req.WithContext(i18n.SetLocale(req.Context(), locale))
}

func findCookie(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// TestCreate_Success verifies that a valid new email redirects to the code-entry
// page and stores the confirmation id in the handoff cookie.
//
// [Ja] TestCreate_Success は、有効な新規メールがコード入力ページへリダイレクトし、
// 受け渡し Cookie に確認 id を保存することを検証します。
func TestCreate_Success(t *testing.T) {
	t.Parallel()

	_, tx := testutil.SetupTx(t)
	handler, _ := newSignUpHandler(t, tx)

	rec := httptest.NewRecorder()
	handler.Create(rec, postSignUp("new@example.com", i18n.LangJa))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/email_confirmation/new" {
		t.Errorf("Location = %q, want %q", loc, "/email_confirmation/new")
	}

	cookie := findCookie(rec, session.EmailConfirmationCookieName)
	if cookie == nil {
		t.Fatalf("メール確認 Cookie %q が設定されていない", session.EmailConfirmationCookieName)
	}
	if cookie.Value == "" {
		t.Error("メール確認 Cookie の値が空 (確認 id が運ばれていない)")
	}
}

// TestCreate_DuplicateEmail verifies that an already-registered email re-renders
// the form with 422 and the duplicate-email message, and sets no handoff cookie.
//
// [Ja] TestCreate_DuplicateEmail は、登録済みメールがフォームを 422 と重複メッセージ付きで
// 再描画し、受け渡し Cookie を設定しないことを検証します。
func TestCreate_DuplicateEmail(t *testing.T) {
	t.Parallel()

	_, tx := testutil.SetupTx(t)
	testutil.NewUserBuilder(t, tx).WithEmail("taken@example.com").Build()
	handler, _ := newSignUpHandler(t, tx)

	rec := httptest.NewRecorder()
	handler.Create(rec, postSignUp("taken@example.com", i18n.LangJa))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "このメールアドレスは既に使用されています") {
		t.Error("重複メールのエラーメッセージが描画されていない")
	}
	// The accessible-error markup must accompany the message so screen readers
	// announce it and associate it with the input.
	//
	// [Ja] スクリーンリーダーがメッセージを読み上げ、入力欄に関連付けられるよう、
	// アクセシブルなエラーマークアップがメッセージに伴っていること。
	if !strings.Contains(body, `aria-invalid="true"`) {
		t.Error("エラー時の入力欄に aria-invalid='true' が無い")
	}
	if !strings.Contains(body, `role="alert"`) {
		t.Error("エラーメッセージに role='alert' が無い")
	}
	if cookie := findCookie(rec, session.EmailConfirmationCookieName); cookie != nil && cookie.Value != "" {
		t.Error("バリデーションエラー時は受け渡し Cookie を設定すべきでない")
	}
}

// TestCreate_EnqueueFailure verifies that when the confirmation mail cannot be
// enqueued, Create re-renders the sign-up form with 500 and a form-wide error
// (the retry path), echoes the email back, and sets no handoff cookie.
//
// [Ja] TestCreate_EnqueueFailure は、確認メールを投入できないとき、Create が
// サインアップフォームを 500 とフォーム全体のエラー (再申請導線) で再描画し、email を
// エコーバックし、受け渡し Cookie を設定しないことを検証します。
func TestCreate_EnqueueFailure(t *testing.T) {
	t.Parallel()

	_, tx := testutil.SetupTx(t)
	handler, inserter := newSignUpHandler(t, tx)
	inserter.Err = errors.New("queue unavailable")

	rec := httptest.NewRecorder()
	handler.Create(rec, postSignUp("new@example.com", i18n.LangJa))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	body := rec.Body.String()
	// The sign-up form is re-rendered (the retry path) with the email preserved
	// and a form-wide alert carrying the user-safe message.
	//
	// [Ja] サインアップフォームが再描画され (再申請導線)、email が保持され、ユーザー
	// 安全なメッセージを載せたフォーム全体のアラートが伴うこと。
	if !strings.Contains(body, `action="/sign_up"`) {
		t.Error("再申請フォームが再描画されていない")
	}
	if !strings.Contains(body, `value="new@example.com"`) {
		t.Error("入力した email がエコーバックされていない")
	}
	if !strings.Contains(body, `role="alert"`) {
		t.Error("フォーム全体のエラーに role='alert' が無い")
	}
	if !strings.Contains(body, "確認コードの送信に失敗しました") {
		t.Error("フォーム全体のエラーメッセージが描画されていない")
	}
	if cookie := findCookie(rec, session.EmailConfirmationCookieName); cookie != nil && cookie.Value != "" {
		t.Error("enqueue 失敗時は受け渡し Cookie を設定すべきでない")
	}
}
