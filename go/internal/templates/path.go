package templates

import (
	"github.com/a-h/templ"
)

// Path represents a URL path within the application. Centralizing path strings
// here lets templates link to routes without hard-coding literals, so a route
// change is made in one place instead of across every template.
//
// [Ja] Path はアプリケーション内の URL パスを表す型です。パス文字列をここに集約
// することで、テンプレートはリテラルをハードコードせずにルートへリンクでき、ルート
// 変更を各テンプレートではなく 1 箇所で行えます。
type Path string

// String returns the path as a string.
//
// [Ja] String はパスを文字列として返します。
func (p Path) String() string {
	return string(p)
}

// SafeURL returns the path as a templ.SafeURL for use in href / action attributes.
//
// [Ja] SafeURL はパスを href / action 属性で使うための templ.SafeURL として返します。
func (p Path) SafeURL() templ.SafeURL {
	return templ.SafeURL(p)
}

// SignUpPath returns the path to the sign-up form.
//
// [Ja] SignUpPath はサインアップフォームのパスを返します。
func SignUpPath() Path {
	return Path("/sign_up")
}

// SignInPath returns the path to the sign-in form.
//
// [Ja] SignInPath はサインインフォームのパスを返します。
func SignInPath() Path {
	return Path("/sign_in")
}
