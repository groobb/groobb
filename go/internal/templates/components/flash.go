package components

import "github.com/groobb/groobb/go/internal/session"

// flashCategoryはFlashTypeを、toastの種別ごとのスタイルを駆動するBasecoatの
// data-category値へ対応させます。属性内の条件分岐ではなく素のGoヘルパーにしているのは、
// templが要素の属性リスト内のelse-if連鎖をサポートせず黙って壊れたコードを生成するためです。
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
