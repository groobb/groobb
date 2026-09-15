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
		{name: "POSTに _method=DELETEでDELETEへ上書きする", method: http.MethodPost, formBody: "_method=DELETE", wantMethod: http.MethodDelete},
		{name: "POSTに _method=PATCHでPATCHへ上書きする", method: http.MethodPost, formBody: "_method=PATCH", wantMethod: http.MethodPatch},
		{name: "POSTに _method=PUTでPUTへ上書きする", method: http.MethodPost, formBody: "_method=PUT", wantMethod: http.MethodPut},
		{name: "小文字の _method=deleteも大文字化して上書きする", method: http.MethodPost, formBody: "_method=delete", wantMethod: http.MethodDelete},
		{name: "_methodが無ければPOSTのまま", method: http.MethodPost, formBody: "name=foo", wantMethod: http.MethodPost},
		{name: "非対応の _method=GETは無視してPOSTのまま", method: http.MethodPost, formBody: "_method=GET", wantMethod: http.MethodPost},
		{name: "GETリクエストは上書きしない", method: http.MethodGet, formBody: "", wantMethod: http.MethodGet},
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
				t.Errorf("r.Method = %q、期待値 = %q", gotMethod, tt.wantMethod)
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
		t.Errorf("r.Method = %q、期待値 = %q", gotMethod, http.MethodPatch)
	}
	// ミドルウェアが _method参照のためにbodyを解析した後でも、後続ハンドラーは
	// 他のフォーム値を読める。
	if gotEmail != "user@example.com" {
		t.Errorf("email = %q、期待値 = %q", gotEmail, "user@example.com")
	}
}
