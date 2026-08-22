package password_reset_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/dispatcher"
	"github.com/groobb/groobb/go/internal/handler/password_reset"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/validator"
)

// newPasswordResetHandler wires a password-reset Handler over the test database's
// repositories, with a Turnstile verifier that passes by default, so a handler
// test exercises the full request path (Turnstile gate, validator, UseCase)
// against a real database. The fake job inserter and verifier are returned so a
// test can assert whether the reset mail was enqueued and can make Turnstile
// verification fail.
//
// [Ja] newPasswordResetHandler はテスト用データベースのリポジトリで、既定で通過する
// Turnstile 検証器を伴って password-reset Handler を組み立て、ハンドラーテストが実 DB に
// 対してリクエスト経路全体 (Turnstile ゲート・バリデーター・UseCase) を通すようにする。
// リセットメールが投入されたかをテストが検証でき、Turnstile 検証を失敗させられるよう、
// フェイクのジョブインサーターと検証器を返す。
func newPasswordResetHandler(t *testing.T, db *database.DB) (*password_reset.Handler, *testutil.FakeJobInserter, *testutil.FakeTurnstileVerifier) {
	t.Helper()

	cfg := &config.Config{Env: "test", AppURL: "https://groobb.example.dev"}
	userRepo := repository.NewUserRepository(db)
	passwordResetTokenRepo := repository.NewPasswordResetTokenRepository(db)

	inserter := &testutil.FakeJobInserter{}
	uc := usecase.NewCreatePasswordResetTokenUsecase(
		db.Writer,
		validator.NewPasswordResetCreateValidator(),
		userRepo,
		passwordResetTokenRepo,
		dispatcher.NewDispatcher(inserter),
		cfg,
	)
	verifier := &testutil.FakeTurnstileVerifier{Passed: true}
	return password_reset.NewHandler(cfg, uc, verifier), inserter, verifier
}

// seedUser creates a committed user with a unique email and returns the email.
//
// [Ja] seedUser はユニークな email を持つコミット済みユーザーを作成し、その email を返す。
func seedUser(t *testing.T, db *database.DB) string {
	t.Helper()

	email := "pwreset-h@example.com"
	userRepo := repository.NewUserRepository(db)
	if _, err := userRepo.Create(context.Background(), repository.CreateUserInput{
		Email:    email,
		Atname:   testutil.UniqueAtname(db),
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

	db := testutil.SetupDB(t)

	handler, inserter, _ := newPasswordResetHandler(t, db)
	email := seedUser(t, db)

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

	db := testutil.SetupDB(t)

	handler, inserter, _ := newPasswordResetHandler(t, db)
	email := "nobody-h@example.com"

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

	db := testutil.SetupDB(t)

	handler, inserter, _ := newPasswordResetHandler(t, db)

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

// TestCreate_TurnstileFailure verifies that when Turnstile verification does not
// pass — a non-pass or a siteverify error — Create stops the request at the bot
// gate: it re-renders the request form with 422 and the form-wide Turnstile
// message (not the enumeration-safe sent page), echoes the email back, forwards
// the submitted token to the verifier, and does not enqueue the reset mail. No
// user is seeded because the gate runs before any account lookup.
//
// [Ja] TestCreate_TurnstileFailure は、Turnstile 検証が通過しないとき (非通過または
// siteverify エラー) に Create が Bot ゲートでリクエストを止めることを検証する。
// 申請フォームを 422 とフォーム全体の Turnstile メッセージで (列挙対策の送信済みページ
// ではなく) 再描画し、email をエコーバックし、送信されたトークンを検証器へ渡し、リセット
// メールを投入しないことを確認する。ゲートはアカウント検索の前に走るため、ユーザーは
// 作成しない。
func TestCreate_TurnstileFailure(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

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

			handler, inserter, verifier := newPasswordResetHandler(t, db)
			verifier.Passed = tt.passed
			verifier.Err = tt.err

			form := url.Values{
				"email":                 {"user@example.com"},
				"cf-turnstile-response": {"submitted-token"},
			}
			req := httptest.NewRequest(http.MethodPost, "/password_reset", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req = req.WithContext(i18n.SetLocale(req.Context(), i18n.LangJa))

			rec := httptest.NewRecorder()
			handler.Create(rec, req)

			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status code = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
			}
			body := rec.Body.String()
			// The request form is re-rendered (not the enumeration-safe sent page),
			// carrying the form-wide Turnstile message.
			//
			// [Ja] 申請フォームが (列挙対策の送信済みページではなく) 再描画され、
			// フォーム全体の Turnstile メッセージを載せていること。
			if !strings.Contains(body, `action="/password_reset"`) {
				t.Error("申請フォームが再描画されていない")
			}
			if !strings.Contains(body, "ロボットでないことの確認に失敗しました") {
				t.Error("Turnstile 失敗のフォーム全体メッセージが描画されていない")
			}
			if !strings.Contains(body, `role="alert"`) {
				t.Error("フォーム全体のエラーに role='alert' が無い")
			}
			// The email is echoed back so the user does not have to retype it.
			//
			// [Ja] ユーザーが再入力しなくて済むよう email はエコーバックされること。
			if !strings.Contains(body, `value="user@example.com"`) {
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
			// No reset mail is enqueued. This is a supplementary check: for this
			// unknown email the UseCase would not enqueue mail even if it ran, so on
			// its own it cannot detect a gate bypass — the 422 + form re-render
			// assertion above does that (a bypass would reach the UseCase and return
			// the 200 sent page). It is kept as a guard that the gate stops before any
			// UseCase side effect.
			//
			// [Ja] リセットメールが投入されないこと。これは補助的なチェックである。この
			// 未知の email では UseCase が走ってもメールを投入しないため、この検証単独では
			// ゲートの迂回を検出できない。迂回は上の 422 + フォーム再描画のアサーションが
			// 捕捉する (迂回されれば UseCase に到達し 200 の送信済みページを返す)。ゲートが
			// UseCase の副作用より前で止まることの担保として残す。
			if inserter.Called {
				t.Error("Turnstile 失敗時にリセットメールが投入された (UseCase に進んでしまっている)")
			}
		})
	}
}
