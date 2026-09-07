package model

import "strings"

// NormalizeLineBreaks rewrites every CRLF and every lone CR in s to LF.
//
// A form is submitted with CRLF line endings, so what was typed and what
// arrives differ by bytes that carry no meaning of their own. The normalized
// value is the one that is checked, stored and drawn back, so the length a body
// is refused at does not depend on how the browser wrote its line endings, and
// what comes back out of the database is what was counted.
//
// [Ja] NormalizeLineBreaks は s の中のCRLFと単独のCRをすべてLFに書き換えます。
//
// フォームはCRLFの改行で送られるため、打たれたものと届いたものは、それ自体では意味を
// 持たないバイトの分だけ食い違います。検証・保存・再描画に使うのは正規化した値であり、
// 本文が拒否される長さがブラウザの書いた改行に左右されず、データベースから出てくるものが
// 数えられたものと一致します。
func NormalizeLineBreaks(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
}
