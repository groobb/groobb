package templates

import (
	"context"
	"net/http"
)

// CurrentPathMiddlewareはリクエストパスをcontextに保存し、テンプレートが自分の
// 描画するリンクが今描画しているページを指すかどうかを判別できるようにします。
// ナビゲーションの項目はこれを使って自身にaria-current="page" を付けます。
//
// internal/middlewareではなくテンプレートの側に置くのは、値を読むのがテンプレートだから
// です。i18n.Middlewareがtemplates.Localeに値を供給するのと同じ形であり、現在ページの
// リンクに印を付けるテンプレートは、既にimportしているプレゼンテーション用ヘルパー
// パッケージ1つを参照すれば済みます。
func CurrentPathMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(SetCurrentPath(r.Context(), r.URL.Path)))
	})
}

// currentPathContextKeyは現在のリクエストパスを保存するcontextのキーです。
type currentPathContextKey struct{}

// SetCurrentPathはpathを今描画しているページのパスとして持つctxの複製を
// 返します。CurrentPathMiddlewareがリクエストごとに呼び、ルーターを通さずテンプレートを
// 描画するテストはミドルウェアの代わりにこれを呼びます。
func SetCurrentPath(ctx context.Context, path string) context.Context {
	return context.WithValue(ctx, currentPathContextKey{}, path)
}

// IsCurrentPathはpathが今描画しているページのパスかどうかを返します。両者は
// 文字列の一致で比較します。保存される値はクエリもフラグメントも含まないr.URL.Pathで
// あり、テンプレートがリンクするのは本パッケージのPath定数だからです。クエリ文字列を
// 付けて組み立てたリンクには、この比較では足りません。
//
// contextがパスを持たないときはfalseを返すため、リクエストの外で描画するテンプレート
// (メールなど) は何も現在ページとして印を付けません。
func IsCurrentPath(ctx context.Context, path string) bool {
	current, ok := ctx.Value(currentPathContextKey{}).(string)
	return ok && current == path
}
