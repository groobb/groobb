package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/groobb/groobb/go/internal/config"
)

// TestAssetCacheは、配信されたアセットがその環境で許される保持期間を伴い、
// アセット自体ではないレスポンスが保持期間を伴わないことを検証します。キャッシュされた
// 404やリダイレクトは、それを生んだ問題より長く残ってしまうためです。
func TestAssetCache(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		env              string
		handler          http.HandlerFunc
		wantCacheControl string
	}{
		{
			name: "dev以外では配信したアセットを1年間保持させる",
			env:  "prod",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			wantCacheControl: "private, max-age=31536000, immutable",
		},
		{
			name: "ステータスを指定せずに書いたボディは配信として扱う",
			env:  "prod",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				if _, err := w.Write([]byte("body { }")); err != nil {
					t.Errorf("レスポンスボディの書き込みに失敗: %v", err)
				}
			},
			wantCacheControl: "private, max-age=31536000, immutable",
		},
		{
			name: "部分応答のアセットは配信時の方針を保つ",
			env:  "prod",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusPartialContent)
			},
			wantCacheControl: "private, max-age=31536000, immutable",
		},
		{
			name: "devではアセットを保存させない",
			env:  "dev",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			wantCacheControl: "no-store",
		},
		{
			name: "存在しないアセットは保存させない",
			env:  "prod",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			wantCacheControl: "private, no-store",
		},
		{
			name: "リダイレクトは保存させない",
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
				t.Errorf("Cache-Control = %q、期待値 = %q", got, tt.wantCacheControl)
			}
		})
	}
}
