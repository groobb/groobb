package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/middleware"
)

func TestMethodOverride(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		method     string
		formBody   string
		wantMethod string
	}{
		{name: "POST に _method=DELETE で DELETE へ上書きする", method: http.MethodPost, formBody: "_method=DELETE", wantMethod: http.MethodDelete},
		{name: "POST に _method=PATCH で PATCH へ上書きする", method: http.MethodPost, formBody: "_method=PATCH", wantMethod: http.MethodPatch},
		{name: "POST に _method=PUT で PUT へ上書きする", method: http.MethodPost, formBody: "_method=PUT", wantMethod: http.MethodPut},
		{name: "小文字の _method=delete も大文字化して上書きする", method: http.MethodPost, formBody: "_method=delete", wantMethod: http.MethodDelete},
		{name: "_method が無ければ POST のまま", method: http.MethodPost, formBody: "name=foo", wantMethod: http.MethodPost},
		{name: "非対応の _method=GET は無視して POST のまま", method: http.MethodPost, formBody: "_method=GET", wantMethod: http.MethodPost},
		{name: "GET リクエストは上書きしない", method: http.MethodGet, formBody: "", wantMethod: http.MethodGet},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var gotMethod string
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod = r.Method
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(tt.method, "/", strings.NewReader(tt.formBody))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rec := httptest.NewRecorder()

			middleware.MethodOverride(next).ServeHTTP(rec, req)

			if gotMethod != tt.wantMethod {
				t.Errorf("r.Method = %q, want %q", gotMethod, tt.wantMethod)
			}
		})
	}
}

func TestMethodOverride_PreservesOtherFormValues(t *testing.T) {
	t.Parallel()

	var gotMethod, gotEmail string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotEmail = r.FormValue("email")
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("_method=PATCH&email=user@example.com"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	middleware.MethodOverride(next).ServeHTTP(rec, req)

	if gotMethod != http.MethodPatch {
		t.Errorf("r.Method = %q, want %q", gotMethod, http.MethodPatch)
	}
	// The downstream handler can still read other form values after the
	// middleware parsed the body to look up _method.
	//
	// [Ja] ミドルウェアが _method 参照のために body を解析した後でも、後続ハンドラーは
	// 他のフォーム値を読める。
	if gotEmail != "user@example.com" {
		t.Errorf("email = %q, want %q", gotEmail, "user@example.com")
	}
}
