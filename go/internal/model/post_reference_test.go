package model_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/model"
)

// TestReferencedPostNumbers verifies what a body is read as referring to: the
// numbers it writes with the convention's two greater-than signs, each of them
// once, in the order they appear, and nothing else.
//
// [Ja] TestReferencedPostNumbers は、本文が何を参照していると読まれるのかを検証します。
// 慣習の不等号 2 つで書かれた番号を、それぞれ 1 度ずつ、現れた順に読み、それ以外は読み
// ません。
func TestReferencedPostNumbers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want []int
	}{
		{name: "no reference", body: "ふつうの本文です。", want: nil},
		{name: "one reference", body: ">>1 そのとおりです。", want: []int{1}},
		{
			name: "several references in the order they are written",
			body: ">>3 と >>1 の話です。",
			want: []int{3, 1},
		},
		{
			// The same number written twice describes one relation between two
			// posts, and the column pair it is stored as is unique.
			//
			// [Ja] 同じ番号を 2 度書いても、それが述べる 2 つの投稿の間の関係は 1 つで
			// あり、それを保存する列の組は一意です。
			name: "the same number twice",
			body: ">>2 >>2 二度書いても参照は 1 つです。",
			want: []int{2},
		},
		{
			// A reference is written mid-sentence as often as at the start of
			// one, so the convention is not anchored to the beginning of a line.
			//
			// [Ja] 参照は行頭と同じくらい文の途中にも書かれるため、この慣習は行頭に
			// 固定されていません。
			name: "in the middle of a line",
			body: "さっきの話ですが >>4 これでどうでしょう。",
			want: []int{4},
		},
		{name: "on the second line", body: "一行目\n>>5 二行目", want: []int{5}},
		{name: "a single greater-than sign", body: "> 1 は引用に見える書き方です。", want: nil},
		{name: "no digits", body: ">> 番号のない参照。", want: nil},
		{
			// Zero is not a reply number: the first post of a thread is 1.
			//
			// [Ja] 0 はレス番号ではありません。スレッドの最初の投稿は 1 です。
			name: "zero",
			body: ">>0 は存在しません。",
			want: nil,
		},
		{name: "leading zeros", body: ">>007 と書いても 7 です。", want: []int{7}},
		{
			// A run of digits too long to be a number cannot name a post, and it
			// must not stop the body from being read.
			//
			// [Ja] 数として扱えない長さの数字の並びは投稿を名指しできず、それによって
			// 本文が読めなくなってもなりません。
			name: "a number too long to be one",
			body: ">>" + strings.Repeat("9", 40) + " と >>8",
			want: []int{8},
		},
		{
			// A number past the end of a thread is text. Which numbers a thread
			// carries is not something a body knows.
			//
			// [Ja] スレッドの終端を越える番号はテキストです。スレッドがどの番号を持つ
			// のかは、本文の知るところではありません。
			name: "a number no thread need carry",
			body: ">>999999 まだ誰も書いていません。",
			want: []int{999999},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := model.ReferencedPostNumbers(tt.body)

			if !slices.Equal(got, tt.want) {
				t.Errorf("ReferencedPostNumbers(%q) = %v, want %v", tt.body, got, tt.want)
			}
		})
	}
}

// TestPostReferenceSpans verifies where a body is read as writing its
// references, which is what the rendering side needs and what
// ReferencedPostNumbers drops. The reading itself is the same one
// TestReferencedPostNumbers covers, so what is checked here is the positions,
// and the repetition that the numbers collapse.
//
// [Ja] TestPostReferenceSpans は、本文が参照をどこに書いていると読まれるのかを検証
// します。それは描画する側が必要とし、ReferencedPostNumbers が捨てるものです。読み取り
// そのものは TestReferencedPostNumbers が扱うため、ここで確かめるのは位置と、番号の側
// では畳まれる繰り返しです。
func TestPostReferenceSpans(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want []model.PostReferenceSpan
	}{
		{name: "no reference", body: "ふつうの本文です。", want: nil},
		{
			name: "at the start of the body",
			body: ">>1 そのとおりです。",
			want: []model.PostReferenceSpan{{Start: 0, End: 3, Number: 1}},
		},
		{
			// The offsets are byte offsets, so text before a reference is
			// measured as it is stored rather than as it is read.
			//
			// [Ja] オフセットはバイト単位であるため、参照より前のテキストは、読まれる
			// 形ではなく保存される形で測られます。
			name: "after Japanese text",
			body: "さっきの話ですが >>4 これでどうでしょう。",
			want: []model.PostReferenceSpan{{Start: 25, End: 28, Number: 4}},
		},
		{
			// Two mentions of one post are two runs of text, each of which is
			// rendered where it was written. Only the relation they describe is
			// the one thing ReferencedPostNumbers reports.
			//
			// [Ja] 1 つの投稿への 2 度の言及は 2 つのテキストの断片であり、それぞれが
			// 書かれた場所で描画されます。1 つになるのは、それらが述べる関係のほうで
			// あり、ReferencedPostNumbers が報告するのはそちらです。
			name: "the same number twice",
			body: ">>2 >>2",
			want: []model.PostReferenceSpan{
				{Start: 0, End: 3, Number: 2},
				{Start: 4, End: 7, Number: 2},
			},
		},
		{
			// The span covers the digits as written while the number drops the
			// padding, so the text renders as >>007 and leads where >>7 leads.
			//
			// [Ja] span は書かれたままの数字を覆い、番号のほうは詰め物を落とします。
			// これによりテキストは >>007 として描画され、>>7 と同じ場所へ繋がります。
			name: "leading zeros",
			body: ">>007",
			want: []model.PostReferenceSpan{{Start: 0, End: 5, Number: 7}},
		},
		{
			name: "a number no thread need carry",
			body: ">>999999",
			want: []model.PostReferenceSpan{{Start: 0, End: 8, Number: 999999}},
		},
		{name: "zero", body: ">>0 は存在しません。", want: nil},
		{
			name: "a number too long to be one",
			body: ">>" + strings.Repeat("9", 40) + " と >>8",
			want: []model.PostReferenceSpan{{Start: 47, End: 50, Number: 8}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := model.PostReferenceSpans(tt.body)

			if !slices.Equal(got, tt.want) {
				t.Errorf("PostReferenceSpans(%q) = %v, want %v", tt.body, got, tt.want)
			}

			for _, span := range got {
				if tt.body[span.Start:span.End] == "" {
					t.Errorf("PostReferenceSpans(%q) returned the empty span %v", tt.body, span)
				}
			}
		})
	}
}
