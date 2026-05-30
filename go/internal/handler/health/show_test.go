package health_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/groobb/groobb/go/internal/handler/health"
)

// TestShow verifies that the health check endpoint returns HTTP 200 with a
// JSON body of {"status": "ok"}.
//
// [Ja] TestShow はヘルスチェックエンドポイントが HTTP 200 と
// {"status": "ok"} の JSON を返すことを検証します。
func TestShow(t *testing.T) {
	t.Parallel()

	handler := health.NewHandler()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	handler.Show(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusOK)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want %q", got, "application/json")
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf(`status = %q, want %q`, body["status"], "ok")
	}
}
