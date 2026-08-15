package middleware

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/groobb/groobb/go/internal/templates"
)

// SanitizeReturnTo returns a normalized request URI when raw names a destination
// this application may safely redirect to, and "" otherwise. Only a same-origin
// relative path is accepted: a single leading "/" followed by neither "/" nor
// "\". The value reaches us from the outside (a query parameter, then a form
// field), so a value that names another origin ("//evil.example.com",
// "https://evil.example.com") or that a browser would read as one
// ("/\evil.example.com") is dropped rather than redirected to, which is what
// keeps the sign-in flow from becoming an open redirect. The accepted path is
// returned re-encoded and without its fragment. RequireAuth captures an HTTP
// request URI, which never contains the browser's top-level fragment; externally
// supplied query or form values are normalized to the same request-URI contract.
//
// [Ja] SanitizeReturnTo は raw が本アプリケーションから安全にリダイレクトできる遷移先を
// 指しているときに正規化した request URI を返し、それ以外では "" を返す。受け付けるのは
// 同一オリジンの相対パスだけで、先頭が "/" 1 つ、その次が "/" でも "\" でもないものに
// 限る。この値は外部 (クエリパラメータ、続いてフォームフィールド) から渡ってくるため、
// 別オリジンを指す値 ("//evil.example.com"、"https://evil.example.com") やブラウザが
// 別オリジンとして解釈する値 ("/\evil.example.com") はリダイレクトせずに破棄する。
// これがサインインフローをオープンリダイレクトにしないための要である。受け付けたパスは
// 再エンコードし、フラグメントを落として返す。RequireAuth が取得する HTTP request URI
// にはブラウザのトップレベルフラグメントが含まれないため、外部から query / form 値として
// 渡された値も同じ request URI の契約へ正規化する。
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

// signInPathWithReturnTo returns the sign-in path for the anonymous request r,
// carrying the requested URL so the visitor lands back on it once signed in. Only
// GET and HEAD get one: replaying the target of an unsafe method as a plain GET
// afterwards drops the visitor on a page they never asked for, so those fall back
// to the bare sign-in path.
//
// [Ja] signInPathWithReturnTo は匿名リクエスト r に対するサインインのパスを、サインイン後に
// 元の URL へ戻れるようリクエスト先を載せて返す。載せるのは GET と HEAD のときだけとする。
// 安全でないメソッドの宛先を後から素の GET でなぞると、訪問者が求めていないページに着地させて
// しまうため、それらは素のサインインパスにフォールバックする。
func signInPathWithReturnTo(r *http.Request) string {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return templates.SignInPath().String()
	}

	return templates.SignInPath().WithReturnTo(SanitizeReturnTo(r.URL.RequestURI())).String()
}
