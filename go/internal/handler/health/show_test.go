package health_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/groobb/groobb/go/internal/handler/health"
)

// TestShowはヘルスチェックエンドポイントがHTTP 200と
// {"status": "ok"} のJSONを返すことを検証します。
func TestShow(t *testing.T) {
	t.Parallel()

	handler := health.NewHandler()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	handler.Show(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q、期待値 = %q", got, "application/json")
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("レスポンスボディのデコードに失敗: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf(`レスポンスのstatus = %q、期待値 = %q`, body["status"], "ok")
	}
}
