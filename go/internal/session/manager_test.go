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

// findCookie returns the cookie with the given name from a recorded response, or
// nil when it is absent.
//
// [Ja] findCookie は記録されたレスポンスから指定名の Cookie を返す。無ければ nil。
func findCookie(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// continuationManagerConfig returns a test Config with the shared signing key
// and the requested environment for Cookie attribute assertions.
//
// [Ja] continuationManagerConfig は共有の署名鍵と、Cookie 属性の検証で指定された実行環境を
// 持つテスト用 Config を返します。
func continuationManagerConfig(t *testing.T, env string) *config.Config {
	t.Helper()

	cfg := testutil.NewTestConfig(t)
	cfg.Env = env
	return cfg
}

// emailConfirmationCookie asks the Manager to issue a real signed continuation
// Cookie for id.
//
// [Ja] emailConfirmationCookie は Manager に id 用の実際の署名付き continuation Cookie を
// 発行させます。
func emailConfirmationCookie(t *testing.T, mgr *session.Manager, id model.EmailConfirmationID) *http.Cookie {
	t.Helper()

	rec := httptest.NewRecorder()
	mgr.SetEmailConfirmationID(rec, id)
	cookie := findCookie(rec, session.EmailConfirmationCookieName)
	if cookie == nil {
		t.Fatalf("メール確認 Cookie %q が設定されていない", session.EmailConfirmationCookieName)
	}
	return cookie
}

// twoFactorPendingCookie asks the Manager to issue a real signed continuation
// Cookie for id.
//
// [Ja] twoFactorPendingCookie は Manager に id 用の実際の署名付き continuation Cookie を
// 発行させます。
func twoFactorPendingCookie(t *testing.T, mgr *session.Manager, id model.UserID) *http.Cookie {
	t.Helper()

	rec := httptest.NewRecorder()
	mgr.SetTwoFactorPendingUserID(rec, id)
	cookie := findCookie(rec, session.TwoFactorPendingCookieName)
	if cookie == nil {
		t.Fatalf("2 段階認証 pending Cookie %q が設定されていない", session.TwoFactorPendingCookieName)
	}
	return cookie
}

// tamperToken changes a significant byte of the encoded signature.
//
// [Ja] tamperToken はエンコード済み署名の有効な 1 バイトを変更します。
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

	t.Run("有効なセッション Cookie から現在のユーザーを解決できる", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: session.CookieName, Value: "valid-token"})

		user, err := mgr.GetCurrentUser(req.Context(), req)
		if err != nil {
			t.Fatalf("GetCurrentUser() error = %v", err)
		}
		if user == nil {
			t.Fatal("GetCurrentUser() = nil, want user")
		}
		if user.ID != userID {
			t.Errorf("user.ID = %v, want %v", user.ID, userID)
		}
	})

	t.Run("Cookie が無い場合は (nil, nil) を返す", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)

		user, err := mgr.GetCurrentUser(req.Context(), req)
		if err != nil {
			t.Fatalf("GetCurrentUser() error = %v", err)
		}
		if user != nil {
			t.Errorf("GetCurrentUser() = %v, want nil", user)
		}
	})

	t.Run("未知のトークンの場合は (nil, nil) を返す", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: session.CookieName, Value: "unknown-token"})

		user, err := mgr.GetCurrentUser(req.Context(), req)
		if err != nil {
			t.Fatalf("GetCurrentUser() error = %v", err)
		}
		if user != nil {
			t.Errorf("GetCurrentUser() = %v, want nil", user)
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
		{name: "本番では Secure を立てる", env: "prod", wantSecure: true},
		{name: "開発では Secure を立てない (平文 HTTP のため)", env: "dev", wantSecure: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mgr := session.NewManager(nil, &config.Config{Env: tt.env})
			rec := httptest.NewRecorder()

			mgr.SetSessionCookie(rec, "the-token")

			cookie := findCookie(rec, session.CookieName)
			if cookie == nil {
				t.Fatalf("セッション Cookie %q が設定されていない", session.CookieName)
			}
			if cookie.Value != "the-token" {
				t.Errorf("cookie.Value = %q, want %q", cookie.Value, "the-token")
			}
			if !cookie.HttpOnly {
				t.Error("セッション Cookie は HttpOnly であるべき")
			}
			if cookie.SameSite != http.SameSiteLaxMode {
				t.Errorf("cookie.SameSite = %v, want %v", cookie.SameSite, http.SameSiteLaxMode)
			}
			if cookie.MaxAge <= 0 {
				t.Errorf("cookie.MaxAge = %d, want 正の値", cookie.MaxAge)
			}
			if cookie.Secure != tt.wantSecure {
				t.Errorf("cookie.Secure = %v, want %v", cookie.Secure, tt.wantSecure)
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
		t.Fatalf("セッション Cookie %q が設定されていない", session.CookieName)
	}
	if cookie.Value != "" {
		t.Errorf("cookie.Value = %q, want 空文字列", cookie.Value)
	}
	if cookie.MaxAge >= 0 {
		t.Errorf("cookie.MaxAge = %d, want 負の値 (削除指示)", cookie.MaxAge)
	}
}

func TestManager_SetEmailConfirmationID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		env        string
		wantSecure bool
	}{
		{name: "本番では Secure を立てる", env: "prod", wantSecure: true},
		{name: "開発では Secure を立てない (平文 HTTP のため)", env: "dev", wantSecure: false},
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
				t.Fatalf("メール確認 Cookie %q が設定されていない", session.EmailConfirmationCookieName)
			}
			if cookie.Value == "" || cookie.Value == id.String() {
				t.Errorf("cookie.Value = %q, want non-empty signed token instead of raw id", cookie.Value)
			}
			if !cookie.HttpOnly {
				t.Error("メール確認 Cookie は HttpOnly であるべき")
			}
			if cookie.SameSite != http.SameSiteLaxMode {
				t.Errorf("cookie.SameSite = %v, want %v", cookie.SameSite, http.SameSiteLaxMode)
			}
			if cookie.MaxAge <= 0 {
				t.Errorf("cookie.MaxAge = %d, want 正の値", cookie.MaxAge)
			}
			if cookie.Secure != tt.wantSecure {
				t.Errorf("cookie.Secure = %v, want %v", cookie.Secure, tt.wantSecure)
			}
		})
	}
}

