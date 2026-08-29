// Package templates provides helper functions called from templ templates.
//
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

// Locale returns the current locale stored in ctx as the string form, used for
// the html lang attribute. The attribute takes a BCP 47 tag, which is what the
// underlying model.Locale spells, so the value goes into the markup as it is.
//
// [Ja] Locale は ctx に格納された現在のロケールを文字列の形で返す。html の lang 属性に
// 用いる。この属性が取るのは BCP 47 の言語タグであり、それは背後の model.Locale が
// 綴っているものそのものであるため、値はそのままマークアップへ入る。
func Locale(ctx context.Context) string {
	return string(i18n.GetLocale(ctx))
}
