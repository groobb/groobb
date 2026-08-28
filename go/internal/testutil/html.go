package testutil

import (
	"strings"
	"testing"
)

// OpeningTag returns the opening tag of the element whose opening tag contains
// marker, so a test can assert about one element's attributes instead of the
// whole document. A page-wide strings.Contains is satisfied by the same
// attribute written anywhere else, which is what lets a class or an ARIA
// attribute be dropped from the element under test without a test noticing.
//
// The marker has to sit inside the opening tag — an id, or an attribute the
// page carries once — because the slice runs from the "<" before it to the
// first ">" after it. A marker in the element's text would run past the tag it
// was meant to name.
//
// [Ja] OpeningTag は marker を開始タグに含む要素の開始タグを返し、テストが文書全体では
// なく 1 つの要素の属性について検証できるようにします。ページ全体に対する
// strings.Contains は、同じ属性がどこか別の場所に書かれていれば満たされてしまいます。
// これが、検証対象の要素からクラスや ARIA 属性が落ちてもテストが気づかない理由です。
//
// marker は開始タグの中に置く必要があります。id か、ページが 1 度だけ持つ属性が該当
// します。切り出しが marker の手前の "<" から直後の ">" までを取るためで、要素のテキスト
// に置いた marker は、名指すはずだったタグを越えて伸びてしまいます。
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

// Element returns the markup from the first occurrence of marker up to the
// first closing after it, so an assertion about one element is not satisfied by
// something written elsewhere on the page.
//
// closing is the literal the caller wants the slice to stop at rather than the
// element's own end tag: the first match wins, so a marker naming an element
// that nests the same tag yields the inner close. That is enough for asserting
// what one region holds, which is what this is for, and a caller that needs the
// whole element picks a closing only its end can produce.
//
// [Ja] Element は marker の最初の出現から、その後の最初の closing までのマークアップを
// 返し、ある要素についての検証がページの別の場所に書かれたもので満たされないように
// します。
//
// closing は要素自身の終了タグではなく、呼び出し側が切り出しを止めたい文字列です。
// 最初に一致したものが採られるため、同じタグを入れ子にする要素を marker が名指した
// 場合は内側の閉じで止まります。ある領域が何を持つかを検証する用途にはこれで足り、
// それが本ヘルパーの目的です。要素の全体が要る呼び出し側は、その終わりだけが生む
// closing を選びます。
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
