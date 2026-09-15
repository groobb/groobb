package sign_in_two_factor_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/handler/sign_in_two_factor"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/validator"
)

// newSignInTwoFactorHandlerはテスト用データベースのリポジトリで
// sign_in_two_factor Handlerを組み立てる。ハンドラーテストがリクエスト経路全体
// (pending Cookie・バリデーター・UseCase・セッションCookie) を実DBに対して通せるように
// するためである。
func newSignInTwoFactorHandler(t *testing.T, db *database.DB) *sign_in_two_factor.Handler {
	t.Helper()

	cfg := testutil.NewTestConfig(t)
	userRepo := repository.NewUserRepository(db)
	userTwoFactorAuthRepo := repository.NewUserTwoFactorAuthRepository(db)
	userSessionRepo := repository.NewUserSessionRepository(db)

	sessionMgr := session.NewManager(userRepo, cfg)
	createSignInTwoFactorUC := usecase.NewCreateSignInTwoFactorUsecase(validator.NewSignInTwoFactorCreateValidator(userTwoFactorAuthRepo))
	createSessionUC := usecase.NewCreateSessionUsecase(userSessionRepo)
	return sign_in_two_factor.NewHandler(cfg, sessionMgr, createSignInTwoFactorUC, createSessionUC)
}

// twoFactorPendingTokenはパスワードサインインのステップと同じ署名付き受け渡しtokenを
// 発行し、ハンドラーテストがcontinuation tokenの境界を迂回しないようにする。
func twoFactorPendingToken(t *testing.T, id model.UserID) string {
	t.Helper()

	mgr := session.NewManager(nil, testutil.NewTestConfig(t))
	rec := httptest.NewRecorder()
	mgr.SetTwoFactorPendingUserID(rec, id)
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == session.TwoFactorPendingCookieName && cookie.Value != "" {
			return cookie.Value
		}
	}
	t.Fatalf("2段階認証pending Cookie %q の署名tokenが発行されていない", session.TwoFactorPendingCookieName)
	return ""
}

// seedUserWithEnabledTwoFactorは既定のTOTP secretに紐づく有効な2FA設定を持つ
// ユーザーをコミットして作成し、ハンドラーテストがそのidをpending Cookieに入れ、一致する
// コードを生成できるようuser idを返す。
func seedUserWithEnabledTwoFactor(t *testing.T, db *database.DB) model.UserID {
	t.Helper()

	ctx := context.Background()

	user, err := repository.NewUserRepository(db).Create(ctx, repository.CreateUserInput{
		Email:    testutil.UniqueEmail(db, "2fa-ch"),
		Atname:   testutil.UniqueAtname(db),
		Locale:   model.LocaleJa,
		TimeZone: "Asia/Tokyo",
	})
	if err != nil {
		t.Fatalf("ユーザーの作成に失敗: %v", err)
	}

	twoFactorRepo := repository.NewUserTwoFactorAuthRepository(db)
	if _, err := twoFactorRepo.Create(ctx, repository.CreateUserTwoFactorAuthInput{
		UserID: user.ID,
		Secret: testutil.DefaultBuilderTOTPSecret,
	}); err != nil {
		t.Fatalf("2段階認証設定の作成に失敗: %v", err)
	}
	enabled, err := twoFactorRepo.Enable(ctx, user.ID, []string{"recoverycode1"})
	if err != nil {
		t.Fatalf("2段階認証の有効化に失敗: %v", err)
	}
	if !enabled {
		t.Fatal("2段階認証を有効化できなかった (未有効化の行が見つからない)")
	}
	return user.ID
}

// getNewはpendingUserIDをpending Cookieに載せたGET /sign_in/two_factor/new
// リクエストを組み立て、contextにロケールを設定する。
func getNew(pendingUserID string, locale model.Locale) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/sign_in/two_factor/new", nil)
	req.AddCookie(&http.Cookie{Name: session.TwoFactorPendingCookieName, Value: pendingUserID})
	return req.WithContext(i18n.SetLocale(req.Context(), locale))
}

