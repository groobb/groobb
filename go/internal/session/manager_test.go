package session_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/testutil"
)

// findCookieは記録されたレスポンスから指定名のCookieを返す。無ければnil。
func findCookie(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// continuationManagerConfigは共有の署名鍵と、Cookie属性の検証で指定された実行環境を
// 持つテスト用Configを返します。
func continuationManagerConfig(t *testing.T, env string) *config.Config {
	t.Helper()

	cfg := testutil.NewTestConfig(t)
	cfg.Env = env
	return cfg
}

// emailConfirmationCookieはManagerにid用の実際の署名付きcontinuation Cookieを
// 発行させます。
func emailConfirmationCookie(t *testing.T, mgr *session.Manager, id model.EmailConfirmationID) *http.Cookie {
	t.Helper()

	rec := httptest.NewRecorder()
	mgr.SetEmailConfirmationID(rec, id)
	cookie := findCookie(rec, session.EmailConfirmationCookieName)
	if cookie == nil {
		t.Fatalf("メール確認Cookie %q が設定されていない", session.EmailConfirmationCookieName)
	}
	return cookie
}

// twoFactorPendingCookieはManagerにid用の実際の署名付きcontinuation Cookieを
// 発行させます。
func twoFactorPendingCookie(t *testing.T, mgr *session.Manager, id model.UserID) *http.Cookie {
	t.Helper()

	rec := httptest.NewRecorder()
	mgr.SetTwoFactorPendingUserID(rec, id)
	cookie := findCookie(rec, session.TwoFactorPendingCookieName)
	if cookie == nil {
		t.Fatalf("2段階認証pending Cookie %q が設定されていない", session.TwoFactorPendingCookieName)
	}
	return cookie
}

// tamperTokenはエンコード済み署名の有効な1バイトを変更します。
func tamperToken(token string) string {
	signatureStart := strings.LastIndexByte(token, '.') + 1
	replacement := byte('A')
	if token[signatureStart] == replacement {
		replacement = 'B'
	}
	return token[:signatureStart] + string(replacement) + token[signatureStart+1:]
}

func TestManager_GetCurrentUser(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	cfg := &config.Config{Env: "test"}
	userRepo := repository.NewUserRepository(db)
	mgr := session.NewManager(userRepo, cfg)

	userID := testutil.NewUserBuilder(t, db).Build()
	testutil.NewUserSessionBuilder(t, db).
		WithUserID(userID).
		WithToken("valid-token").
		Build()

	t.Run("有効なセッションCookieから現在のユーザーを解決できる", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: session.CookieName, Value: "valid-token"})

		user, err := mgr.GetCurrentUser(req.Context(), req)
		if err != nil {
			t.Fatalf("GetCurrentUser()のエラー = %v", err)
		}
		if user == nil {
			t.Fatal("GetCurrentUser() = nil、期待値はユーザー")
		}
		if user.ID != userID {
			t.Errorf("user.ID = %v、期待値 = %v", user.ID, userID)
		}
	})

	t.Run("Cookieが無い場合は (nil, nil) を返す", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)

		user, err := mgr.GetCurrentUser(req.Context(), req)
		if err != nil {
			t.Fatalf("GetCurrentUser()のエラー = %v", err)
		}
		if user != nil {
			t.Errorf("GetCurrentUser() = %v、期待値 = nil", user)
		}
	})

	t.Run("未知のトークンの場合は (nil, nil) を返す", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: session.CookieName, Value: "unknown-token"})

		user, err := mgr.GetCurrentUser(req.Context(), req)
		if err != nil {
			t.Fatalf("GetCurrentUser()のエラー = %v", err)
		}
		if user != nil {
			t.Errorf("GetCurrentUser() = %v、期待値 = nil", user)
		}
	})
}

func TestManager_SetSessionCookie(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		env        string
		wantSecure bool
	}{
		{name: "本番ではSecureを立てる", env: "prod", wantSecure: true},
		{name: "開発ではSecureを立てない (平文HTTPのため)", env: "dev", wantSecure: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mgr := session.NewManager(nil, &config.Config{Env: tt.env})
			rec := httptest.NewRecorder()

			mgr.SetSessionCookie(rec, "the-token")

			cookie := findCookie(rec, session.CookieName)
			if cookie == nil {
				t.Fatalf("セッションCookie %q が設定されていない", session.CookieName)
			}
			if cookie.Value != "the-token" {
				t.Errorf("cookie.Value = %q、期待値 = %q", cookie.Value, "the-token")
			}
			if !cookie.HttpOnly {
				t.Error("セッションCookieはHttpOnlyであるべき")
			}
			if cookie.SameSite != http.SameSiteLaxMode {
				t.Errorf("cookie.SameSite = %v、期待値 = %v", cookie.SameSite, http.SameSiteLaxMode)
			}
			if cookie.MaxAge <= 0 {
				t.Errorf("cookie.MaxAge = %d、期待値は正の値", cookie.MaxAge)
			}
			if cookie.Secure != tt.wantSecure {
				t.Errorf("cookie.Secure = %v、期待値 = %v", cookie.Secure, tt.wantSecure)
			}
		})
	}
}

