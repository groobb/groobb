package sign_in_two_factor_recovery_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/handler/sign_in_two_factor_recovery"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/query"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/validator"
)

// seededHandlerRecoveryCodes are the known recovery codes a handler test enrolls,
// each in the eight-lowercase-alphanumeric format the validator accepts.
//
// [Ja] seededHandlerRecoveryCodes はハンドラーテストが登録する既知のリカバリーコードで、
// それぞれバリデーターが受理する 8 文字の小文字英数字の形式です。
var seededHandlerRecoveryCodes = []string{"abcd1234", "efgh5678"}

// newSignInTwoFactorRecoveryHandler wires a sign_in_two_factor_recovery Handler
// over the shared pool (not a rolled-back transaction) because completing the
// challenge consumes a recovery code and creates a session row through the pool in
// one transaction; an outer transaction's seed rows would be invisible to it. Its
// tests seed committed users with unique emails (the test database is reset by make
// test). This exercises the full request path (pending cookie, validator, UseCase,
// session cookie) against a real database.
//
// [Ja] newSignInTwoFactorRecoveryHandler は共有プール (ロールバックされるトランザクション
// ではなく) で sign_in_two_factor_recovery Handler を組み立てる。チャレンジの完了は
// リカバリーコードの消費とセッション行の作成をプール経由で 1 トランザクションで行うため、
// 外側トランザクションで仕込んだ行はそこから見えないからである。テストはユニークな email で
// コミット済みユーザーを仕込む (テスト DB は make test がリセットする)。これにより
// リクエスト経路全体 (pending Cookie・バリデーター・UseCase・セッション Cookie) を実 DB に
// 対して通す。
func newSignInTwoFactorRecoveryHandler(t *testing.T) *sign_in_two_factor_recovery.Handler {
	t.Helper()

	cfg := &config.Config{Env: "test"}
	db := testutil.GetTestDB()
	queries := query.New(db)
	userRepo := repository.NewUserRepository(queries)
	userTwoFactorAuthRepo := repository.NewUserTwoFactorAuthRepository(queries)
	userSessionRepo := repository.NewUserSessionRepository(queries)

	sessionMgr := session.NewManager(userRepo, cfg)
	createUC := usecase.NewCreateSignInTwoFactorRecoveryUsecase(
		db,
		validator.NewSignInTwoFactorRecoveryCreateValidator(userTwoFactorAuthRepo),
		userTwoFactorAuthRepo,
		userSessionRepo,
	)
	return sign_in_two_factor_recovery.NewHandler(cfg, sessionMgr, createUC)
}

// seedUserWithRecoveryCodes creates a committed user with an enabled 2FA setting
// holding the given recovery codes, returning the user id so a handler test can put
// it in the pending cookie and submit a matching code.
//
// [Ja] seedUserWithRecoveryCodes は指定のリカバリーコードを持つ有効な 2FA 設定付きの
// コミット済みユーザーを作成し、ハンドラーテストがその id を pending Cookie に入れ、一致する
// コードを送信できるよう user id を返す。
func seedUserWithRecoveryCodes(t *testing.T, recoveryCodes []string) model.UserID {
	t.Helper()

	ctx := context.Background()
	queries := query.New(testutil.GetTestDB())

	user, err := repository.NewUserRepository(queries).Create(ctx, repository.CreateUserInput{
		Email:    fmt.Sprintf("2fa-rc-%s@example.com", uuid.NewString()),
		Atname:   testutil.UniqueAtname(),
		Locale:   i18n.LangJa,
		TimeZone: "Asia/Tokyo",
	})
	if err != nil {
		t.Fatalf("ユーザーの作成に失敗: %v", err)
	}

	twoFactorRepo := repository.NewUserTwoFactorAuthRepository(queries)
	if _, err := twoFactorRepo.Create(ctx, repository.CreateUserTwoFactorAuthInput{
		UserID: user.ID,
		Secret: testutil.DefaultBuilderTOTPSecret,
	}); err != nil {
		t.Fatalf("2 段階認証設定の作成に失敗: %v", err)
	}
	enabled, err := twoFactorRepo.Enable(ctx, user.ID, recoveryCodes)
	if err != nil {
		t.Fatalf("2 段階認証の有効化に失敗: %v", err)
	}
	if !enabled {
		t.Fatal("2 段階認証を有効化できなかった (未有効化の行が見つからない)")
	}
	return user.ID
}

// getNew builds a GET /sign_in/two_factor/recovery/new request, attaching the
// pending cookie when pendingUserID is non-empty, with the locale set in its
// context.
//
// [Ja] getNew は GET /sign_in/two_factor/recovery/new リクエストを組み立て、pendingUserID が
// 空でなければ pending Cookie を付け、context にロケールを設定する。
func getNew(pendingUserID, locale string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/sign_in/two_factor/recovery/new", nil)
	if pendingUserID != "" {
		req.AddCookie(&http.Cookie{Name: session.TwoFactorPendingCookieName, Value: pendingUserID})
	}
	return req.WithContext(i18n.SetLocale(req.Context(), locale))
}

// TestNew verifies that GET /sign_in/two_factor/recovery/new returns HTTP 200 with
// the recovery-code entry form (code field and CSRF hidden field), the localized
// heading, and the link back to the authenticator challenge, when the pending
// cookie is present. The cookie id is not resolved here, so any value is enough to
// render the thin form.
//
// [Ja] TestNew は、pending Cookie がある場合に GET /sign_in/two_factor/recovery/new が
// HTTP 200 と、リカバリーコード入力フォーム (code フィールド・CSRF hidden フィールド)、
// ローカライズ済み見出し、認証アプリチャレンジへ戻るリンクを返すことを検証する。ここでは
// Cookie の id を解決しないため、薄いフォームの描画には任意の値で足りる。
func TestNew(t *testing.T) {
	t.Parallel()

	handler := newSignInTwoFactorRecoveryHandler(t)

	tests := []struct {
		name        string
		locale      string
		wantHeading string
	}{
		{name: "Japanese", locale: i18n.LangJa, wantHeading: "リカバリーコード"},
		{name: "English", locale: i18n.LangEn, wantHeading: "Recovery code"},
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
				`action="/sign_in/two_factor/recovery"`,
				`method="POST"`,
				`name="csrf_token"`,
				`name="code"`,
				`<label for="code"`,
				// The link back to the authenticator-code challenge.
				//
				// [Ja] 認証アプリのコードチャレンジへ戻るリンク。
				`href="/sign_in/two_factor/new"`,
				// A transient, non-public auth interstitial is kept out of search results.
				//
				// [Ja] 一時的で非公開の認証中間ページは検索結果に出さない。
				`name="robots" content="noindex"`,
			}
			for _, want := range wants {
				if !strings.Contains(body, want) {
					t.Errorf("response body does not contain %q", want)
				}
			}
		})
	}
}

// TestNew_NoCookieRedirectsToSignIn verifies that GET
// /sign_in/two_factor/recovery/new without the pending cookie redirects to
// sign-in, since there is no pending challenge to complete.
//
// [Ja] TestNew_NoCookieRedirectsToSignIn は、pending Cookie の無い
// GET /sign_in/two_factor/recovery/new がサインインへリダイレクトすることを検証する。
// 完了すべき保留中のチャレンジが無いためである。
func TestNew_NoCookieRedirectsToSignIn(t *testing.T) {
	t.Parallel()

	handler := newSignInTwoFactorRecoveryHandler(t)

	rec := httptest.NewRecorder()
	handler.New(rec, getNew("", i18n.LangJa))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/sign_in" {
		t.Errorf("Location = %q, want %q", loc, "/sign_in")
	}
}
