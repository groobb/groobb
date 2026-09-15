package validator

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// isWellFormedTextはsがアプリケーションのテキスト入力規則 (妥当なUTF-8で
// NULを含まないこと) を満たすかどうかを返します。
//
// パーセントデコードされたフォームの値には任意のバイト列が含まれうるため、
// タイトルや投稿本文を受け付ける前にバリデーターでこの規則を適用します。
func isWellFormedText(s string) bool {
	return utf8.ValidString(s) && !strings.ContainsRune(s, 0)
}

// brailleBlankは点を1つも持たない点字のマスです。hasVisibleCharが参照する
// どちらのプロパティ表にも属さないため、ここで名指しします。
const brailleBlank = '\u2800'

// hasVisibleCharは、何かを描く文字がsに含まれるかを返します。空白でも、既定で
// 無視される文字でも、brailleBlankでもない図形文字です。
//
// 異体字セレクターやbrailleBlankなど、一部の図形文字は単独では何も描きません。それ
// だけでタイトルや本文の必須チェックを通さず、受理するテキストでは描画を保つためにその
// まま保持します。brailleBlankは点字テキストの中では、他所での空白と同じく語を区切り
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