func TestManager_GetEmailConfirmationID(t *testing.T) {
	t.Parallel()

	mgr := session.NewManager(nil, testutil.NewTestConfig(t))

	t.Run("有効な署名付き token から確認 id を取り出せる", func(t *testing.T) {
		t.Parallel()

		id := model.EmailConfirmationID(testutil.UnusedID)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(emailConfirmationCookie(t, mgr, id))

		got, ok := mgr.GetEmailConfirmationID(req)
		if !ok {
			t.Fatal("GetEmailConfirmationID() ok = false, want true")
		}
		if got != id {
			t.Errorf("GetEmailConfirmationID() = %v, want %v", got, id)
		}
	})

	t.Run("Cookie が無い場合は ok=false を返す", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/", nil)

		if _, ok := mgr.GetEmailConfirmationID(req); ok {
			t.Error("GetEmailConfirmationID() ok = true, want false")
		}
	})

	t.Run("未署名の整数 ID は ok=false を返す", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: session.EmailConfirmationCookieName, Value: model.EmailConfirmationID(testutil.UnusedID).String()})

		if _, ok := mgr.GetEmailConfirmationID(req); ok {
			t.Error("GetEmailConfirmationID() ok = true, want false")
		}
	})

	t.Run("署名を改ざんした token は ok=false を返す", func(t *testing.T) {
		t.Parallel()

		cookie := emailConfirmationCookie(t, mgr, model.EmailConfirmationID(testutil.UnusedID))
		cookie.Value = tamperToken(cookie.Value)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(cookie)

		if _, ok := mgr.GetEmailConfirmationID(req); ok {
			t.Error("GetEmailConfirmationID() ok = true, want false")
		}
	})

	t.Run("2 段階認証用 token は ok=false を返す", func(t *testing.T) {
		t.Parallel()

		cookie := twoFactorPendingCookie(t, mgr, model.UserID(testutil.UnusedID))
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{
			Name:  session.EmailConfirmationCookieName,
			Value: cookie.Value,
		})

		if _, ok := mgr.GetEmailConfirmationID(req); ok {
			t.Error("GetEmailConfirmationID() ok = true, want false")
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
		t.Fatalf("メール確認 Cookie %q が設定されていない", session.EmailConfirmationCookieName)
	}
	if cookie.Value != "" {
		t.Errorf("cookie.Value = %q, want 空文字列", cookie.Value)
	}
	if cookie.MaxAge >= 0 {
		t.Errorf("cookie.MaxAge = %d, want 負の値 (削除指示)", cookie.MaxAge)
	}
}