func TestManager_DeleteSessionCookie(t *testing.T) {
	t.Parallel()

	mgr := session.NewManager(nil, &config.Config{Env: "test"})
	rec := httptest.NewRecorder()

	mgr.DeleteSessionCookie(rec)

	cookie := findCookie(rec, session.CookieName)
	if cookie == nil {
		t.Fatalf("セッションCookie %q が設定されていない", session.CookieName)
	}
	if cookie.Value != "" {
		t.Errorf("cookie.Value = %q、期待値は空文字列", cookie.Value)
	}
	if cookie.MaxAge >= 0 {
		t.Errorf("cookie.MaxAge = %d、期待値は負の値 (削除指示)", cookie.MaxAge)
	}
}

func TestManager_SetEmailConfirmationID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		env        string
		wantSecure bool
	}{
		{name: "本番ではSecureを立てる", env: "prod", wantSecure: true},
		{name: "開発ではSecureを立てない (平文HTTPのため)", env: "dev", wantSecure: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mgr := session.NewManager(nil, continuationManagerConfig(t, tt.env))
			rec := httptest.NewRecorder()

			id := model.EmailConfirmationID(testutil.UnusedID)
			mgr.SetEmailConfirmationID(rec, id)

			cookie := findCookie(rec, session.EmailConfirmationCookieName)
			if cookie == nil {
				t.Fatalf("メール確認Cookie %q が設定されていない", session.EmailConfirmationCookieName)
			}
			if cookie.Value == "" || cookie.Value == id.String() {
				t.Errorf("cookie.Value = %q、期待値は生のidではなく空でない署名付きトークン", cookie.Value)
			}
			if !cookie.HttpOnly {
				t.Error("メール確認CookieはHttpOnlyであるべき")
			}
			if cookie.SameSite != http.SameSiteLaxMode {
				t.Errorf("cookie.SameSite = %v、期待値 = %v", cookie.SameSite, http.SameSiteLaxMode)
			}
			if cookie.MaxAge <= 0 {
				t.Errorf("cookie.MaxAge = %d、期待値は正の値", cookie.MaxAge)
			}
			if cookie.Secure != tt.wantSecure {
				t.Errorf("cookie.Secure = %v、期待値 = %v", cookie.Secure, tt.wantSecure)
			}
		})
	}
}

func TestManager_GetEmailConfirmationID(t *testing.T) {
	t.Parallel()

	mgr := session.NewManager(nil, testutil.NewTestConfig(t))

	t.Run("有効な署名付きtokenから確認idを取り出せる", func(t *testing.T) {
		t.Parallel()

		id := model.EmailConfirmationID(testutil.UnusedID)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(emailConfirmationCookie(t, mgr, id))

		got, ok := mgr.GetEmailConfirmationID(req)
		if !ok {
			t.Fatal("GetEmailConfirmationID()のok = false、期待値 = true")
		}
		if got != id {
			t.Errorf("GetEmailConfirmationID() = %v、期待値 = %v", got, id)
		}
	})

	t.Run("Cookieが無い場合はok=falseを返す", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/", nil)

		if _, ok := mgr.GetEmailConfirmationID(req); ok {
			t.Error("GetEmailConfirmationID()のok = true、期待値 = false")
		}
	})

	t.Run("未署名の整数IDはok=falseを返す", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: session.EmailConfirmationCookieName, Value: model.EmailConfirmationID(testutil.UnusedID).String()})

		if _, ok := mgr.GetEmailConfirmationID(req); ok {
			t.Error("GetEmailConfirmationID()のok = true、期待値 = false")
		}
	})

	t.Run("署名を改ざんしたtokenはok=falseを返す", func(t *testing.T) {
		t.Parallel()

		cookie := emailConfirmationCookie(t, mgr, model.EmailConfirmationID(testutil.UnusedID))
		cookie.Value = tamperToken(cookie.Value)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(cookie)

		if _, ok := mgr.GetEmailConfirmationID(req); ok {
			t.Error("GetEmailConfirmationID()のok = true、期待値 = false")
		}
	})

	t.Run("2段階認証用tokenはok=falseを返す", func(t *testing.T) {
		t.Parallel()

		cookie := twoFactorPendingCookie(t, mgr, model.UserID(testutil.UnusedID))
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{
			Name:  session.EmailConfirmationCookieName,
			Value: cookie.Value,
		})

		if _, ok := mgr.GetEmailConfirmationID(req); ok {
			t.Error("GetEmailConfirmationID()のok = true、期待値 = false")
		}
	})
}

func TestManager_DeleteEmailConfirmationID(t *testing.T) {
	t.Parallel()

	mgr := session.NewManager(nil, &config.Config{Env: "test"})
	rec := httptest.NewRecorder()

	mgr.DeleteEmailConfirmationID(rec)

	cookie := findCookie(rec, session.EmailConfirmationCookieName)
	if cookie == nil {
		t.Fatalf("メール確認Cookie %q が設定されていない", session.EmailConfirmationCookieName)
	}
	if cookie.Value != "" {
		t.Errorf("cookie.Value = %q、期待値は空文字列", cookie.Value)
	}
	if cookie.MaxAge >= 0 {
		t.Errorf("cookie.MaxAge = %d、期待値は負の値 (削除指示)", cookie.MaxAge)
	}
}

