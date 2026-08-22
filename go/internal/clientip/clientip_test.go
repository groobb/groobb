package clientip_test

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"github.com/groobb/groobb/go/internal/clientip"
	"github.com/groobb/groobb/go/internal/config"
)

// loopback and cloudflareEdge stand for the two hops a self-hosted instance
// typically sits behind: a proxy on the same host, and the network in front of
// it. Two hops are used throughout because trusting only the nearest one is the
// mistake the resolution has to make visible.
//
// [Ja] loopback と cloudflareEdge は、セルフホストのインスタンスが背後に置く典型的な 2 つの
// hop、すなわち同じホスト上のプロキシと、その前段のネットワークを表します。全体を通して
// 2 hop を使うのは、最も近い hop だけを信頼することこそが、解決の結果に現れなければ
// ならない誤りだからです。
const (
	loopback       = "127.0.0.1"
	cloudflareEdge = "198.51.100.0/24"
)

// TestGetClientIP verifies the resolution as a whole: which source the client
// address is taken from, and that a forwarding header never reaches it from a
// peer that is not a configured proxy.
//
// [Ja] TestGetClientIP は解決の全体を検証します。クライアントのアドレスをどの入力から
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
			name:       "no trusted proxy: the peer is the client and the port is stripped",
			remoteAddr: "203.0.113.7:54321",
			want:       "203.0.113.7",
		},
		{
			name:       "no trusted proxy: an IPv6 peer keeps its address without the port",
			remoteAddr: "[2001:db8::1]:54321",
			want:       "2001:db8::1",
		},
		{
			name:       "no trusted proxy: a peer without a port is returned as it stands",
			remoteAddr: "203.0.113.7",
			want:       "203.0.113.7",
		},
		{
			name:       "no trusted proxy: an unparsable peer is returned as it stands",
			remoteAddr: "@",
			want:       "@",
		},
		{
			name:       "no trusted proxy: X-Forwarded-For is not read at all",
			remoteAddr: "203.0.113.7:54321",
			headers:    map[string][]string{"X-Forwarded-For": {"198.51.100.9"}},
			want:       "203.0.113.7",
		},
		{
			name:           "a peer outside the trusted proxies cannot forward an address",
			trustedProxies: []string{loopback},
			remoteAddr:     "203.0.113.7:54321",
			headers:        map[string][]string{"X-Forwarded-For": {"192.0.2.5"}},
			want:           "203.0.113.7",
		},
		{
			name:           "a trusted peer forwards the client address",
			trustedProxies: []string{loopback},
			remoteAddr:     "127.0.0.1:54321",
			headers:        map[string][]string{"X-Forwarded-For": {"203.0.113.7"}},
			want:           "203.0.113.7",
		},
		{
			name:           "the address the client wrote itself is not taken",
			trustedProxies: []string{loopback},
			remoteAddr:     "127.0.0.1:54321",
			headers:        map[string][]string{"X-Forwarded-For": {"192.0.2.5, 203.0.113.7"}},
			want:           "203.0.113.7",
		},
		{
			name:           "the chain is walked past every trusted hop",
			trustedProxies: []string{loopback, cloudflareEdge},
			remoteAddr:     "127.0.0.1:54321",
			headers:        map[string][]string{"X-Forwarded-For": {"203.0.113.7, 198.51.100.5"}},
			want:           "203.0.113.7",
		},
		{
			name:           "a hop left out of the list becomes the client",
			trustedProxies: []string{loopback},
			remoteAddr:     "127.0.0.1:54321",
			headers:        map[string][]string{"X-Forwarded-For": {"203.0.113.7, 198.51.100.5"}},
			want:           "198.51.100.5",
		},
		{
			name:           "a chain split across header lines is read as one chain",
			trustedProxies: []string{loopback, cloudflareEdge},
			remoteAddr:     "127.0.0.1:54321",
			headers:        map[string][]string{"X-Forwarded-For": {"203.0.113.7", "198.51.100.5"}},
			want:           "203.0.113.7",
		},
		{
			name:           "a forwarded address carrying a port keeps only the address",
			trustedProxies: []string{loopback},
			remoteAddr:     "127.0.0.1:54321",
			headers:        map[string][]string{"X-Forwarded-For": {"203.0.113.7:41234"}},
			want:           "203.0.113.7",
		},
		{
			name:           "a chain of trusted proxies alone falls back to the peer",
			trustedProxies: []string{loopback, cloudflareEdge},
			remoteAddr:     "127.0.0.1:54321",
			headers:        map[string][]string{"X-Forwarded-For": {"198.51.100.9, 198.51.100.5"}},
			want:           "127.0.0.1",
		},
		{
			name:           "an entry that is not an address stops the walk",
			trustedProxies: []string{loopback},
			remoteAddr:     "127.0.0.1:54321",
			headers:        map[string][]string{"X-Forwarded-For": {"203.0.113.7, unknown"}},
			want:           "127.0.0.1",
		},
		{
			name:           "a trusted peer without the header is the client itself",
			trustedProxies: []string{loopback},
			remoteAddr:     "127.0.0.1:54321",
			want:           "127.0.0.1",
		},
		{
			name:           "a 4-in-6 peer matches the IPv4 prefix written for it",
			trustedProxies: []string{loopback},
			remoteAddr:     "[::ffff:127.0.0.1]:54321",
			headers:        map[string][]string{"X-Forwarded-For": {"203.0.113.7"}},
			want:           "203.0.113.7",
		},
		{
			name:           "an IPv6 peer with a zone matches its configured prefix",
			trustedProxies: []string{"fe80::/10"},
			remoteAddr:     "[fe80::1%eth0]:54321",
			headers:        map[string][]string{"X-Forwarded-For": {"2001:db8::1"}},
			want:           "2001:db8::1",
		},
		{
			name:           "a 4-in-6 block written for the proxy matches the IPv4 peer",
			trustedProxies: []string{"::ffff:127.0.0.1/128"},
			remoteAddr:     "127.0.0.1:54321",
			headers:        map[string][]string{"X-Forwarded-For": {"203.0.113.7"}},
			want:           "203.0.113.7",
		},
		{
			name:           "a forwarded IPv6 address in brackets without a port is read",
			trustedProxies: []string{loopback},
			remoteAddr:     "127.0.0.1:54321",
			headers:        map[string][]string{"X-Forwarded-For": {"[2001:db8::1]"}},
			want:           "2001:db8::1",
		},
		{
			name:           "a forwarded address left unclosed stops the walk",
			trustedProxies: []string{loopback},
			remoteAddr:     "127.0.0.1:54321",
			headers:        map[string][]string{"X-Forwarded-For": {"[2001:db8::1"}},
			want:           "127.0.0.1",
		},
		{
			name:           "the pass-through headers are not consulted",
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
				t.Errorf("GetClientIP() = %q, want %q", got, tt.want)
			}
		})
	}
}

