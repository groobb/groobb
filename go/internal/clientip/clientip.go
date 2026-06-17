// Package clientip resolves the client IP address from an HTTP request.
//
// [Ja] clientip パッケージは HTTP リクエストからクライアント IP アドレスを解決します。
package clientip

import (
	"net"
	"net/http"
)

// GetClientIP returns the client IP recorded on the session for audit. It strips
// the port from RemoteAddr ("host:port"), falling back to the raw value when it
// has no port.
//
// It does not yet account for a reverse proxy's forwarding headers
// (X-Forwarded-For / CF-Connecting-IP). Trusting those is only safe once the
// proxy in front is known to overwrite client-supplied values, otherwise a
// client could spoof the recorded IP; proxy-aware resolution is therefore left
// to the later reverse-proxy / sign-in rate-limit task. The sister Korylus
// projects already resolve those headers here, and Groobb converges on that form
// in that task.
//
// [Ja] GetClientIP は監査のためセッションに記録するクライアント IP を返します。
// RemoteAddr ("host:port") からポートを除き、ポートが無ければ生の値にフォールバック
// します。
//
// リバースプロキシの転送ヘッダー (X-Forwarded-For / CF-Connecting-IP) はまだ考慮
// しません。これらを信頼できるのは前段のプロキシがクライアント申告値を上書きすると
// 分かっている場合に限られ、そうでなければクライアントが記録 IP を詐称できるため、
// プロキシ対応の解決は後続のリバースプロキシ / サインインのレート制限タスクに委ねます。
// 姉妹 Korylus プロジェクトは既にここでこれらのヘッダーを解決しており、Groobb はその
// タスクで同じ形に収束させます。
func GetClientIP(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}
