package sign_up_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/dispatcher"
	"github.com/groobb/groobb/go/internal/handler/sign_up"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/validator"
)

// newSignUpHandler wires a sign-up Handler over the test database's repositories,
// a fake job inserter, and a Turnstile verifier that passes by default, so a
// handler test exercises the full request path (Turnstile gate, validator,
// UseCase, session cookie) against a real database. The inserter and verifier are
// returned too so a test can make enqueue fail or make Turnstile verification
// fail.
//
// [Ja] newSignUpHandler はテスト用データベースのリポジトリ、フェイクのジョブ
// インサーター、既定で通過する Turnstile 検証器でサインアップ Handler を組み立て、
// ハンドラーテストが実 DB に対してリクエスト経路全体 (Turnstile ゲート・バリデーター・
// UseCase・セッション Cookie) を通すようにします。テストが enqueue や Turnstile 検証を
// 失敗させられるよう、インサーターと検証器も併せて返します。
func newSignUpHandler(t *testing.T, db *database.DB) (*sign_up.Handler, *testutil.FakeJobInserter, *testutil.FakeTurnstileVerifier) {
	t.Helper()

	cfg := testutil.NewTestConfig(t)
	userRepo := repository.NewUserRepository(db)
	emailConfirmationRepo := repository.NewEmailConfirmationRepository(db)

	inserter := &testutil.FakeJobInserter{}
	uc := usecase.NewCreateSignUpUsecase(
		validator.NewSignUpCreateValidator(userRepo),
		emailConfirmationRepo,
		dispatcher.NewDispatcher(inserter),
	)
	sessionMgr := session.NewManager(userRepo, cfg)
	verifier := &testutil.FakeTurnstileVerifier{Passed: true}
	return sign_up.NewHandler(cfg, sessionMgr, uc, verifier), inserter, verifier
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

	db := testutil.SetupDB(t)
	handler, _, _ := newSignUpHandler(t, db)

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

	db := testutil.SetupDB(t)
	testutil.NewUserBuilder(t, db).WithEmail("taken@example.com").Build()
	handler, _, _ := newSignUpHandler(t, db)

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

	db := testutil.SetupDB(t)
	handler, inserter, _ := newSignUpHandler(t, db)
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

// TestCreate_TurnstileFailure verifies that when Turnstile verification does not
// pass — a non-pass or a siteverify error — Create stops the request at the bot
// gate: it re-renders the form with 422 and the form-wide Turnstile message,
// echoes the email back, forwards the submitted token to the verifier, does not
// enqueue the confirmation mail, and sets no handoff cookie.
//
// [Ja] TestCreate_TurnstileFailure は、Turnstile 検証が通過しないとき (非通過または
// siteverify エラー) に Create が Bot ゲートでリクエストを止めることを検証します。
// フォームを 422 とフォーム全体の Turnstile メッセージで再描画し、email をエコーバックし、
// 送信されたトークンを検証器へ渡し、確認メールを投入せず、受け渡し Cookie を設定しない
// ことを確認します。
func TestCreate_TurnstileFailure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		passed bool
		err    error
	}{
		{name: "非通過", passed: false, err: nil},
		{name: "検証エラー", passed: false, err: errors.New("siteverify unavailable")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			db := testutil.SetupDB(t)
			handler, inserter, verifier := newSignUpHandler(t, db)
			verifier.Passed = tt.passed
			verifier.Err = tt.err

			form := url.Values{
				"email":                 {"new@example.com"},
				"cf-turnstile-response": {"submitted-token"},
			}
			req := httptest.NewRequest(http.MethodPost, "/sign_up", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req = req.WithContext(i18n.SetLocale(req.Context(), i18n.LangJa))

			rec := httptest.NewRecorder()
			handler.Create(rec, req)

			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status code = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
			}
			body := rec.Body.String()
			if !strings.Contains(body, "ロボットでないことの確認に失敗しました") {
				t.Error("Turnstile 失敗のフォーム全体メッセージが描画されていない")
			}
			if !strings.Contains(body, `role="alert"`) {
				t.Error("フォーム全体のエラーに role='alert' が無い")
			}
			// The email is echoed back so the user does not have to retype it.
			//
			// [Ja] ユーザーが再入力しなくて済むよう email はエコーバックされること。
			if !strings.Contains(body, `value="new@example.com"`) {
				t.Error("入力した email がエコーバックされていない")
			}
			// The submitted token reached the verifier, confirming the handler read
			// the correct cf-turnstile-response field.
			//
			// [Ja] 送信されたトークンが検証器へ到達しており、ハンドラーが正しい
			// cf-turnstile-response フィールドを読んでいることを確認する。
			if verifier.Token != "submitted-token" {
				t.Errorf("verifier に渡ったトークン = %q, want %q", verifier.Token, "submitted-token")
			}
			// The bot gate must stop the request before the UseCase, so no
			// confirmation mail is enqueued.
			//
			// [Ja] Bot ゲートは UseCase の前でリクエストを止めるため、確認メールは
			// 投入されないこと。
			if inserter.Called {
				t.Error("Turnstile 失敗時に確認メールが投入された (UseCase に進んでしまっている)")
			}
			if cookie := findCookie(rec, session.EmailConfirmationCookieName); cookie != nil && cookie.Value != "" {
				t.Error("Turnstile 失敗時は受け渡し Cookie を設定すべきでない")
			}
		})
	}
}
