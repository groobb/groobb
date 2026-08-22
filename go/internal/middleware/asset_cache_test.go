package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/groobb/groobb/go/internal/config"
)

// TestAssetCache verifies that a served asset carries the lifetime its
// environment allows, and that a response which is not the asset itself carries
// none: a cached 404 or redirect would outlive the problem that produced it.
//
// [Ja] TestAssetCache は、配信されたアセットがその環境で許される保持期間を伴い、
// アセット自体ではないレスポンスが保持期間を伴わないことを検証します。キャッシュされた
// 404 やリダイレクトは、それを生んだ問題より長く残ってしまうためです。
func TestAssetCache(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		env              string
		handler          http.HandlerFunc
		wantCacheControl string
	}{
		{
			name: "non-dev keeps a served asset for a year",
			env:  "prod",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			wantCacheControl: "private, max-age=31536000, immutable",
		},
		{
			name: "a body written without a status is treated as served",
			env:  "prod",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				if _, err := w.Write([]byte("body { }")); err != nil {
					t.Errorf("failed to write the response body: %v", err)
				}
			},
			wantCacheControl: "private, max-age=31536000, immutable",
		},
		{
			name: "a partial asset keeps the served asset policy",
			env:  "prod",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusPartialContent)
			},
			wantCacheControl: "private, max-age=31536000, immutable",
		},
		{
			name: "dev does not store assets",
			env:  "dev",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			wantCacheControl: "no-store",
		},
		{
			name: "a missing asset is not stored",
			env:  "prod",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			wantCacheControl: "private, no-store",
		},
		{
			name: "a redirect is not stored",
			env:  "prod",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusMovedPermanently)
			},
			wantCacheControl: "private, no-store",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{Env: tt.env}
			handler := AssetCache(cfg)(tt.handler)

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/static/css/style.css", nil))

			if got := rec.Header().Get("Cache-Control"); got != tt.wantCacheControl {
				t.Errorf("Cache-Control = %q, want %q", got, tt.wantCacheControl)
			}
		})
	}
}
