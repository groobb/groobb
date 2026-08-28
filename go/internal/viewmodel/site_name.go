package viewmodel

import "context"

// siteNameContextKey is the context key the name of the site is stored under.
//
// [Ja] siteNameContextKey はサイトの名前を保存する context のキーです。
type siteNameContextKey struct{}

// SetSiteName returns a copy of ctx carrying name as the name of the site the
// page being rendered belongs to. An instance serves exactly one community
// (ADR 0006), so the site a visitor is on is that community, and its name is
// what every page's title ends with.
//
// The middleware that resolves the community calls it per request; a test that
// renders a page without going through the router calls it in place of the
// middleware.
//
// [Ja] SetSiteName は、今描画しているページが属するサイトの名前として name を持つ ctx の
// 複製を返します。1 インスタンスはちょうど 1 つのコミュニティを運営する (ADR 0006) ため、
// 訪問者がいるサイトはそのコミュニティであり、その名前がどのページのタイトルの末尾にも
// 置かれます。
//
// コミュニティを解決するミドルウェアがリクエストごとに呼び、ルーターを通さずページを
// 描画するテストはミドルウェアの代わりにこれを呼びます。
func SetSiteName(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, siteNameContextKey{}, name)
}

// SiteNameFromContext returns the name of the site the page being rendered
// belongs to, or an empty string when the context carries none: an instance
// whose community has not been created yet, or a template rendered outside a
// request. A page then carries its own name alone rather than a title trailing
// off into nothing.
//
// [Ja] SiteNameFromContext は、今描画しているページが属するサイトの名前を返します。
// context がそれを持たないとき (コミュニティがまだ作られていないインスタンス、または
// リクエストの外で描画するテンプレート) は空文字列を返します。その場合ページは、末尾が
// 尻切れになったタイトルではなく、自身の名前だけを運びます。
func SiteNameFromContext(ctx context.Context) string {
	name, _ := ctx.Value(siteNameContextKey{}).(string)
	return name
}