// TestNewは、pending Cookieがある場合にGET /sign_in/two_factor/newがHTTP 200と、
// コード入力フォーム (codeフィールド・CSRF hiddenフィールド) を、サポートする各ロケールの
// ローカライズ済み見出しとともに返すことを検証する。Cookieは有効な署名付き
// continuation tokenを運ぶ。
func TestNew(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler := newSignInTwoFactorHandler(t, db)

	tests := []struct {
		name        string
		locale      model.Locale
		wantHeading string
	}{
		{name: "日本語", locale: model.LocaleJa, wantHeading: "2段階認証"},
		{name: "英語", locale: model.LocaleEn, wantHeading: "Two-factor authentication"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			handler.New(rec, getNew(twoFactorPendingToken(t, model.UserID(testutil.UnusedID)), tt.locale))

			if rec.Code != http.StatusOK {
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
			}
			if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
				t.Errorf("Content-Type = %q、期待値の接頭辞 = %q", got, "text/html")
			}

			body := rec.Body.String()
			wants := []string{
				tt.wantHeading,
				`action="/sign_in/two_factor"`,
				`method="POST"`,
				`name="csrf_token"`,
				`name="code"`,
				`<label for="code"`,
				// 一時的で非公開の認証中間ページは検索結果に出さない。
				`name="robots" content="noindex"`,
			}
			for _, want := range wants {
				if !strings.Contains(body, want) {
					t.Errorf("レスポンスボディに %q が含まれていない", want)
				}
			}
		})
	}
}

// TestNew_NoCookieRedirectsToSignInは、pending Cookieの無い
// GET /sign_in/two_factor/newがサインインへリダイレクトすること (完了すべき保留中の
// チャレンジが無いため)、そしてリクエストが遷移先を運んでいたときはリダイレクトがそれを
// 保つことを検証する。
func TestNew_NoCookieRedirectsToSignIn(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler := newSignInTwoFactorHandler(t, db)

	tests := []struct {
		name         string
		target       string
		wantLocation string
	}{
		{
			name:         "遷移先なし",
			target:       "/sign_in/two_factor/new",
			wantLocation: "/sign_in",
		},
		// チャレンジが失われても訪問者の目的の画面は変わらないため、やり直しでも遷移先を
		// 運び、ホームに着地させない。
		{
			name:         "遷移先あり",
			target:       "/sign_in/two_factor/new?return_to=%2Fsettings",
			wantLocation: "/sign_in?return_to=%2Fsettings",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, tt.target, nil)
			req = req.WithContext(i18n.SetLocale(req.Context(), model.LocaleJa))
			rec := httptest.NewRecorder()

			handler.New(rec, req)

			if rec.Code != http.StatusSeeOther {
				t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusSeeOther)
			}
			if loc := rec.Header().Get("Location"); loc != tt.wantLocation {
				t.Errorf("Location = %q、期待値 = %q", loc, tt.wantLocation)
			}
		})
	}
}

// TestNew_ReturnToは、TOTPチャレンジページがパスワードのステップから引き渡された
// 遷移先を保つことを検証する。遷移先はフォームのhiddenフィールドと、リカバリーコード
// チャレンジへのリンクの両方に載るため、リカバリーコードへ切り替えても訪問者の向かっていた
// 先を失わない。別オリジンを指す遷移先は両方から落ちるため、どちらもセッションを発行する
// ステップへオープンリダイレクトを引き渡せない。
func TestNew_ReturnTo(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler := newSignInTwoFactorHandler(t, db)

	tests := []struct {
		name       string
		returnTo   string
		wantHidden bool
		wantLink   string
	}{
		{
			name:       "同一オリジンの相対パスは引き継ぐ",
			returnTo:   "/settings",
			wantHidden: true,
			wantLink:   `href="/sign_in/two_factor/recovery/new?return_to=%2Fsettings"`,
		},
		{
			name:       "別オリジンを指す値は引き継がない",
			returnTo:   "https://evil.example.com/settings",
			wantHidden: false,
			wantLink:   `href="/sign_in/two_factor/recovery/new"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			target := "/sign_in/two_factor/new?return_to=" + url.QueryEscape(tt.returnTo)
			req := httptest.NewRequest(http.MethodGet, target, nil)
			req.AddCookie(&http.Cookie{
				Name:  session.TwoFactorPendingCookieName,
				Value: twoFactorPendingToken(t, model.UserID(testutil.UnusedID)),
			})
			req = req.WithContext(i18n.SetLocale(req.Context(), model.LocaleJa))
			rec := httptest.NewRecorder()

			handler.New(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
			}
			body := rec.Body.String()
			if got := strings.Contains(body, `name="return_to"`); got != tt.wantHidden {
				t.Errorf("return_toのhiddenフィールドの有無 = %v、期待値 = %v", got, tt.wantHidden)
			}
			if tt.wantHidden && !strings.Contains(body, `value="/settings"`) {
				t.Error(`ボディにvalue="/settings" が含まれていない`)
			}
			if !strings.Contains(body, tt.wantLink) {
				t.Errorf("ボディに %q が含まれていない", tt.wantLink)
			}
		})
	}
}
