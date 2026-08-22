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

// setupTwoFactorAuthHandler wires a settings_two_factor_auth Handler over the
// test database and creates a user to set up 2FA. It returns the handler, the 2FA
// repository (for assertions and seeding via the builder), and a user model to
// place in the request context (as RequireAuth would).
//
// [Ja] setupTwoFactorAuthHandler はテスト用データベース上に settings_two_factor_auth
// Handler を組み立て、2FA を設定するユーザーを作成する。ハンドラー・(検証と登録行の投入用の)
// 2FA リポジトリ・(RequireAuth のように) リクエスト context に載せるユーザーモデルを返す。
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

// getNew builds a GET /settings/two_factor_auth/new request with the user in the
// context (as RequireAuth would place it) and the locale set.
//
// [Ja] getNew は (RequireAuth が置くように) context にユーザーを載せ、ロケールを設定した
// GET /settings/two_factor_auth/new リクエストを組み立てる。
func getNew(user *model.User, locale string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/settings/two_factor_auth/new", nil)
	ctx := i18n.SetLocale(req.Context(), locale)
	ctx = middleware.SetUserToContext(ctx, user)
	return req.WithContext(ctx)
}

// findCookie returns the cookie with the given name from the response, or nil.
//
// [Ja] findCookie はレスポンスから指定名の Cookie を返す。無ければ nil。
func findCookie(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// decodeFlash reads and decodes the flash cookie from the response, mirroring the
// base64-encoded JSON that FlashManager writes. It fails the test if the cookie is
// missing or malformed.
//
// [Ja] decodeFlash はレスポンスのフラッシュ Cookie を読み取ってデコードする。
// FlashManager が書き込む base64 エンコードされた JSON と対になる。Cookie が無い、または
// 壊れている場合はテストを失敗させる。
func decodeFlash(t *testing.T, rec *httptest.ResponseRecorder) *session.FlashMessage {
	t.Helper()

	c := findCookie(rec, session.FlashCookieName)
	if c == nil {
		t.Fatal("フラッシュ Cookie が設定されていない")
	}
	data, err := base64.StdEncoding.DecodeString(c.Value)
	if err != nil {
		t.Fatalf("フラッシュ Cookie の base64 デコードに失敗: %v", err)
	}
	var flash session.FlashMessage
	if err := json.Unmarshal(data, &flash); err != nil {
		t.Fatalf("フラッシュ Cookie の JSON デコードに失敗: %v", err)
	}
	return &flash
}

// TestNew verifies that GET /settings/two_factor_auth/new returns HTTP 200, persists
// a not-yet-enabled enrollment, and renders the localized heading, the QR code as a
// PNG data URI, the enrollment secret for manual entry, the code form posting to
// /settings/two_factor_auth with the CSRF hidden field, and the noindex robots meta,
// for each supported locale.
//
// [Ja] TestNew は GET /settings/two_factor_auth/new が HTTP 200 を返し、未有効化の登録を
// 永続化し、サポートする各ロケールについて、ローカライズされた見出し・PNG data URI としての
// QR コード・手動入力用の登録 secret・CSRF hidden フィールド付きで /settings/two_factor_auth へ
// POST するコードフォーム・noindex の robots メタを描画することを検証する。
func TestNew(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		locale      string
		wantHeading string
	}{
		{name: "Japanese", locale: i18n.LangJa, wantHeading: "2 段階認証の設定"},
		{name: "English", locale: i18n.LangEn, wantHeading: "Set up two-factor authentication"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			db := testutil.SetupDB(t)
			h, repo, user := setupTwoFactorAuthHandler(t, db)

			rec := httptest.NewRecorder()
			h.New(rec, getNew(user, tt.locale))

			if rec.Code != http.StatusOK {
				t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
			}
			if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
				t.Errorf("Content-Type = %q, want prefix %q", got, "text/html")
			}
			// The page shows the plaintext TOTP secret, so it must not be cached.
			//
			// [Ja] このページは平文の TOTP secret を表示するため、キャッシュされてはならない。
			if got := rec.Header().Get("Cache-Control"); got != "no-store" {
				t.Errorf("Cache-Control = %q, want %q", got, "no-store")
			}

			// A not-yet-enabled enrollment row is created, and its secret is shown for
			// manual entry.
			//
			// [Ja] 未有効化の登録行が作成され、その secret が手動入力用に表示される。
			stored, err := repo.FindByUserID(context.Background(), user.ID)
			if err != nil {
				t.Fatalf("FindByUserID() error = %v", err)
			}
			if stored == nil {
				t.Fatal("未有効化の登録行が作成されていない")
			}
			if stored.Enabled {
				t.Error("作成された行の Enabled = true, want false")
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
				`<meta name="robots" content="noindex"`,
				`lang="` + tt.locale + `"`,
			}
			for _, want := range wants {
				if !strings.Contains(body, want) {
					t.Errorf("response body does not contain %q", want)
				}
			}
		})
	}
}

// TestNew_AlreadyEnabled verifies that when 2FA is already enabled, GET
// /settings/two_factor_auth/new does not re-enroll but shows the disable confirmation
// form (heading and the DELETE form) instead, so the settings page is the one place
// to turn 2FA off. No enrollment QR is rendered.
//
// [Ja] TestNew_AlreadyEnabled は、2FA が既に有効なとき GET /settings/two_factor_auth/new が
// 再登録せず、代わりに無効化の確認フォーム (見出しと DELETE フォーム) を表示し、この設定
// ページが 2FA を無効化する唯一の場所になることを検証する。登録用 QR は描画されない。
func TestNew_AlreadyEnabled(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	h, _, user := setupTwoFactorAuthHandler(t, db)
	testutil.NewUserTwoFactorAuthBuilder(t, db).WithUserID(user.ID).WithEnabled(true).Build()

	rec := httptest.NewRecorder()
	h.New(rec, getNew(user, i18n.LangJa))

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	wants := []string{
		"2 段階認証の無効化",
		`action="/settings/two_factor_auth"`,
		`value="DELETE"`,
		`name="current_password"`,
		`name="code"`,
	}
	for _, want := range wants {
		if !strings.Contains(body, want) {
			t.Errorf("response body does not contain %q", want)
		}
	}
	// It must not re-enroll: no QR code is rendered.
	//
	// [Ja] 再登録してはならない: QR コードは描画されない。
	if strings.Contains(body, "data:image/png;base64,") {
		t.Error("既に有効なのに登録用 QR が描画されている")
	}
}