// mustPrefixes parses the trusted proxies of a case through the parser the
// configuration uses, so that a case states them as the text an operator writes
// and the resolution is checked against the prefixes an instance would actually
// hold. A malformed value fails the case rather than the resolution.
//
// Parsing them here instead would be a second normalisation of the same text,
// and the two could then disagree about which addresses an entry covers without
// any test noticing.
//
// [Ja] mustPrefixes はケースの信頼するプロキシを、設定が使うのと同じパーサーで解析します。
// ケースは運用者が書くテキストのまま記述でき、解決は実際のインスタンスが持つ prefix と
// 照合されます。不正な値は解決ではなくケース自体を失敗させます。
//
// ここで独自に解析すると同じテキストに対する 2 つ目の正規化になり、ある項目がどのアドレスを
// 覆うかについて両者の見解が食い違っても、どのテストもそれに気づけなくなります。
func mustPrefixes(t *testing.T, values []string) []netip.Prefix {
	t.Helper()

	prefixes := make([]netip.Prefix, 0, len(values))
	for _, value := range values {
		prefix, err := config.ParseTrustedProxy(value)
		if err != nil {
			t.Fatalf("the trusted proxy %q of this case %s", value, err)
		}
		prefixes = append(prefixes, prefix)
	}

	return prefixes
}
