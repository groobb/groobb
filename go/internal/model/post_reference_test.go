package model_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/model"
)

// TestReferencedPostNumbersは、本文が何を参照していると読まれるのかを検証します。
// 慣習の不等号2つで書かれた番号を、それぞれ1度ずつ、現れた順に読み、それ以外は読み
// ません。
func TestReferencedPostNumbers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want []int
	}{
		{name: "参照が無い", body: "ふつうの本文です。", want: nil},
		{name: "参照が1つ", body: ">>1そのとおりです。", want: []int{1}},
		{
			name: "書かれた順に並ぶ複数の参照",
			body: ">>3と >>1の話です。",
			want: []int{3, 1},
		},
		{
			// 同じ番号を2度書いても、それが述べる2つの投稿の間の関係は1つで
			// あり、それを保存する列の組は一意です。
			name: "同じ番号が2度",
			body: ">>2 >>2二度書いても参照は1つです。",
			want: []int{2},
		},
		{
			// 参照は行頭と同じくらい文の途中にも書かれるため、この慣習は行頭に
			// 固定されていません。
			name: "行の途中",
			body: "さっきの話ですが >>4これでどうでしょう。",
			want: []int{4},
		},
		{name: "2行目", body: "一行目\n>>5二行目", want: []int{5}},
		{name: "大なり記号が1つだけ", body: "> 1は引用に見える書き方です。", want: nil},
		{name: "数字が無い", body: ">> 番号のない参照。", want: nil},
		{
			// 0はレス番号ではありません。スレッドの最初の投稿は1です。
			name: "ゼロ",
			body: ">>0は存在しません。",
			want: nil,
		},
		{name: "先頭にゼロが付く", body: ">>007と書いても7です。", want: []int{7}},
		{
			// 数として扱えない長さの数字の並びは投稿を名指しできず、それによって
			// 本文が読めなくなってもなりません。
			name: "番号として長すぎる数字",
			body: ">>" + strings.Repeat("9", 40) + " と >>8",
			want: []int{8},
		},
		{
			// スレッドの終端を越える番号はテキストです。スレッドがどの番号を持つ
			// のかは、本文の知るところではありません。
			name: "どのスレッドも持つ必要のない番号",
			body: ">>999999まだ誰も書いていません。",
			want: []int{999999},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := model.ReferencedPostNumbers(tt.body)

			if !slices.Equal(got, tt.want) {
				t.Errorf("ReferencedPostNumbers(%q) = %v、期待値 = %v", tt.body, got, tt.want)
			}
		})
	}
}

// TestPostReferenceSpansは、本文が参照をどこに書いていると読まれるのかを検証
// します。それは描画する側が必要とし、ReferencedPostNumbersが捨てるものです。読み取り
// そのものはTestReferencedPostNumbersが扱うため、ここで確かめるのは位置と、番号の側
// では畳まれる繰り返しです。
func TestPostReferenceSpans(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want []model.PostReferenceSpan
	}{
		{name: "参照が無い", body: "ふつうの本文です。", want: nil},
		{
			name: "本文の先頭",
			body: ">>1そのとおりです。",
			want: []model.PostReferenceSpan{{Start: 0, End: 3, Number: 1}},
		},
		{
			// オフセットはバイト単位であるため、参照より前のテキストは、読まれる
			// 形ではなく保存される形で測られます。
			name: "日本語のテキストの後ろ",
			body: "さっきの話ですが >>4これでどうでしょう。",
			want: []model.PostReferenceSpan{{Start: 25, End: 28, Number: 4}},
		},
		{
			// 1つの投稿への2度の言及は2つのテキストの断片であり、それぞれが
			// 書かれた場所で描画されます。1つになるのは、それらが述べる関係のほうで
			// あり、ReferencedPostNumbersが報告するのはそちらです。
			name: "同じ番号が2度",
			body: ">>2 >>2",
			want: []model.PostReferenceSpan{
				{Start: 0, End: 3, Number: 2},
				{Start: 4, End: 7, Number: 2},
			},
		},
		{
			// spanは書かれたままの数字を覆い、番号のほうは詰め物を落とします。
			// これによりテキストは >>007として描画され、>>7と同じ場所へ繋がります。
			name: "先頭にゼロが付く",
			body: ">>007",
			want: []model.PostReferenceSpan{{Start: 0, End: 5, Number: 7}},
		},
		{
			name: "どのスレッドも持つ必要のない番号",
			body: ">>999999",
			want: []model.PostReferenceSpan{{Start: 0, End: 8, Number: 999999}},
		},
		{name: "ゼロ", body: ">>0は存在しません。", want: nil},
		{
			name: "番号として長すぎる数字",
			body: ">>" + strings.Repeat("9", 40) + " と >>8",
			want: []model.PostReferenceSpan{{Start: 47, End: 50, Number: 8}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := model.PostReferenceSpans(tt.body)

			if !slices.Equal(got, tt.want) {
				t.Errorf("PostReferenceSpans(%q) = %v、期待値 = %v", tt.body, got, tt.want)
			}

			for _, span := range got {
				if tt.body[span.Start:span.End] == "" {
					t.Errorf("PostReferenceSpans(%q)が空のspan %v を返した", tt.body, span)
				}
			}
		})
	}
}
