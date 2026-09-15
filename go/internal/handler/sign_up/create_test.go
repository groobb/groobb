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
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/validator"
)

// newSignUpHandlerはテスト用データベースのリポジトリ、フェイクのジョブ
// インサーター、既定で通過するTurnstile検証器でサインアップHandlerを組み立て、
// ハンドラーテストが実DBに対してリクエスト経路全体 (Turnstileゲート・バリデーター・
// UseCase・セッションCookie) を通すようにします。テストがenqueueやTurnstile検証を
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

// postSignUpは指定したemailをフォームデータとして運ぶPOST /sign_upリクエストを
// 組み立て、contextにロケールを設定します。
func postSignUp(email string, locale model.Locale) *http.Request {
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

// TestCreate_Successは、有効な新規メールがコード入力ページへリダイレクトし、
// 受け渡しCookieに確認idを保存することを検証します。
func TestCreate_Success(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	handler, _, _ := newSignUpHandler(t, db)

	rec := httptest.NewRecorder()
	handler.Create(rec, postSignUp("new@example.com", model.LocaleJa))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/email_confirmation/new" {
		t.Errorf("Location = %q、期待値 = %q", loc, "/email_confirmation/new")
	}

	cookie := findCookie(rec, session.EmailConfirmationCookieName)
	if cookie == nil {
		t.Fatalf("メール確認Cookie %q が設定されていない", session.EmailConfirmationCookieName)
	}
	if cookie.Value == "" {
		t.Error("メール確認Cookieの値が空 (確認idが運ばれていない)")
	}
}

// TestCreate_DuplicateEmailは、登録済みメールがフォームを422と重複メッセージ付きで
// 再描画し、受け渡しCookieを設定しないことを検証します。
func TestCreate_DuplicateEmail(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	testutil.NewUserBuilder(t, db).WithEmail("taken@example.com").Build()
	handler, _, _ := newSignUpHandler(t, db)

	rec := httptest.NewRecorder()
	handler.Create(rec, postSignUp("taken@example.com", model.LocaleJa))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusUnprocessableEntity)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "このメールアドレスは既に使用されています") {
		t.Error("重複メールのエラーメッセージが描画されていない")
	}
	// スクリーンリーダーがメッセージを読み上げ、入力欄に関連付けられるよう、
	// アクセシブルなエラーマークアップがメッセージに伴っていること。
	if !strings.Contains(body, `aria-invalid="true"`) {
		t.Error("エラー時の入力欄にaria-invalid='true' が無い")
	}
	if !strings.Contains(body, `role="alert"`) {
		t.Error("エラーメッセージにrole='alert' が無い")
	}
	if cookie := findCookie(rec, session.EmailConfirmationCookieName); cookie != nil && cookie.Value != "" {
		t.Error("バリデーションエラー時は受け渡しCookieを設定すべきでない")
	}
}

// TestCreate_EnqueueFailureは、確認メールを投入できないとき、Createが
// サインアップフォームを500とフォーム全体のエラー (再申請導線) で再描画し、emailを
// エコーバックし、受け渡しCookieを設定しないことを検証します。
func TestCreate_EnqueueFailure(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	handler, inserter, _ := newSignUpHandler(t, db)
	inserter.Err = errors.New("queue unavailable")

	rec := httptest.NewRecorder()
	handler.Create(rec, postSignUp("new@example.com", model.LocaleJa))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusInternalServerError)
	}
	body := rec.Body.String()
	// サインアップフォームが再描画され (再申請導線)、emailが保持され、ユーザー
	// 安全なメッセージを載せたフォーム全体のアラートが伴うこと。
	if !strings.Contains(body, `action="/sign_up"`) {
		t.Error("再申請フォームが再描画されていない")
	}
	if !strings.Contains(body, `value="new@example.com"`) {
		t.Error("入力したemailがエコーバックされていない")
	}
	if !strings.Contains(body, `role="alert"`) {
		t.Error("フォーム全体のエラーにrole='alert' が無い")
	}
	if !strings.Contains(body, "確認コードの送信に失敗しました") {
		t.Error("フォーム全体のエラーメッセージが描画されていない")
	}
	if cookie := findCookie(rec, session.EmailConfirmationCookieName); cookie != nil && cookie.Value != "" {
		t.Error("enqueue失敗時は受け渡しCookieを設定すべきでない")
	}
}

// TestCreate_TurnstileFailureは、Turnstile検証が通過しないとき (非通過または
// siteverifyエラー) にCreateがBotゲートでリクエストを止めることを検証します。
// フォームを422とフォーム全体のTurnstileメッセージで再描画し、emailをエコーバックし、
// 送信されたトークンを検証器へ渡し、確認メールを投入せず、受け渡しCookieを設定しない
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
			req = req.WithContext(i18n.SetLocale(req.Context(), model.LocaleJa))

			rec := httptest.NewRecorder()
			handler.Create(rec, req)

			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusUnprocessableEntity)
			}
			body := rec.Body.String()
			if !strings.Contains(body, "ロボットでないことの確認に失敗しました") {
				t.Error("Turnstile失敗のフォーム全体メッセージが描画されていない")
			}
			if !strings.Contains(body, `role="alert"`) {
				t.Error("フォーム全体のエラーにrole='alert' が無い")
			}
			// ユーザーが再入力しなくて済むようemailはエコーバックされること。
			if !strings.Contains(body, `value="new@example.com"`) {
				t.Error("入力したemailがエコーバックされていない")
			}
			// 送信されたトークンが検証器へ到達しており、ハンドラーが正しい
			// cf-turnstile-responseフィールドを読んでいることを確認する。
			if verifier.Token != "submitted-token" {
				t.Errorf("verifierに渡ったトークン = %q、期待値 = %q", verifier.Token, "submitted-token")
			}
			// BotゲートはUseCaseの前でリクエストを止めるため、確認メールは
			// 投入されないこと。
			if inserter.Called {
				t.Error("Turnstile失敗時に確認メールが投入された (UseCaseに進んでしまっている)")
			}
			if cookie := findCookie(rec, session.EmailConfirmationCookieName); cookie != nil && cookie.Value != "" {
				t.Error("Turnstile失敗時は受け渡しCookieを設定すべきでない")
			}
		})
	}
}
