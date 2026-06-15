package session_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/session"
)

func TestFlashManager_SetAndGetFlash(t *testing.T) {
	t.Parallel()

	fm := session.NewFlashManager(&config.Config{Env: "test"})

	tests := []struct {
		name     string
		set      func(http.ResponseWriter, string)
		wantType session.FlashType
	}{
		{name: "success", set: fm.SetSuccess, wantType: session.FlashSuccess},
		{name: "error", set: fm.SetError, wantType: session.FlashError},
		{name: "warning", set: fm.SetWarning, wantType: session.FlashWarning},
		{name: "info", set: fm.SetInfo, wantType: session.FlashInfo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			setRec := httptest.NewRecorder()
			tt.set(setRec, "こんにちは")

			cookie := findCookie(setRec, session.FlashCookieName)
			if cookie == nil {
				t.Fatalf("フラッシュ Cookie %q が設定されていない", session.FlashCookieName)
			}
			if cookie.HttpOnly {
				t.Error("フラッシュ Cookie は JS から読めるよう HttpOnly でないべき")
			}

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.AddCookie(cookie)
			getRec := httptest.NewRecorder()

			flash := fm.GetFlash(getRec, req)
			if flash == nil {
				t.Fatal("GetFlash() = nil, want flash")
			}
			if flash.Type != tt.wantType {
				t.Errorf("flash.Type = %q, want %q", flash.Type, tt.wantType)
			}
			if flash.Message != "こんにちは" {
				t.Errorf("flash.Message = %q, want %q", flash.Message, "こんにちは")
			}
		})
	}
}

func TestFlashManager_GetFlashClearsCookie(t *testing.T) {
	t.Parallel()

	fm := session.NewFlashManager(&config.Config{Env: "test"})

	setRec := httptest.NewRecorder()
	fm.SetSuccess(setRec, "一度きり")
	cookie := findCookie(setRec, session.FlashCookieName)
	if cookie == nil {
		t.Fatalf("フラッシュ Cookie %q が設定されていない", session.FlashCookieName)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(cookie)
	getRec := httptest.NewRecorder()

	if flash := fm.GetFlash(getRec, req); flash == nil {
		t.Fatal("GetFlash() = nil, want flash")
	}

	cleared := findCookie(getRec, session.FlashCookieName)
	if cleared == nil {
		t.Fatal("GetFlash() は読み取り後に消去 Cookie を設定するはず")
	}
	if cleared.MaxAge >= 0 {
		t.Errorf("消去 Cookie の MaxAge = %d, want 負の値", cleared.MaxAge)
	}
}

func TestFlashManager_GetFlashNoCookie(t *testing.T) {
	t.Parallel()

	fm := session.NewFlashManager(&config.Config{Env: "test"})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	getRec := httptest.NewRecorder()

	if flash := fm.GetFlash(getRec, req); flash != nil {
		t.Errorf("GetFlash() = %v, want nil", flash)
	}
}

func TestFlashManager_GetFlashCorruptCookie(t *testing.T) {
	t.Parallel()

	fm := session.NewFlashManager(&config.Config{Env: "test"})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// "!" is outside the base64 alphabet, so decoding fails.
	// [Ja] "!" は base64 のアルファベット外なのでデコードに失敗する。
	req.AddCookie(&http.Cookie{Name: session.FlashCookieName, Value: "!!!not-base64!!!"})
	getRec := httptest.NewRecorder()

	if flash := fm.GetFlash(getRec, req); flash != nil {
		t.Errorf("壊れた Cookie の GetFlash() = %v, want nil", flash)
	}
	cleared := findCookie(getRec, session.FlashCookieName)
	if cleared == nil || cleared.MaxAge >= 0 {
		t.Error("壊れた Cookie は消去されるはず")
	}
}

func TestFlashManager_Middleware(t *testing.T) {
	t.Parallel()

	fm := session.NewFlashManager(&config.Config{Env: "test"})

	setRec := httptest.NewRecorder()
	fm.SetInfo(setRec, "ミドルウェア経由")
	cookie := findCookie(setRec, session.FlashCookieName)
	if cookie == nil {
		t.Fatalf("フラッシュ Cookie %q が設定されていない", session.FlashCookieName)
	}

	var got *session.FlashMessage
	handler := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = session.FlashFromContext(r.Context())
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()

	fm.Middleware(handler).ServeHTTP(rec, req)

	if got == nil {
		t.Fatal("FlashFromContext() = nil, ミドルウェアが context へ格納するはず")
	}
	if got.Type != session.FlashInfo {
		t.Errorf("flash.Type = %q, want %q", got.Type, session.FlashInfo)
	}
	if got.Message != "ミドルウェア経由" {
		t.Errorf("flash.Message = %q, want %q", got.Message, "ミドルウェア経由")
	}
}

func TestFlashFromContext_Absent(t *testing.T) {
	t.Parallel()

	if flash := session.FlashFromContext(context.Background()); flash != nil {
		t.Errorf("FlashFromContext() = %v, want nil", flash)
	}
}
