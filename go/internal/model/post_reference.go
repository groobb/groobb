package model

import (
	"regexp"
	"strconv"
	"time"
)

// PostReference records that one post refers to another, extracted from the
// body when the post is saved. Reading it back is how a post learns which later
// posts replied to it.
//
// It is stored rather than derived from the bodies on screen because a screen
// showing only part of a thread would miss every reference made by a post it
// does not carry.
//
// [Ja] PostReference は、ある投稿が別の投稿を参照していることを記録します。値は投稿の
// 保存時に本文から抽出します。これを読み返すことで、投稿は自分に返信した後続の投稿を
// 知ります。
//
// 画面に出ている本文から導くのではなく保存するのは、スレッドの一部だけを表示する画面では、
// そこに載っていない投稿からの参照がすべて欠けるためです。
type PostReference struct {
	ID PostReferenceID

	// PostID is the post that wrote the reference, and ReferencedPostID the post
	// it points at. Both sit in the same thread, because a reply number only
	// means anything within one.
	//
	// [Ja] PostID は参照を書いた投稿、ReferencedPostID はそれが指す投稿です。レス番号は
	// 1 つのスレッドの中でしか意味を持たないため、両者は同じスレッドにあります。
	PostID           PostID
	ReferencedPostID PostID

	CreatedAt time.Time
	UpdatedAt time.Time
}

// postReferencePattern matches a reply reference as it is written in a body:
// the two ASCII greater-than signs the convention uses, followed by the digits
// of a reply number. Only ASCII is matched, so the full-width forms a Japanese
// keyboard produces are text like any other.
//
// [Ja] postReferencePattern は、本文に書かれたままのレス参照に一致します。慣習が使う
// ASCII の不等号 2 つと、それに続くレス番号の数字です。一致するのは ASCII だけであり、
// 日本語入力が生む全角の形は他のテキストと同じく本文の一部です。
var postReferencePattern = regexp.MustCompile(`>>([0-9]+)`)

// PostReferenceSpan locates one reply reference in a body: where the text of it
// sits, and the reply number it names. A body that writes the same number twice
// produces two spans, because each of them is a separate run of text.
//
// [Ja] PostReferenceSpan は本文の中のレス参照 1 つの在り処、すなわちその文字列がどこに
// あるかと、それが名指すレス番号を示します。同じ番号を 2 度書いた本文は 2 つの span を
// 生みます。それぞれが別々のテキストの断片であるためです。
type PostReferenceSpan struct {
	// Start and End are byte offsets into the body the span was read from, so
	// that body[Start:End] is the reference as it was written.
	//
	// [Ja] Start と End は、span を読み取った本文へのバイト単位のオフセットです。
	// body[Start:End] が、書かれたままのその参照になります。
	Start int
	End   int

	// Number is the reply number the reference names. A reference written with
	// leading zeros names the post that one written without them names, so what
	// is carried here is the number rather than its digits.
	//
	// [Ja] Number は参照が名指すレス番号です。先頭に 0 を付けて書かれた参照は、それを
	// 付けずに書かれた参照と同じ投稿を名指すため、ここが運ぶのは数字の並びではなく数です。
	Number int
}

// PostReferenceSpans returns every reply reference a body writes, in the order
// they are written, along with where each one sits.
//
// Rendering a body needs the positions that ReferencedPostNumbers drops: the
// body is shown as it was written, with each reference turned into a link in
// place, so the text around them has to survive the reading. Both functions read
// a body by the one pattern above, so that a >>N stored as a reference is the
// same >>N that renders as a link. Were the two written separately and left to
// drift, a post could hold a reference its own body renders as plain text, or
// render a link to a post that knows nothing of it.
//
// The numbers are what the body claims, not what the thread holds; see
// ReferencedPostNumbers.
//
// [Ja] PostReferenceSpans は、本文が書いたレス参照をすべて、書かれた順に、それぞれの
// 在り処とともに返します。
//
// 本文の描画には、ReferencedPostNumbers が捨てる位置が要ります。本文は書かれたままに
// 表示し、各参照をその場でリンクに変えるため、その周りのテキストが読み取りを生き延び
// なければなりません。2 つの関数はともに上記の 1 つのパターンで本文を読みます。参照として
// 保存される >>N と、リンクとして描画される >>N を同じものにするためです。両者を別々に
// 書いて離れるに任せれば、投稿が自身の本文ではただのテキストとして描画される参照を持ったり、
// そのことを知らない投稿へのリンクを描画したりすることになります。
//
// 返る番号は本文が主張するものであって、スレッドが持つものではありません。
// ReferencedPostNumbers を参照してください。
func PostReferenceSpans(body string) []PostReferenceSpan {
	matches := postReferencePattern.FindAllStringSubmatchIndex(body, -1)
	if len(matches) == 0 {
		return nil
	}

	spans := make([]PostReferenceSpan, 0, len(matches))
	for _, match := range matches {
		// A number no post can carry is dropped here rather than left to the
		// caller: zero is not a reply number, and a run of digits too long for
		// an int is not a number at all.
		//
		// [Ja] どの投稿も持ち得ない番号は、呼び出し元に委ねずここで落とします。0 は
		// レス番号ではなく、int に収まらない長さの数字の並びはそもそも数ではありません。
		number, err := strconv.Atoi(body[match[2]:match[3]])
		if err != nil || number < 1 {
			continue
		}

		spans = append(spans, PostReferenceSpan{Start: match[0], End: match[1], Number: number})
	}

	return spans
}

// ReferencedPostNumbers returns the reply numbers a body refers to, in the order
// they are written and without repeating one the body writes twice: a reference
// is a relation between two posts, so writing it again says nothing new.
//
// The numbers are what the body claims, not what the thread holds. Whether a
// post carries each number is the caller's to resolve, because this function is
// given the body alone: a >>N pointing past the end of a thread is text, and
// stays text on the way in (no row is written for it) and on the way out (it is
// rendered without a link).
//
// [Ja] ReferencedPostNumbers は、本文が参照するレス番号を、書かれた順で、同じ番号を
// 2 度書いた本文でも繰り返さずに返します。参照は 2 つの投稿の間の関係であり、もう一度
// 書いても新たに述べるものが無いためです。
//
// 返るのは本文が主張する番号であって、スレッドが実際に持つ番号ではありません。それぞれの
// 番号の投稿が存在するかの解決は呼び出し元の責務です。本関数が受け取るのは本文だけである
// ためで、スレッドの終端を越える >>N はテキストであり、書き込む側でも (行を作らない)
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
