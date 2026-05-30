package welcome_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/handler/welcome"
)

// TestShow verifies that the top page returns HTTP 200 with an HTML body that
// includes the service name and the versioned asset references.
//
// [Ja] TestShow はトップページが HTTP 200 と、サービス名およびバージョン付きの
// アセット参照を含む HTML ボディを返すことを検証します。
func TestShow(t *testing.T) {
	t.Parallel()

	handler := welcome.NewHandler(&config.Config{Env: "dev"})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.Show(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusOK)
	}

	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
		t.Errorf("Content-Type = %q, want prefix %q", got, "text/html")
	}

	body := rec.Body.String()
	for _, want := range []string{"Groobb", "/static/css/style.css?v=", "/static/js/main.js?v="} {
		if !strings.Contains(body, want) {
			t.Errorf("response body does not contain %q", want)
		}
	}
}
