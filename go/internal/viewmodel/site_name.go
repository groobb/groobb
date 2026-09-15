package viewmodel

import "context"

// siteNameContextKeyはサイトの名前を保存するcontextのキーです。
type siteNameContextKey struct{}

// SetSiteNameは、今描画しているページが属するサイトの名前としてnameを持つctxの
// 複製を返します。1インスタンスはちょうど1つのコミュニティを運営する (ADR 0006) ため、
// 訪問者がいるサイトはそのコミュニティであり、その名前がどのページのタイトルの末尾にも
// 置かれます。
//
// コミュニティを解決するミドルウェアがリクエストごとに呼び、ルーターを通さずページを
// 描画するテストはミドルウェアの代わりにこれを呼びます。
func SetSiteName(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, siteNameContextKey{}, name)
}

// SiteNameFromContextは、今描画しているページが属するサイトの名前を返します。
// contextがそれを持たないとき (コミュニティがまだ作られていないインスタンス、または
// リクエストの外で描画するテンプレート) は空文字列を返します。その場合ページは、末尾が
// 尻切れになったタイトルではなく、自身の名前だけを運びます。
func SiteNameFromContext(ctx context.Context) string {
	name, _ := ctx.Value(siteNameContextKey{}).(string)
	return name
}
