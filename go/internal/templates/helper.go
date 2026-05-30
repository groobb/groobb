// Package templates provides helper functions called from templ templates.
// [Ja] templates パッケージは templ テンプレートから呼び出されるヘルパー関数を提供します。
package templates

import (
	"context"

	"github.com/groobb/groobb/go/internal/i18n"
)

// T translates messageID using the locale stored in ctx. It is a thin wrapper
// over i18n.T so templates depend on the templates package rather than i18n.
//
// [Ja] T は ctx に格納されたロケールで messageID を翻訳する。テンプレートが i18n
// ではなく templates パッケージに依存するようにするための i18n.T の薄いラッパー。
func T(ctx context.Context, messageID string, data ...map[string]any) string {
	return i18n.T(ctx, messageID, data...)
}

// Locale returns the current locale stored in ctx, used for the html lang
// attribute.
//
// [Ja] Locale は ctx に格納された現在のロケールを返す。html の lang 属性に用いる。
func Locale(ctx context.Context) string {
	return i18n.GetLocale(ctx)
}