func TestManager_SetTwoFactorPendingUserID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		env        string
		wantSecure bool
	}{
		{name: "本番ではSecureを立てる", env: "prod", wantSecure: true},
		{name: "開発ではSecureを立てない (平文HTTPのため)", env: "dev", wantSecure: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mgr := session.NewManager(nil, continuationManagerConfig(t, tt.env))
			rec := httptest.NewRecorder()

			id := model.UserID(testutil.UnusedID)
			mgr.SetTwoFactorPendingUserID(rec, id)

			cookie := findCookie(rec, session.TwoFactorPendingCookieName)
			if cookie == nil {
				t.Fatalf("2段階認証pending Cookie %q が設定されていない", session.TwoFactorPendingCookieName)
			}
			if cookie.Value == "" || cookie.Value == id.String() {
				t.Errorf("cookie.Value = %q、期待値は生のidではなく空でない署名付きトークン", cookie.Value)
			}
			if !cookie.HttpOnly {
				t.Error("2段階認証pending CookieはHttpOnlyであるべき")
			}
			if cookie.SameSite != http.SameSiteLaxMode {
				t.Errorf("cookie.SameSite = %v、期待値 = %v", cookie.SameSite, http.SameSiteLaxMode)
			}
			if cookie.MaxAge <= 0 {
				t.Errorf("cookie.MaxAge = %d、期待値は正の値", cookie.MaxAge)
			}
			if cookie.Secure != tt.wantSecure {
				t.Errorf("cookie.Secure = %v、期待値 = %v", cookie.Secure, tt.wantSecure)
			}
		})
	}
}

func TestManager_GetTwoFactorPendingUserID(t *testing.T) {
	t.Parallel()

	mgr := session.NewManager(nil, testutil.NewTestConfig(t))

	t.Run("有効な署名付きtokenから保留中のユーザーidを取り出せる", func(t *testing.T) {
		t.Parallel()

		id := model.UserID(testutil.UnusedID)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(twoFactorPendingCookie(t, mgr, id))

		got, ok := mgr.GetTwoFactorPendingUserID(req)
		if !ok {
			t.Fatal("GetTwoFactorPendingUserID()のok = false、期待値 = true")
		}
		if got != id {
			t.Errorf("GetTwoFactorPendingUserID() = %v、期待値 = %v", got, id)
		}
	})

	t.Run("Cookieが無い場合はok=falseを返す", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/", nil)

		if _, ok := mgr.GetTwoFactorPendingUserID(req); ok {
			t.Error("GetTwoFactorPendingUserID()のok = true、期待値 = false")
		}
	})

	t.Run("未署名の整数IDはok=falseを返す", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: session.TwoFactorPendingCookieName, Value: model.UserID(testutil.UnusedID).String()})

		if _, ok := mgr.GetTwoFactorPendingUserID(req); ok {
			t.Error("GetTwoFactorPendingUserID()のok = true、期待値 = false")
		}
	})

	t.Run("署名を改ざんしたtokenはok=falseを返す", func(t *testing.T) {
		t.Parallel()

		cookie := twoFactorPendingCookie(t, mgr, model.UserID(testutil.UnusedID))
		cookie.Value = tamperToken(cookie.Value)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(cookie)

		if _, ok := mgr.GetTwoFactorPendingUserID(req); ok {
			t.Error("GetTwoFactorPendingUserID()のok = true、期待値 = false")
		}
	})

	t.Run("メール確認用tokenはok=falseを返す", func(t *testing.T) {
		t.Parallel()

		cookie := emailConfirmationCookie(t, mgr, model.EmailConfirmationID(testutil.UnusedID))
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{
			Name:  session.TwoFactorPendingCookieName,
			Value: cookie.Value,
		})

		if _, ok := mgr.GetTwoFactorPendingUserID(req); ok {
			t.Error("GetTwoFactorPendingUserID()のok = true、期待値 = false")
		}
	})
}

func TestManager_DeleteTwoFactorPendingUserID(t *testing.T) {
	t.Parallel()

	mgr := session.NewManager(nil, &config.Config{Env: "test"})
	rec := httptest.NewRecorder()

	mgr.DeleteTwoFactorPendingUserID(rec)

	cookie := findCookie(rec, session.TwoFactorPendingCookieName)
	if cookie == nil {
		t.Fatalf("2段階認証pending Cookie %q が設定されていない", session.TwoFactorPendingCookieName)
	}
	if cookie.Value != "" {
		t.Errorf("cookie.Value = %q、期待値は空文字列", cookie.Value)
	}
	if cookie.MaxAge >= 0 {
		t.Errorf("cookie.MaxAge = %d、期待値は負の値 (削除指示)", cookie.MaxAge)
	}
}
