package model

import (
	"regexp"
	"strconv"
	"time"
)

// PostReferenceは、ある投稿が別の投稿を参照していることを記録します。値は投稿の
// 保存時に本文から抽出します。これを読み返すことで、投稿は自分に返信した後続の投稿を
// 知ります。
//
// 画面に出ている本文から導くのではなく保存するのは、スレッドの一部だけを表示する画面では、
// そこに載っていない投稿からの参照がすべて欠けるためです。
type PostReference struct {
	ID PostReferenceID

	// PostIDは参照を書いた投稿、ReferencedPostIDはそれが指す投稿です。レス番号は
	// 1つのスレッドの中でしか意味を持たないため、両者は同じスレッドにあります。
	PostID           PostID
	ReferencedPostID PostID

	CreatedAt time.Time
	UpdatedAt time.Time
}

// postReferencePatternは、本文に書かれたままのレス参照に一致します。慣習が使う
// ASCIIの不等号2つと、それに続くレス番号の数字です。一致するのはASCIIだけであり、
// 日本語入力が生む全角の形は他のテキストと同じく本文の一部です。
var postReferencePattern = regexp.MustCompile(`>>([0-9]+)`)

// PostReferenceSpanは本文の中のレス参照1つの在り処、すなわちその文字列がどこに
// あるかと、それが名指すレス番号を示します。同じ番号を2度書いた本文は2つのspanを
// 生みます。それぞれが別々のテキストの断片であるためです。
type PostReferenceSpan struct {
	// StartとEndは、spanを読み取った本文へのバイト単位のオフセットです。
	// body[Start:End] が、書かれたままのその参照になります。
	Start int
	End   int

	// Numberは参照が名指すレス番号です。先頭に0を付けて書かれた参照は、それを
	// 付けずに書かれた参照と同じ投稿を名指すため、ここが運ぶのは数字の並びではなく数です。
	Number int
}

// PostReferenceSpansは、本文が書いたレス参照をすべて、書かれた順に、それぞれの
// 在り処とともに返します。
//
// 本文の描画には、ReferencedPostNumbersが捨てる位置が要ります。本文は書かれたままに
// 表示し、各参照をその場でリンクに変えるため、その周りのテキストが読み取りを生き延び
// なければなりません。2つの関数はともに上記の1つのパターンで本文を読みます。参照として
// 保存される >>Nと、リンクとして描画される >>Nを同じものにするためです。両者を別々に
// 書いて離れるに任せれば、投稿が自身の本文ではただのテキストとして描画される参照を持ったり、
// そのことを知らない投稿へのリンクを描画したりすることになります。
//
// 返る番号は本文が主張するものであって、スレッドが持つものではありません。
// ReferencedPostNumbersを参照してください。
func PostReferenceSpans(body string) []PostReferenceSpan {
	matches := postReferencePattern.FindAllStringSubmatchIndex(body, -1)
	if len(matches) == 0 {
		return nil
	}

	spans := make([]PostReferenceSpan, 0, len(matches))
	for _, match := range matches {
		// どの投稿も持ち得ない番号は、呼び出し元に委ねずここで落とします。0は
		// レス番号ではなく、intに収まらない長さの数字の並びはそもそも数ではありません。
		number, err := strconv.Atoi(body[match[2]:match[3]])
		if err != nil || number < 1 {
			continue
		}

		spans = append(spans, PostReferenceSpan{Start: match[0], End: match[1], Number: number})
	}

	return spans
}

// ReferencedPostNumbersは、本文が参照するレス番号を、書かれた順で、同じ番号を
// 2度書いた本文でも繰り返さずに返します。参照は2つの投稿の間の関係であり、もう一度
// 書いても新たに述べるものが無いためです。
//
// 返るのは本文が主張する番号であって、スレッドが実際に持つ番号ではありません。それぞれの
// 番号の投稿が存在するかの解決は呼び出し元の責務です。本関数が受け取るのは本文だけである
// ためで、スレッドの終端を越える >>Nはテキストであり、書き込む側でも (行を作らない)
// 描画する側でも (リンクにしない) テキストのままです。
func ReferencedPostNumbers(body string) []int {
	spans := PostReferenceSpans(body)
	if len(spans) == 0 {
		return nil
	}

	numbers := make([]int, 0, len(spans))
	seen := make(map[int]bool, len(spans))
	for _, span := range spans {
		if seen[span.Number] {
			continue
		}

		seen[span.Number] = true
		numbers = append(numbers, span.Number)
	}

	return numbers
}
