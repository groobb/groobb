package session_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/query"
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

func TestManager_GetCurrentUser(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	cfg := &config.Config{Env: "test"}
	userRepo := repository.NewUserRepository(query.New(db)).WithTx(tx)
	mgr := session.NewManager(userRepo, cfg)

	userID := testutil.NewUserBuilder(t, tx).Build()
	testutil.NewUserSessionBuilder(t, tx).
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

			mgr := session.NewManager(nil, &config.Config{Env: tt.env})
			rec := httptest.NewRecorder()

			id := model.EmailConfirmationID(uuid.New())
			mgr.SetEmailConfirmationID(rec, id)

			cookie := findCookie(rec, session.EmailConfirmationCookieName)
			if cookie == nil {
				t.Fatalf("メール確認 Cookie %q が設定されていない", session.EmailConfirmationCookieName)
			}
			if cookie.Value != id.String() {
				t.Errorf("cookie.Value = %q, want %q", cookie.Value, id.String())
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

	mgr := session.NewManager(nil, &config.Config{Env: "test"})

	t.Run("有効な id Cookie から確認 id を取り出せる", func(t *testing.T) {
		t.Parallel()

		id := model.EmailConfirmationID(uuid.New())
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: session.EmailConfirmationCookieName, Value: id.String()})

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

	t.Run("UUID として不正な値の場合は ok=false を返す", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: session.EmailConfirmationCookieName, Value: "not-a-uuid"})

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
