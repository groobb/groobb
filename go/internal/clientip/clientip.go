// clientipパッケージはHTTPリクエストからクライアントIPアドレスを解決します。
package clientip

import (
	"net/http"
	"net/netip"
	"strings"
)

// forwardedForHeaderは解決が読む唯一の転送ヘッダーです。プロキシはこのヘッダーを
// 置き換えるのではなく追記するため、下のチェーン走査は各hopとクライアントが書いた値とを
// 区別できます。プロキシが素通しするだけのヘッダー (CF-Connecting-IP・X-Real-IP) には
// その根拠がありません。上書きしないhopの背後にいる者は、そこへ任意のアドレスを書けます。
const forwardedForHeader = "X-Forwarded-For"

// GetClientIPは監査のためセッションに記録するクライアントIPを返します。
//
// trustedProxiesは、このインスタンス自身のリバースプロキシが接続してくるネットワークを
// 指します (config.Config.TrustedProxies)。1つも設定されていない場合は接続してきたピアの
// アドレスを使い、転送ヘッダーは一切読みません。直接公開されているインスタンスに必要なのは
// この挙動です。転送ヘッダーは、追記すると分かっているプロキシが前段に立つまでは、
// クライアントが与えた入力に過ぎません。
//
// ピアが信頼するプロキシの場合は、X-Forwarded-Forのチェーンを最も近いhopから外側へ辿り、
// 信頼するプロキシ自身ではない最初のアドレスをクライアントとして採用します。この向きで
// 辿るのは、各プロキシが自分の見たピアを追記するためです。左端の項目はクライアントが書いた
// 値そのものなので、それを採ると誰でも自分のアドレスを名乗れてしまいます。アドレスとして
// 解釈できない項目 ("unknown" や難読化された識別子) に当たった時点で走査を止め、ピアへ
// フォールバックします。位置づけられない項目はプロキシであることも示せず、そこを越えて
// 辿るとクライアントが操作できるテキストに届いてしまうためです。
//
// trustedProxiesには最も近いhopだけでなくすべてのhopを書きます。Cloudflareと
// ローカルのCaddyの背後にあるインスタンスがループバックアドレスだけを信頼している場合、
// 訪問者はCloudflareのエッジとして解決されます。チェーンの中で信頼されない最も近い項目が
// それになるためです。
func GetClientIP(r *http.Request, trustedProxies []netip.Prefix) string {
	peer, ok := parseAddr(r.RemoteAddr)
	if !ok {
		return r.RemoteAddr
	}

	if !isTrusted(peer, trustedProxies) {
		return peer.String()
	}

	if client, ok := forwardedClient(r.Header.Values(forwardedForHeader), trustedProxies); ok {
		return client.String()
	}

	return peer.String()
}

// forwardedClientは、X-Forwarded-Forのチェーンを最も近いhopから外側へ読んだとき、
// 信頼するプロキシではない最初のアドレスを返します。
//
// ヘッダーはチェーン全体を持つ1つの値として届くことも、hopごとに1つの値として届くことも
// あるため、値の並びをカンマ区切りの1本のチェーンとして読みます。どちらの形で届くかは
// 経由したプロキシ次第であり、値をまたいだ順序は追記された順序そのものだからです。
func forwardedClient(values []string, trustedProxies []netip.Prefix) (netip.Addr, bool) {
	entries := strings.Split(strings.Join(values, ","), ",")

	for i := len(entries) - 1; i >= 0; i-- {
		addr, ok := parseAddr(entries[i])
		if !ok {
			return netip.Addr{}, false
		}
		if !isTrusted(addr, trustedProxies) {
			return addr, true
		}
	}

	return netip.Addr{}, false
}

// parseAddrはRemoteAddrや転送ヘッダーに現れる形のアドレスを1つ解析します。
// アドレスと併せてポートも受け付けるのは、RemoteAddrが常にポートを伴い、転送ヘッダーも
// 伴うことがあるためです。IPv6に埋め込まれたIPv4アドレスは展開し、運用者がそのアドレスに
// 対して書くIPv4のprefixと照合できるようにします。IPv6のzoneは、prefixがインター
// フェイスに依存しないアドレス範囲を表すため取り除きます。
func parseAddr(value string) (netip.Addr, bool) {
	value = strings.TrimSpace(value)

	if addr, err := netip.ParseAddr(value); err == nil {
		return normalizeAddr(addr), true
	}
	if addrPort, err := netip.ParseAddrPort(value); err == nil {
		return normalizeAddr(addrPort.Addr()), true
	}
	if host, ok := unbracketedHost(value); ok {
		if addr, err := netip.ParseAddr(host); err == nil {
			return normalizeAddr(addr), true
		}
	}

	return netip.Addr{}, false
}

// unbracketedHostは、"アドレス:ポート" の形と同じ書き方でポートだけを省いた
// IPv6アドレス ("[2001:db8::1]") からブラケットを取り除きます。この形は上の2つの
// パーサーのどちらも受け付けません。ポートを追記するときにアドレスをブラケットで囲む
// プロキシは、ポートを省くときもブラケットを残すため、ここでチェーンの走査を止めると
// 訪問者がプロキシのものとして扱われてしまいます。
//
// 両側のブラケットを必須にしているのは、"[2001:db8::1" をアドレスとして読むと、途中で
// 切れた項目をプロキシが書いたものとして採ってしまうためです。走査は位置づけられない
// ものに当たったら止める必要があります。
func unbracketedHost(value string) (string, bool) {
	host, ok := strings.CutPrefix(value, "[")
	if !ok {
		return "", false
	}

	return strings.CutSuffix(host, "]")
}

// normalizeAddrはアドレスをprefixの照合に使う形へ揃えます。
func normalizeAddr(addr netip.Addr) netip.Addr {
	return addr.Unmap().WithZone("")
}

// isTrustedはaddrが信頼するプロキシのネットワークのいずれかに属するかどうかを
// 返します。
func isTrusted(addr netip.Addr, trustedProxies []netip.Prefix) bool {
	for _, prefix := range trustedProxies {
		if prefix.Contains(addr) {
			return true
		}
	}

	return false
}
