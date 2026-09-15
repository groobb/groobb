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
				t.Fatalf("フラッシュCookie %q が設定されていない", session.FlashCookieName)
			}
			if cookie.HttpOnly {
				t.Error("フラッシュCookieがHttpOnlyになっている (JSから読めるようHttpOnlyでないことを期待)")
			}

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.AddCookie(cookie)
			getRec := httptest.NewRecorder()

			flash := fm.GetFlash(getRec, req)
			if flash == nil {
				t.Fatal("GetFlash() = nil、期待値はフラッシュ")
			}
			if flash.Type != tt.wantType {
				t.Errorf("flash.Type = %q、期待値 = %q", flash.Type, tt.wantType)
			}
			if flash.Message != "こんにちは" {
				t.Errorf("flash.Message = %q、期待値 = %q", flash.Message, "こんにちは")
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
		t.Fatalf("フラッシュCookie %q が設定されていない", session.FlashCookieName)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(cookie)
	getRec := httptest.NewRecorder()

	if flash := fm.GetFlash(getRec, req); flash == nil {
		t.Fatal("GetFlash() = nil、期待値はフラッシュ")
	}

	cleared := findCookie(getRec, session.FlashCookieName)
	if cleared == nil {
		t.Fatal("GetFlash() は読み取り後に消去Cookieを設定するはず")
	}
	if cleared.MaxAge >= 0 {
		t.Errorf("消去CookieのMaxAge = %d、期待値は負の値", cleared.MaxAge)
	}
}

func TestFlashManager_GetFlashNoCookie(t *testing.T) {
	t.Parallel()

	fm := session.NewFlashManager(&config.Config{Env: "test"})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	getRec := httptest.NewRecorder()

	if flash := fm.GetFlash(getRec, req); flash != nil {
		t.Errorf("GetFlash() = %v、期待値 = nil", flash)
	}
}

func TestFlashManager_GetFlashCorruptCookie(t *testing.T) {
	t.Parallel()

	fm := session.NewFlashManager(&config.Config{Env: "test"})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// "!" はbase64のアルファベット外なのでデコードに失敗する。
	req.AddCookie(&http.Cookie{Name: session.FlashCookieName, Value: "!!!not-base64!!!"})
	getRec := httptest.NewRecorder()

	if flash := fm.GetFlash(getRec, req); flash != nil {
		t.Errorf("壊れたCookieのGetFlash() = %v、期待値 = nil", flash)
	}
	cleared := findCookie(getRec, session.FlashCookieName)
	if cleared == nil || cleared.MaxAge >= 0 {
		t.Error("壊れたCookieは消去されるはず")
	}
}

func TestFlashManager_Middleware(t *testing.T) {
	t.Parallel()

	fm := session.NewFlashManager(&config.Config{Env: "test"})

	setRec := httptest.NewRecorder()
	fm.SetInfo(setRec, "ミドルウェア経由")
	cookie := findCookie(setRec, session.FlashCookieName)
	if cookie == nil {
		t.Fatalf("フラッシュCookie %q が設定されていない", session.FlashCookieName)
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
		t.Fatal("FlashFromContext() = nil、ミドルウェアがcontextへ格納するはず")
	}
	if got.Type != session.FlashInfo {
		t.Errorf("flash.Type = %q、期待値 = %q", got.Type, session.FlashInfo)
	}
	if got.Message != "ミドルウェア経由" {
		t.Errorf("flash.Message = %q、期待値 = %q", got.Message, "ミドルウェア経由")
	}
}

func TestFlashFromContext_Absent(t *testing.T) {
	t.Parallel()

	if flash := session.FlashFromContext(context.Background()); flash != nil {
		t.Errorf("FlashFromContext() = %v、期待値 = nil", flash)
	}
}
