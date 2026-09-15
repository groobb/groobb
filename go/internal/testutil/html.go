package testutil

import (
	"strings"
	"testing"
)

// OpeningTagはmarkerを開始タグに含む要素の開始タグを返し、テストが文書全体では
// なく1つの要素の属性について検証できるようにします。ページ全体に対する
// strings.Containsは、同じ属性がどこか別の場所に書かれていれば満たされてしまいます。
// これが、検証対象の要素からクラスやARIA属性が落ちてもテストが気づかない理由です。
//
// markerは開始タグの中に置く必要があります。idか、ページが1度だけ持つ属性が該当
// します。切り出しがmarkerの手前の "<" から直後の ">" までを取るためで、要素のテキスト
// に置いたmarkerは、名指すはずだったタグを越えて伸びてしまいます。
func OpeningTag(t *testing.T, body, marker string) string {
	t.Helper()

	at := strings.Index(body, marker)
	if at < 0 {
		t.Fatalf("レスポンスボディに %q が含まれていない", marker)
	}
	start := strings.LastIndex(body[:at], "<")
	end := strings.Index(body[at:], ">")
	if start < 0 || end < 0 {
		t.Fatalf("%q を含む開始タグを取り出せない", marker)
	}
	return body[start : at+end+1]
}

// Elementはmarkerの最初の出現から、その後の最初のclosingまでのマークアップを
// 返し、ある要素についての検証がページの別の場所に書かれたもので満たされないように
// します。
//
// closingは要素自身の終了タグではなく、呼び出し側が切り出しを止めたい文字列です。
// 最初に一致したものが採られるため、同じタグを入れ子にする要素をmarkerが名指した
// 場合は内側の閉じで止まります。ある領域が何を持つかを検証する用途にはこれで足り、
// それが本ヘルパーの目的です。要素の全体が要る呼び出し側は、その終わりだけが生む
// closingを選びます。
func Element(t *testing.T, body, marker, closing string) string {
	t.Helper()

	at := strings.Index(body, marker)
	if at < 0 {
		t.Fatalf("レスポンスボディに %q が含まれていない", marker)
	}
	rest := body[at:]
	end := strings.Index(rest, closing)
	if end < 0 {
		t.Fatalf("%q を含む要素の終わり (%q) が見つからない", marker, closing)
	}
	return rest[:end]
}
