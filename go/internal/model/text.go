package model

import "strings"

// NormalizeLineBreaksはsの中のCRLFと単独のCRをすべてLFに書き換えます。
//
// フォームはCRLFの改行で送られるため、打たれたものと届いたものは、それ自体では意味を
// 持たないバイトの分だけ食い違います。検証・保存・再描画に使うのは正規化した値であり、
// 本文が拒否される長さがブラウザの書いた改行に左右されず、データベースから出てくるものが
// 数えられたものと一致します。
func NormalizeLineBreaks(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
}
