// templatesパッケージはtemplテンプレートから呼び出されるヘルパー関数を提供します。
package templates

import (
	"context"

	"github.com/groobb/groobb/go/internal/i18n"
)

// Tはctxに格納されたロケールでmessageIDを翻訳する。テンプレートがi18n
// ではなくtemplatesパッケージに依存するようにするためのi18n.Tの薄いラッパー。
func T(ctx context.Context, messageID string, data ...map[string]any) string {
	return i18n.T(ctx, messageID, data...)
}

// Localeはctxに格納された現在のロケールを文字列の形で返す。htmlのlang属性に
// 用いる。この属性が取るのはBCP 47の言語タグであり、それは背後のmodel.Localeが
// 綴っているものそのものであるため、値はそのままマークアップへ入る。
func Locale(ctx context.Context) string {
	return string(i18n.GetLocale(ctx))
}
