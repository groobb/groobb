package templates

import (
	"context"
	"net/http"
)

// CurrentPathMiddleware stores the request path in the context so a template can
// tell whether a link it renders points at the page being rendered. A navigation
// entry uses it to mark itself with aria-current="page".
//
// It lives beside the templates rather than in internal/middleware because the
// templates are what read the value, the same way i18n.Middleware feeds
// templates.Locale. A template that marks a current link then reaches for the
// one presentation helper package it already imports.
//
// [Ja] CurrentPathMiddleware はリクエストパスを context に保存し、テンプレートが自分の
// 描画するリンクが今描画しているページを指すかどうかを判別できるようにします。
// ナビゲーションの項目はこれを使って自身に aria-current="page" を付けます。
//
// internal/middleware ではなくテンプレートの側に置くのは、値を読むのがテンプレートだから
// です。i18n.Middleware が templates.Locale に値を供給するのと同じ形であり、現在ページの
// リンクに印を付けるテンプレートは、既に import しているプレゼンテーション用ヘルパー
// パッケージ 1 つを参照すれば済みます。
func CurrentPathMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(SetCurrentPath(r.Context(), r.URL.Path)))
	})
}

// currentPathContextKey is the context key the current request path is stored
// under.
//
// [Ja] currentPathContextKey は現在のリクエストパスを保存する context のキーです。
type currentPathContextKey struct{}

// SetCurrentPath returns a copy of ctx carrying path as the path of the page
// being rendered. CurrentPathMiddleware calls it per request; a test that
// renders a template without going through the router calls it in place of the
// middleware.
//
// [Ja] SetCurrentPath は path を今描画しているページのパスとして持つ ctx の複製を
// 返します。CurrentPathMiddleware がリクエストごとに呼び、ルーターを通さずテンプレートを
// 描画するテストはミドルウェアの代わりにこれを呼びます。
func SetCurrentPath(ctx context.Context, path string) context.Context {
	return context.WithValue(ctx, currentPathContextKey{}, path)
}

// IsCurrentPath reports whether path is the path of the page being rendered.
// The two are compared literally: the stored value is r.URL.Path, which carries
// neither query nor fragment, and templates link to the Path constants of this
// package. A link built with a query string would need more than this
// comparison.
//
// It reports false when the context carries no path, so a template rendered
// outside a request (an email) marks nothing as the current page.
//
// [Ja] IsCurrentPath は path が今描画しているページのパスかどうかを返します。両者は
// 文字列の一致で比較します。保存される値はクエリもフラグメントも含まない r.URL.Path で
// あり、テンプレートがリンクするのは本パッケージの Path 定数だからです。クエリ文字列を
// 付けて組み立てたリンクには、この比較では足りません。
//
// context がパスを持たないときは false を返すため、リクエストの外で描画するテンプレート
// (メールなど) は何も現在ページとして印を付けません。
func IsCurrentPath(ctx context.Context, path string) bool {
	current, ok := ctx.Value(currentPathContextKey{}).(string)
	return ok && current == path
}