func TestManager_SetTwoFactorPendingUserID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		env        string
		wantSecure bool
	}{
		{name: "本番では Secure を立てる", env: "prod", wantSecure: true},
		{name: "開発では Secure を立てない (平文 HTTP のため)", env: "dev", wantSecure: false},
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
				t.Fatalf("2 段階認証 pending Cookie %q が設定されていない", session.TwoFactorPendingCookieName)
			}
			if cookie.Value == "" || cookie.Value == id.String() {
				t.Errorf("cookie.Value = %q, want non-empty signed token instead of raw id", cookie.Value)
			}
			if !cookie.HttpOnly {
				t.Error("2 段階認証 pending Cookie は HttpOnly であるべき")
			}
			if cookie.SameSite != http.SameSiteLaxMode {
				t.Errorf("cookie.SameSite = %v, want %v", cookie.SameSite, http.SameSiteLaxMode)
			}
			if cookie.MaxAge <= 0 {
				t.Errorf("cookie.MaxAge = %d, want 正の値", cookie.MaxAge)
			}
			if cookie.Secure != tt.wantSecure {
				t.Errorf("cookie.Secure = %v, want %v", cookie.Secure, tt.wantSecure)
			}
		})
	}
}

func TestManager_GetTwoFactorPendingUserID(t *testing.T) {
	t.Parallel()

	mgr := session.NewManager(nil, testutil.NewTestConfig(t))

	t.Run("有効な署名付き token から保留中のユーザー id を取り出せる", func(t *testing.T) {
		t.Parallel()

		id := model.UserID(testutil.UnusedID)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(twoFactorPendingCookie(t, mgr, id))

		got, ok := mgr.GetTwoFactorPendingUserID(req)
		if !ok {
			t.Fatal("GetTwoFactorPendingUserID() ok = false, want true")
		}
		if got != id {
			t.Errorf("GetTwoFactorPendingUserID() = %v, want %v", got, id)
		}
	})

	t.Run("Cookie が無い場合は ok=false を返す", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/", nil)

		if _, ok := mgr.GetTwoFactorPendingUserID(req); ok {
			t.Error("GetTwoFactorPendingUserID() ok = true, want false")
		}
	})

	t.Run("未署名の整数 ID は ok=false を返す", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: session.TwoFactorPendingCookieName, Value: model.UserID(testutil.UnusedID).String()})

		if _, ok := mgr.GetTwoFactorPendingUserID(req); ok {
			t.Error("GetTwoFactorPendingUserID() ok = true, want false")
		}
	})

	t.Run("署名を改ざんした token は ok=false を返す", func(t *testing.T) {
		t.Parallel()

		cookie := twoFactorPendingCookie(t, mgr, model.UserID(testutil.UnusedID))
		cookie.Value = tamperToken(cookie.Value)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(cookie)

		if _, ok := mgr.GetTwoFactorPendingUserID(req); ok {
			t.Error("GetTwoFactorPendingUserID() ok = true, want false")
		}
	})

	t.Run("メール確認用 token は ok=false を返す", func(t *testing.T) {
		t.Parallel()

		cookie := emailConfirmationCookie(t, mgr, model.EmailConfirmationID(testutil.UnusedID))
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{
			Name:  session.TwoFactorPendingCookieName,
			Value: cookie.Value,
		})

		if _, ok := mgr.GetTwoFactorPendingUserID(req); ok {
			t.Error("GetTwoFactorPendingUserID() ok = true, want false")
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
		t.Fatalf("2 段階認証 pending Cookie %q が設定されていない", session.TwoFactorPendingCookieName)
	}
	if cookie.Value != "" {
		t.Errorf("cookie.Value = %q, want 空文字列", cookie.Value)
	}
	if cookie.MaxAge >= 0 {
		t.Errorf("cookie.MaxAge = %d, want 負の値 (削除指示)", cookie.MaxAge)
	}
}
