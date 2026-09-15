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
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/validator"
)

// newPasswordResetHandlerはテスト用データベースのリポジトリで、既定で通過する
// Turnstile検証器を伴ってpassword-reset Handlerを組み立て、ハンドラーテストが実DBに
// 対してリクエスト経路全体 (Turnstileゲート・バリデーター・UseCase) を通すようにする。
// リセットメールが投入されたかをテストが検証でき、Turnstile検証を失敗させられるよう、
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

// seedUserはユニークなemailを持つコミット済みユーザーを作成し、そのemailを返す。
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

// postPasswordResetは指定したemailをフォームデータとして運ぶ
// POST /password_resetリクエストを組み立て、contextにロケールを設定する。
func postPasswordReset(email string, locale model.Locale) *http.Request {
	form := url.Values{"email": {email}}
	req := httptest.NewRequest(http.MethodPost, "/password_reset", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req.WithContext(i18n.SetLocale(req.Context(), locale))
}

// TestCreate_KnownEmailは、登録済みのemailが送信済み確認 (200) を描画し、
// リセットメールを投入することを検証する。
func TestCreate_KnownEmail(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler, inserter, _ := newPasswordResetHandler(t, db)
	email := seedUser(t, db)

	rec := httptest.NewRecorder()
	handler.Create(rec, postPasswordReset(email, model.LocaleJa))

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), "メールをご確認ください") {
		t.Error("送信済み確認ページが描画されていない")
	}
	if !inserter.Called {
		t.Error("登録済みemailではリセットメールを投入すべき")
	}
	if _, ok := inserter.Args.(dispatcher.SendPasswordResetArgs); !ok {
		t.Errorf("投入ジョブの型 = %T、期待値 = SendPasswordResetArgs", inserter.Args)
	}
}

// TestCreate_UnknownEmailは列挙攻撃対策の経路を検証する。未登録のemailは同じ
// 送信済み確認 (200) を描画するがメールは投入しないため、レスポンスは登録済みの場合と
// 区別できない。
func TestCreate_UnknownEmail(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler, inserter, _ := newPasswordResetHandler(t, db)
	email := "nobody-h@example.com"

	rec := httptest.NewRecorder()
	handler.Create(rec, postPasswordReset(email, model.LocaleJa))

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), "メールをご確認ください") {
		t.Error("送信済み確認ページが描画されていない (未登録でも同じ応答であるべき)")
	}
	if inserter.Called {
		t.Error("未登録emailではメールを投入すべきでない")
	}
}

// TestCreate_InvalidEmailは、形式不正のemailがフォームを422で形式エラー付きに
// 再描画し、メールを投入しないことを検証する。
func TestCreate_InvalidEmail(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler, inserter, _ := newPasswordResetHandler(t, db)

	rec := httptest.NewRecorder()
	handler.Create(rec, postPasswordReset("not-an-email", model.LocaleJa))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusUnprocessableEntity)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `action="/password_reset"`) {
		t.Error("申請フォームが再描画されていない")
	}
	if !strings.Contains(body, "正しいメールアドレスを入力してください") {
		t.Error("email形式エラーのメッセージが描画されていない")
	}
	if !strings.Contains(body, `aria-invalid="true"`) {
		t.Error("エラー時の入力欄にaria-invalid='true' が無い")
	}
	if inserter.Called {
		t.Error("形式不正のemailではメールを投入すべきでない")
	}
}

// TestCreate_TurnstileFailureは、Turnstile検証が通過しないとき (非通過または
// siteverifyエラー) にCreateがBotゲートでリクエストを止めることを検証する。
// 申請フォームを422とフォーム全体のTurnstileメッセージで (列挙対策の送信済みページ
// ではなく) 再描画し、emailをエコーバックし、送信されたトークンを検証器へ渡し、リセット
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
			req = req.WithContext(i18n.SetLocale(req.Context(), model.LocaleJa))

			rec := httptest.NewRecorder()
			handler.Create(rec, req)

			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusUnprocessableEntity)
			}
			body := rec.Body.String()
			// 申請フォームが (列挙対策の送信済みページではなく) 再描画され、
			// フォーム全体のTurnstileメッセージを載せていること。
			if !strings.Contains(body, `action="/password_reset"`) {
				t.Error("申請フォームが再描画されていない")
			}
			if !strings.Contains(body, "ロボットでないことの確認に失敗しました") {
				t.Error("Turnstile失敗のフォーム全体メッセージが描画されていない")
			}
			if !strings.Contains(body, `role="alert"`) {
				t.Error("フォーム全体のエラーにrole='alert' が無い")
			}
			// ユーザーが再入力しなくて済むようemailはエコーバックされること。
			if !strings.Contains(body, `value="user@example.com"`) {
				t.Error("入力したemailがエコーバックされていない")
			}
			// 送信されたトークンが検証器へ到達しており、ハンドラーが正しい
			// cf-turnstile-responseフィールドを読んでいることを確認する。
			if verifier.Token != "submitted-token" {
				t.Errorf("verifierに渡ったトークン = %q、期待値 = %q", verifier.Token, "submitted-token")
			}
			// リセットメールが投入されないこと。これは補助的なチェックである。この
			// 未知のemailではUseCaseが走ってもメールを投入しないため、この検証単独では
			// ゲートの迂回を検出できない。迂回は上の422 + フォーム再描画のアサーションが
			// 捕捉する (迂回されればUseCaseに到達し200の送信済みページを返す)。ゲートが
			// UseCaseの副作用より前で止まることの担保として残す。
			if inserter.Called {
				t.Error("Turnstile失敗時にリセットメールが投入された (UseCaseに進んでしまっている)")
			}
		})
	}
}
