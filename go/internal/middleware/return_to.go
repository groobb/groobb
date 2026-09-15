package middleware

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/groobb/groobb/go/internal/templates"
)

// SanitizeReturnToはrawが本アプリケーションから安全にリダイレクトできる遷移先を
// 指しているときに正規化したrequest URIを返し、それ以外では "" を返す。受け付けるのは
// 同一オリジンの相対パスだけで、先頭が "/" 1つ、その次が "/" でも "\" でもないものに
// 限る。この値は外部 (クエリパラメータ、続いてフォームフィールド) から渡ってくるため、
// 別オリジンを指す値 ("//evil.example.com"、"https://evil.example.com") やブラウザが
// 別オリジンとして解釈する値 ("/\evil.example.com") はリダイレクトせずに破棄する。
// これがサインインフローをオープンリダイレクトにしないための要である。受け付けたパスは
// 再エンコードし、フラグメントを落として返す。RequireAuthが取得するHTTP request URI
// にはブラウザのトップレベルフラグメントが含まれないため、外部からquery / form値として
// 渡された値も同じrequest URIの契約へ正規化する。
func SanitizeReturnTo(raw string) string {
	if !strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") || strings.HasPrefix(raw, `/\`) {
		return ""
	}

	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "" || u.Host != "" {
		return ""
	}

	return u.RequestURI()
}

// signInPathWithReturnToは匿名リクエストrに対するサインインのパスを、サインイン後に
// 元のURLへ戻れるようリクエスト先を載せて返す。載せるのはGETとHEADのときだけとする。
// 安全でないメソッドの宛先を後から素のGETでなぞると、訪問者が求めていないページに着地させて
// しまうため、それらは素のサインインパスにフォールバックする。
func signInPathWithReturnTo(r *http.Request) string {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return templates.SignInPath().String()
	}

	return templates.SignInPath().WithReturnTo(SanitizeReturnTo(r.URL.RequestURI())).String()
}
