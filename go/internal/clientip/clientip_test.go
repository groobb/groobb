package clientip_test

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"github.com/groobb/groobb/go/internal/clientip"
	"github.com/groobb/groobb/go/internal/config"
)

// loopbackとcloudflareEdgeは、セルフホストのインスタンスが背後に置く典型的な2つの
// hop、すなわち同じホスト上のプロキシと、その前段のネットワークを表します。全体を通して
// 2 hopを使うのは、最も近いhopだけを信頼することこそが、解決の結果に現れなければ
// ならない誤りだからです。
const (
	loopback       = "127.0.0.1"
	cloudflareEdge = "198.51.100.0/24"
)

// TestGetClientIPは解決の全体を検証します。クライアントのアドレスをどの入力から
// 取るか、そして設定されたプロキシではないピアからの転送ヘッダーが決してそこへ届かない
// ことです。
func TestGetClientIP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		trustedProxies []string
		remoteAddr     string
		headers        map[string][]string
		want           string
	}{
		{
			name:       "信頼するプロキシ無し: 接続元がクライアントで、ポートは取り除かれる",
			remoteAddr: "203.0.113.7:54321",
			want:       "203.0.113.7",
		},
		{
			name:       "信頼するプロキシ無し: IPv6の接続元はポートを除いたアドレスになる",
			remoteAddr: "[2001:db8::1]:54321",
			want:       "2001:db8::1",
		},
		{
			name:       "信頼するプロキシ無し: ポートの無い接続元はそのまま返る",
			remoteAddr: "203.0.113.7",
			want:       "203.0.113.7",
		},
		{
			name:       "信頼するプロキシ無し: 解析できない接続元はそのまま返る",
			remoteAddr: "@",
			want:       "@",
		},
		{
			name:       "信頼するプロキシ無し: X-Forwarded-Forはまったく読まない",
			remoteAddr: "203.0.113.7:54321",
			headers:    map[string][]string{"X-Forwarded-For": {"198.51.100.9"}},
			want:       "203.0.113.7",
		},
		{
			name:           "信頼するプロキシ外の接続元はアドレスを転送できない",
			trustedProxies: []string{loopback},
			remoteAddr:     "203.0.113.7:54321",
			headers:        map[string][]string{"X-Forwarded-For": {"192.0.2.5"}},
			want:           "203.0.113.7",
		},
		{
			name:           "信頼する接続元はクライアントのアドレスを転送する",
			trustedProxies: []string{loopback},
			remoteAddr:     "127.0.0.1:54321",
			headers:        map[string][]string{"X-Forwarded-For": {"203.0.113.7"}},
			want:           "203.0.113.7",
		},
		{
			name:           "クライアントが自分で書いたアドレスは採用しない",
			trustedProxies: []string{loopback},
			remoteAddr:     "127.0.0.1:54321",
			headers:        map[string][]string{"X-Forwarded-For": {"192.0.2.5, 203.0.113.7"}},
			want:           "203.0.113.7",
		},
		{
			name:           "チェーンは信頼するホップをすべて越えて辿る",
			trustedProxies: []string{loopback, cloudflareEdge},
			remoteAddr:     "127.0.0.1:54321",
			headers:        map[string][]string{"X-Forwarded-For": {"203.0.113.7, 198.51.100.5"}},
			want:           "203.0.113.7",
		},
		{
			name:           "一覧に無いホップがクライアントになる",
			trustedProxies: []string{loopback},
			remoteAddr:     "127.0.0.1:54321",
			headers:        map[string][]string{"X-Forwarded-For": {"203.0.113.7, 198.51.100.5"}},
			want:           "198.51.100.5",
		},
		{
			name:           "複数のヘッダー行に分かれたチェーンを1つのチェーンとして読む",
			trustedProxies: []string{loopback, cloudflareEdge},
			remoteAddr:     "127.0.0.1:54321",
			headers:        map[string][]string{"X-Forwarded-For": {"203.0.113.7", "198.51.100.5"}},
			want:           "203.0.113.7",
		},
		{
			name:           "ポート付きで転送されたアドレスはアドレスだけを残す",
			trustedProxies: []string{loopback},
			remoteAddr:     "127.0.0.1:54321",
			headers:        map[string][]string{"X-Forwarded-For": {"203.0.113.7:41234"}},
			want:           "203.0.113.7",
		},
		{
			name:           "信頼するプロキシだけのチェーンは接続元に戻る",
			trustedProxies: []string{loopback, cloudflareEdge},
			remoteAddr:     "127.0.0.1:54321",
			headers:        map[string][]string{"X-Forwarded-For": {"198.51.100.9, 198.51.100.5"}},
			want:           "127.0.0.1",
		},
		{
			name:           "アドレスでない項目で辿るのを止める",
			trustedProxies: []string{loopback},
			remoteAddr:     "127.0.0.1:54321",
			headers:        map[string][]string{"X-Forwarded-For": {"203.0.113.7, unknown"}},
			want:           "127.0.0.1",
		},
		{
			name:           "ヘッダーの無い信頼する接続元はそれ自体がクライアント",
			trustedProxies: []string{loopback},
			remoteAddr:     "127.0.0.1:54321",
			want:           "127.0.0.1",
		},
		{
			name:           "4-in-6の接続元はそれ用に書いたIPv4のprefixに一致する",
			trustedProxies: []string{loopback},
			remoteAddr:     "[::ffff:127.0.0.1]:54321",
			headers:        map[string][]string{"X-Forwarded-For": {"203.0.113.7"}},
			want:           "203.0.113.7",
		},
		{
			name:           "ゾーン付きのIPv6の接続元は設定したprefixに一致する",
			trustedProxies: []string{"fe80::/10"},
			remoteAddr:     "[fe80::1%eth0]:54321",
			headers:        map[string][]string{"X-Forwarded-For": {"2001:db8::1"}},
			want:           "2001:db8::1",
		},
		{
			name:           "プロキシ用に書いた4-in-6のブロックはIPv4の接続元に一致する",
			trustedProxies: []string{"::ffff:127.0.0.1/128"},
			remoteAddr:     "127.0.0.1:54321",
			headers:        map[string][]string{"X-Forwarded-For": {"203.0.113.7"}},
			want:           "203.0.113.7",
		},
		{
			name:           "ポートの無い角括弧付きで転送されたIPv6アドレスを読む",
			trustedProxies: []string{loopback},
			remoteAddr:     "127.0.0.1:54321",
			headers:        map[string][]string{"X-Forwarded-For": {"[2001:db8::1]"}},
			want:           "2001:db8::1",
		},
		{
			name:           "閉じていない転送アドレスで辿るのを止める",
			trustedProxies: []string{loopback},
			remoteAddr:     "127.0.0.1:54321",
			headers:        map[string][]string{"X-Forwarded-For": {"[2001:db8::1"}},
			want:           "127.0.0.1",
		},
		{
			name:           "素通しのヘッダーは参照しない",
			trustedProxies: []string{loopback},
			remoteAddr:     "127.0.0.1:54321",
			headers: map[string][]string{
				"CF-Connecting-IP": {"192.0.2.5"},
				"X-Real-IP":        {"192.0.2.6"},
				"X-Forwarded-For":  {"203.0.113.7"},
			},
			want: "203.0.113.7",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr
			for name, values := range tt.headers {
				for _, value := range values {
					req.Header.Add(name, value)
				}
			}

			if got := clientip.GetClientIP(req, mustPrefixes(t, tt.trustedProxies)); got != tt.want {
				t.Errorf("GetClientIP() = %q、期待値 = %q", got, tt.want)
			}
		})
	}
}

// mustPrefixesはケースの信頼するプロキシを、設定が使うのと同じパーサーで解析します。
// ケースは運用者が書くテキストのまま記述でき、解決は実際のインスタンスが持つprefixと
// 照合されます。不正な値は解決ではなくケース自体を失敗させます。
//
// ここで独自に解析すると同じテキストに対する2つ目の正規化になり、ある項目がどのアドレスを
// 覆うかについて両者の見解が食い違っても、どのテストもそれに気づけなくなります。
func mustPrefixes(t *testing.T, values []string) []netip.Prefix {
	t.Helper()

	prefixes := make([]netip.Prefix, 0, len(values))
	for _, value := range values {
		prefix, err := config.ParseTrustedProxy(value)
		if err != nil {
			t.Fatalf("このケースの信頼するプロキシ %q が不正: %s", value, err)
		}
		prefixes = append(prefixes, prefix)
	}

	return prefixes
}
