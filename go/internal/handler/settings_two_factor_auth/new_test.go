package settings_two_factor_auth_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/handler/settings_two_factor_auth"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/validator"
)

// setupTwoFactorAuthHandlerはテスト用データベース上にsettings_two_factor_auth
// Handlerを組み立て、2FAを設定するユーザーを作成する。ハンドラー・(検証と登録行の投入用の)
// 2FAリポジトリ・(RequireAuthのように) リクエストcontextに載せるユーザーモデルを返す。
func setupTwoFactorAuthHandler(t *testing.T, db *database.DB) (*settings_two_factor_auth.Handler, *repository.UserTwoFactorAuthRepository, *model.User) {
	t.Helper()

	cfg := &config.Config{Env: "test"}
	email := "2fa-h@example.com"
	userID := testutil.NewUserBuilder(t, db).WithEmail(email).Build()

	repo := repository.NewUserTwoFactorAuthRepository(db)
	userPasswordRepo := repository.NewUserPasswordRepository(db)
	prepareUC := usecase.NewPrepareTwoFactorAuthUsecase(repo)
	enableUC := usecase.NewEnableTwoFactorAuthUsecase(
		validator.NewSettingsTwoFactorAuthCreateValidator(repo),
		repo,
	)
	disableUC := usecase.NewDisableTwoFactorAuthUsecase(
		validator.NewSettingsTwoFactorAuthDeleteValidator(userPasswordRepo, repo),
		repo,
	)
	h := settings_two_factor_auth.NewHandler(cfg, session.NewFlashManager(cfg), prepareUC, enableUC, disableUC)

	return h, repo, &model.User{ID: userID, Email: email}
}

// getNewは (RequireAuthが置くように) contextにユーザーを載せ、ロケールを設定した
// GET /settings/two_factor_auth/newリクエストを組み立てる。
func getNew(user *model.User, locale model.Locale) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/settings/two_factor_auth/new", nil)
	ctx := i18n.SetLocale(req.Context(), locale)
	ctx = middleware.SetUserToContext(ctx, user)
	return req.WithContext(ctx)
}

// findCookieはレスポンスから指定名のCookieを返す。無ければnil。
func findCookie(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// decodeFlashはレスポンスのフラッシュCookieを読み取ってデコードする。
// FlashManagerが書き込むbase64エンコードされたJSONと対になる。Cookieが無い、または
// 壊れている場合はテストを失敗させる。
func decodeFlash(t *testing.T, rec *httptest.ResponseRecorder) *session.FlashMessage {
	t.Helper()

	c := findCookie(rec, session.FlashCookieName)
	if c == nil {
		t.Fatal("フラッシュCookieが設定されていない")
	}
	data, err := base64.StdEncoding.DecodeString(c.Value)
	if err != nil {
		t.Fatalf("フラッシュCookieのbase64デコードに失敗: %v", err)
	}
	var flash session.FlashMessage
	if err := json.Unmarshal(data, &flash); err != nil {
		t.Fatalf("フラッシュCookieのJSONデコードに失敗: %v", err)
	}
	return &flash
}

// TestNewはGET /settings/two_factor_auth/newがHTTP 200を返し、未有効化の登録を
// 永続化し、サポートする各ロケールについて、ローカライズされた見出し・PNG data URIとしての
// QRコード・手動入力用の登録secret・CSRF hiddenフィールド付きで /settings/two_factor_authへ
// POSTするコードフォーム・noindexのrobotsメタを描画することを検証する。
func TestNew(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		locale        model.Locale
		wantHeading   string
		wantHeaderNav string
	}{
		{name: "日本語", locale: model.LocaleJa, wantHeading: "2段階認証の設定", wantHeaderNav: "グローバルナビゲーション"},
		{name: "英語", locale: model.LocaleEn, wantHeading: "Set up two-factor authentication", wantHeaderNav: "Global navigation"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			db := testutil.SetupDB(t)
			h, repo, user := setupTwoFactorAuthHandler(t, db)

			rec := httptest.NewRecorder()
			h.New(rec, getNew(user, tt.locale))

			if rec.Code != http.StatusOK {
				t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
			}
			if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
				t.Errorf("Content-Type = %q、期待値の接頭辞 = %q", got, "text/html")
			}
			// このページは平文のTOTP secretを表示するため、キャッシュされてはならない。
			if got := rec.Header().Get("Cache-Control"); got != "no-store" {
				t.Errorf("Cache-Control = %q、期待値 = %q", got, "no-store")
			}

			// 未有効化の登録行が作成され、そのsecretが手動入力用に表示される。
			stored, err := repo.FindByUserID(context.Background(), user.ID)
			if err != nil {
				t.Fatalf("FindByUserID()のエラー = %v", err)
			}
			if stored == nil {
				t.Fatal("未有効化の登録行が作成されていない")
			}
			if stored.Enabled {
				t.Error("作成された行のEnabled = true、期待値 = false")
			}

			body := rec.Body.String()
			wants := []string{
				tt.wantHeading,
				"data:image/png;base64,",
				stored.Secret,
				`action="/settings/two_factor_auth"`,
				`method="POST"`,
				`name="csrf_token"`,
				`name="code"`,
				`aria-label="` + tt.wantHeaderNav + `"`,
				`href="/home"`,
				`<meta name="robots" content="noindex"`,
				`lang="` + string(tt.locale) + `"`,
			}
			for _, want := range wants {
				if !strings.Contains(body, want) {
					t.Errorf("レスポンスボディに %q が含まれていない", want)
				}
			}
		})
	}
}

// TestNew_AlreadyEnabledは、2FAが既に有効なときGET /settings/two_factor_auth/newが
// 再登録せず、代わりに無効化の確認フォーム (見出しとDELETEフォーム) を表示し、この設定
// ページが2FAを無効化する唯一の場所になることを検証する。登録用QRは描画されない。
func TestNew_AlreadyEnabled(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	h, _, user := setupTwoFactorAuthHandler(t, db)
	testutil.NewUserTwoFactorAuthBuilder(t, db).WithUserID(user.ID).WithEnabled(true).Build()

	rec := httptest.NewRecorder()
	h.New(rec, getNew(user, model.LocaleJa))

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	wants := []string{
		"2段階認証の無効化",
		`action="/settings/two_factor_auth"`,
		`value="DELETE"`,
		`name="current_password"`,
		`name="code"`,
		`aria-label="グローバルナビゲーション"`,
		`href="/home"`,
	}
	for _, want := range wants {
		if !strings.Contains(body, want) {
			t.Errorf("レスポンスボディに %q が含まれていない", want)
		}
	}
	// 再登録してはならない: QRコードは描画されない。
	if strings.Contains(body, "data:image/png;base64,") {
		t.Error("既に有効なのに登録用QRが描画されている")
	}
}
