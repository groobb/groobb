package validator

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// isWellFormedText reports whether s meets the application's text input rules:
// valid UTF-8 with no NUL.
//
// Percent-decoded form values can contain arbitrary bytes, so validators
// enforce these rules before accepting titles or post bodies.
//
// [Ja] isWellFormedText は s がアプリケーションのテキスト入力規則 (妥当なUTF-8で
// NULを含まないこと) を満たすかどうかを返します。
//
// パーセントデコードされたフォームの値には任意のバイト列が含まれうるため、
// タイトルや投稿本文を受け付ける前にバリデーターでこの規則を適用します。
func isWellFormedText(s string) bool {
	return utf8.ValidString(s) && !strings.ContainsRune(s, 0)
}

// brailleBlank is the braille cell with no dots raised. It belongs to neither
// of the property tables hasVisibleChar consults, so it is named here.
//
// [Ja] brailleBlank は点を1つも持たない点字のマスです。hasVisibleChar が参照する
// どちらのプロパティ表にも属さないため、ここで名指しします。
const brailleBlank = '\u2800'

// hasVisibleChar reports whether s contains a character that draws something:
// a graphic character that is neither whitespace, nor default-ignorable, nor
// brailleBlank.
//
// Some graphic characters, including variation selectors and brailleBlank,
// draw nothing on their own. They must not satisfy the required check for a
// title or body, but are kept in accepted text to preserve its rendering:
// brailleBlank separates words within braille text, as a space does elsewhere.
//
// [Ja] hasVisibleChar は、何かを描く文字が s に含まれるかを返します。空白でも、既定で
// 無視される文字でも、brailleBlank でもない図形文字です。
//
// 異体字セレクターや brailleBlank など、一部の図形文字は単独では何も描きません。それ
// だけでタイトルや本文の必須チェックを通さず、受理するテキストでは描画を保つためにその
// まま保持します。brailleBlank は点字テキストの中では、他所での空白と同じく語を区切り
// ます。
func hasVisibleChar(s string) bool {
	for _, r := range s {
		if r == brailleBlank || unicode.In(r, unicode.Variation_Selector, unicode.Other_Default_Ignorable_Code_Point) {
			continue
		}
		if unicode.IsGraphic(r) && !unicode.IsSpace(r) {
			return true
		}
	}
	return false
}
