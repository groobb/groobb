package clientip_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/groobb/groobb/go/internal/clientip"
)

// TestGetClientIP verifies that the port is stripped from RemoteAddr, that IPv6
// addresses are handled, and that a value without a port is returned unchanged.
// Forwarding headers are intentionally ignored for now, so an X-Forwarded-For is
// not consulted.
//
// [Ja] TestGetClientIP は RemoteAddr からポートが除かれること、IPv6 アドレスが扱える
// こと、ポートの無い値がそのまま返ることを検証します。転送ヘッダーは現時点では意図的に
// 無視するため、X-Forwarded-For は参照されません。
func TestGetClientIP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		remoteAddr   string
		forwardedFor string
		want         string
	}{
		{
			name:       "IPv4 with port strips the port",
			remoteAddr: "203.0.113.7:54321",
			want:       "203.0.113.7",
		},
		{
			name:       "IPv6 with port strips the port",
			remoteAddr: "[2001:db8::1]:54321",
			want:       "2001:db8::1",
		},
		{
			name:       "value without a port is returned as-is",
			remoteAddr: "203.0.113.7",
			want:       "203.0.113.7",
		},
		{
			name:         "X-Forwarded-For is ignored (not yet proxy-aware)",
			remoteAddr:   "203.0.113.7:54321",
			forwardedFor: "198.51.100.9",
			want:         "203.0.113.7",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.forwardedFor != "" {
				req.Header.Set("X-Forwarded-For", tt.forwardedFor)
			}

			if got := clientip.GetClientIP(req); got != tt.want {
				t.Errorf("GetClientIP() = %q, want %q", got, tt.want)
			}
		})
	}
}
