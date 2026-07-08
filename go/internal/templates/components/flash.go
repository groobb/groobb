package components

import "github.com/groobb/groobb/go/internal/session"

// flashCategory maps a FlashType to the Basecoat data-category value that drives
// the toast's per-category styling. It is a plain Go helper (not an in-attribute
// conditional) because templ does not support else-if chains inside an element's
// attribute list and silently miscompiles them.
//
// [Ja] flashCategory は FlashType を、toast の種別ごとのスタイルを駆動する Basecoat の
// data-category 値へ対応させます。属性内の条件分岐ではなく素の Go ヘルパーにしているのは、
// templ が要素の属性リスト内の else-if 連鎖をサポートせず黙って壊れたコードを生成するためです。
func flashCategory(t session.FlashType) string {
	switch t {
	case session.FlashSuccess:
		return "success"
	case session.FlashError:
		return "error"
	case session.FlashWarning:
		return "warning"
	default:
		return "info"
	}
}
