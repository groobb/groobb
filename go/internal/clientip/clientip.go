// Package clientip resolves the client IP address from an HTTP request.
//
// [Ja] clientip パッケージは HTTP リクエストからクライアント IP アドレスを解決します。
package clientip

import (
	"net/http"
	"net/netip"
	"strings"
)

// forwardedForHeader is the only forwarding header the resolution reads. A
// proxy appends to it rather than replacing it, which is what lets the chain
// walk below tell the hops apart from what the client wrote. A header a proxy
// merely passes through (CF-Connecting-IP, X-Real-IP) carries no such evidence:
// whoever sits behind a hop that does not overwrite it can put any address
// there.
//
// [Ja] forwardedForHeader は解決が読む唯一の転送ヘッダーです。プロキシはこのヘッダーを
// 置き換えるのではなく追記するため、下のチェーン走査は各 hop とクライアントが書いた値とを
// 区別できます。プロキシが素通しするだけのヘッダー (CF-Connecting-IP・X-Real-IP) には
// その根拠がありません。上書きしない hop の背後にいる者は、そこへ任意のアドレスを書けます。
const forwardedForHeader = "X-Forwarded-For"

// GetClientIP returns the client IP recorded on the session for audit.
//
// trustedProxies names the networks the instance's own reverse proxies connect
// from (config.Config.TrustedProxies). With none configured, the address of the
// peer that connected is used and no forwarding header is read at all, which is
// what an instance exposed directly needs: a forwarding header is
// client-supplied input until a proxy known to append to it sits in front.
//
// When the peer is a trusted proxy, the X-Forwarded-For chain is walked from
// the closest hop outwards and the first address that is not itself a trusted
// proxy is taken as the client. The walk runs in that direction because each
// proxy appends the peer it saw: the leftmost entry is whatever the client
// wrote, so taking it would let anyone name their own address. The walk stops
// at an entry that is not an address at all ("unknown", an obfuscated
// identifier) and falls back to the peer, because an entry that cannot be
// placed cannot be shown to be a proxy either, and walking past it would reach
// client-controlled text.
//
// Every hop belongs in trustedProxies, not only the nearest one: an instance
// behind Cloudflare and a local Caddy that trusts the loopback address alone
// resolves its visitors to Cloudflare's edge, since that is then the closest
// entry in the chain that is not trusted.
//
// [Ja] GetClientIP は監査のためセッションに記録するクライアント IP を返します。
//
// trustedProxies は、このインスタンス自身のリバースプロキシが接続してくるネットワークを
// 指します (config.Config.TrustedProxies)。1 つも設定されていない場合は接続してきたピアの
// アドレスを使い、転送ヘッダーは一切読みません。直接公開されているインスタンスに必要なのは
// この挙動です。転送ヘッダーは、追記すると分かっているプロキシが前段に立つまでは、
// クライアントが与えた入力に過ぎません。
//
// ピアが信頼するプロキシの場合は、X-Forwarded-For のチェーンを最も近い hop から外側へ辿り、
// 信頼するプロキシ自身ではない最初のアドレスをクライアントとして採用します。この向きで
// 辿るのは、各プロキシが自分の見たピアを追記するためです。左端の項目はクライアントが書いた
// 値そのものなので、それを採ると誰でも自分のアドレスを名乗れてしまいます。アドレスとして
// 解釈できない項目 ("unknown" や難読化された識別子) に当たった時点で走査を止め、ピアへ
// フォールバックします。位置づけられない項目はプロキシであることも示せず、そこを越えて
// 辿るとクライアントが操作できるテキストに届いてしまうためです。
//
// trustedProxies には最も近い hop だけでなくすべての hop を書きます。Cloudflare と
// ローカルの Caddy の背後にあるインスタンスがループバックアドレスだけを信頼している場合、
// 訪問者は Cloudflare のエッジとして解決されます。チェーンの中で信頼されない最も近い項目が
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

// forwardedClient returns the first address of the X-Forwarded-For chain, read
// from its closest hop outwards, that is not a trusted proxy.
//
// The header may arrive as one value holding the whole chain or as one value
// per hop, so the values are read as a single comma-separated chain: which of
// the two a request carries is up to the proxies it passed through, and the
// order across values is the order they were appended in.
//
// [Ja] forwardedClient は、X-Forwarded-For のチェーンを最も近い hop から外側へ読んだとき、
// 信頼するプロキシではない最初のアドレスを返します。
//
// ヘッダーはチェーン全体を持つ 1 つの値として届くことも、hop ごとに 1 つの値として届くことも
// あるため、値の並びをカンマ区切りの 1 本のチェーンとして読みます。どちらの形で届くかは
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

// parseAddr parses one address as it appears in RemoteAddr or in a forwarding
// header. A port is accepted along with the address because RemoteAddr always
// carries one and a forwarding header may. An IPv4-in-IPv6 address is
// unwrapped so that it is matched against the IPv4 prefix an operator would
// write for it, and an IPv6 zone is removed because a prefix describes an
// address range independently of an interface.
//
// [Ja] parseAddr は RemoteAddr や転送ヘッダーに現れる形のアドレスを 1 つ解析します。
// アドレスと併せてポートも受け付けるのは、RemoteAddr が常にポートを伴い、転送ヘッダーも
// 伴うことがあるためです。IPv6 に埋め込まれた IPv4 アドレスは展開し、運用者がそのアドレスに
// 対して書く IPv4 の prefix と照合できるようにします。IPv6 の zone は、prefix がインター
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

// unbracketedHost removes the brackets from an IPv6 address written the way the
// "address:port" form writes it but with the port left off ("[2001:db8::1]"),
// which neither parser above accepts. A proxy that brackets the address when it
// appends the port keeps the brackets when it omits it, and ending the chain
// walk there would attribute the visitor to the proxy instead.
//
// Both brackets are required: reading "[2001:db8::1" as an address would take a
// truncated entry for one the proxy wrote, and the walk has to stop at anything
// it cannot place.
//
// [Ja] unbracketedHost は、"アドレス:ポート" の形と同じ書き方でポートだけを省いた
// IPv6 アドレス ("[2001:db8::1]") からブラケットを取り除きます。この形は上の 2 つの
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

// normalizeAddr puts an address in the form used for prefix comparisons.
//
// [Ja] normalizeAddr はアドレスを prefix の照合に使う形へ揃えます。
func normalizeAddr(addr netip.Addr) netip.Addr {
	return addr.Unmap().WithZone("")
}

// isTrusted reports whether addr falls in one of the trusted proxy networks.
//
// [Ja] isTrusted は addr が信頼するプロキシのネットワークのいずれかに属するかどうかを
// 返します。
func isTrusted(addr netip.Addr, trustedProxies []netip.Prefix) bool {
	for _, prefix := range trustedProxies {
		if prefix.Contains(addr) {
			return true
		}
	}

	return false
}
