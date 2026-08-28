package viewmodel_test

import (
	"slices"
	"testing"

	"github.com/groobb/groobb/go/internal/viewmodel"
)

// postBodyText, postBodyReference and postBodyURL build the tokens a case
// expects, so that the tables below read as the body they describe rather than
// as struct literals.
//
// [Ja] postBodyText・postBodyReference・postBodyURL は、あるケースが期待するトークンを
// 組み立てます。下記の表が構造体リテラルの並びではなく、それが述べる本文として読める
// ようにするためです。
func postBodyText(s string) viewmodel.PostBodyToken {
	return viewmodel.PostBodyToken{Kind: viewmodel.PostBodyText, Text: s}
}

func postBodyReference(s string, number int) viewmodel.PostBodyToken {
	return viewmodel.PostBodyToken{Kind: viewmodel.PostBodyPostReference, Text: s, Number: number}
}

func postBodyURL(s string) viewmodel.PostBodyToken {
	return viewmodel.PostBodyToken{Kind: viewmodel.PostBodyURL, Text: s}
}

// thread is the set of reply numbers the cases below are read against: a thread
// holding the first three posts, which is enough for a body to name one that is
// there and one that is not.
//
// [Ja] thread は、下記のケースが照らされるレス番号の集合です。最初の 3 つの投稿を持つ
// スレッドであり、本文がそこにある番号と無い番号の両方を名指すのに足ります。
var thread = map[int]bool{1: true, 2: true, 3: true}

// TestNewPostBody verifies how a body is split for rendering: what becomes a
// link, what stays text, and that the pieces put back together are the body
// again.
//
// [Ja] TestNewPostBody は、描画のために本文がどう分解されるのかを検証します。何がリンク
// になり、何がテキストのままであるか、そして断片を繋ぎ直したものが元の本文になることです。
func TestNewPostBody(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want []viewmodel.PostBodyToken
	}{
		{name: "empty", body: "", want: nil},
		{name: "plain text", body: "ふつうの本文です。", want: []viewmodel.PostBodyToken{postBodyText("ふつうの本文です。")}},
		{
			name: "a reference to a post the thread carries",
			body: ">>1 そのとおりです。",
			want: []viewmodel.PostBodyToken{postBodyReference(">>1", 1), postBodyText(" そのとおりです。")},
		},
		{
			// The thread is what a reply number means anything in, so a number
			// nobody has written yet leads nowhere and is left as it reads.
			//
			// [Ja] レス番号が意味を持つ場所はスレッドであるため、まだ誰も書いていない
			// 番号はどこへも繋がらず、読んだとおりの形で残ります。
			name: "a reference to a post the thread does not carry",
			body: ">>999 まだ誰も書いていません。",
			want: []viewmodel.PostBodyToken{postBodyText(">>999 まだ誰も書いていません。")},
		},
		{
			name: "several references",
			body: ">>2 >>3 の話です。",
			want: []viewmodel.PostBodyToken{
				postBodyReference(">>2", 2),
				postBodyText(" "),
				postBodyReference(">>3", 3),
				postBodyText(" の話です。"),
			},
		},
		{
			// The same number twice is stored as one reference and rendered as
			// two links, because the body wrote it in two places.
			//
			// [Ja] 同じ番号を 2 度書いたものは、1 つの参照として保存され、2 つのリンク
			// として描画されます。本文がそれを 2 箇所に書いたためです。
			name: "the same reference twice",
			body: ">>1 >>1",
			want: []viewmodel.PostBodyToken{postBodyReference(">>1", 1), postBodyText(" "), postBodyReference(">>1", 1)},
		},
		{
			name: "a reference with leading zeros",
			body: ">>001 です。",
			want: []viewmodel.PostBodyToken{postBodyReference(">>001", 1), postBodyText(" です。")},
		},
		{
			// Everything that only looks like the convention is text: one
			// greater-than sign is how a quotation is written, a full-width form
			// is what a Japanese keyboard produces, and zero is not a post.
			//
			// [Ja] 慣習に見えるだけのものはすべてテキストです。不等号 1 つは引用の書き方
			// であり、全角の形は日本語入力が生むものであり、0 は投稿ではありません。
			name: "text that only looks like a reference",
			body: "> 1 と ＞＞1 と >> 1 と >>0 は参照ではありません。",
			want: []viewmodel.PostBodyToken{postBodyText("> 1 と ＞＞1 と >> 1 と >>0 は参照ではありません。")},
		},
		{
			name: "a url on its own",
			body: "https://example.com/help",
			want: []viewmodel.PostBodyToken{postBodyURL("https://example.com/help")},
		},
		{
			name: "a url in a sentence",
			body: "詳しくは https://example.com/help をどうぞ。",
			want: []viewmodel.PostBodyToken{
				postBodyText("詳しくは "),
				postBodyURL("https://example.com/help"),
				postBodyText(" をどうぞ。"),
			},
		},
		{
			// The punctuation closing the sentence is not part of the address,
			// and a link carrying it would ask for a page that is not there.
			//
			// [Ja] 文を閉じる句読点はアドレスの一部ではなく、それを含んだリンクは、そこ
			// に無いページを要求することになります。
			name: "a url followed by punctuation",
			body: "http://example.com/help。ほかは https://example.com/faq!",
			want: []viewmodel.PostBodyToken{
				postBodyURL("http://example.com/help"),
				postBodyText("。ほかは "),
				postBodyURL("https://example.com/faq"),
				postBodyText("!"),
			},
		},
		{
			// A parenthesis the address opened is its own; one it did not
			// belongs to the sentence that wrapped it.
			//
			// [Ja] アドレス自身が開いた括弧はアドレスのものであり、開いていない括弧は、
			// それを囲んだ文のものです。
			name: "a url around parentheses",
			body: "(https://example.com/a) と https://example.com/w_(x)",
			want: []viewmodel.PostBodyToken{
				postBodyText("("),
				postBodyURL("https://example.com/a"),
				postBodyText(") と "),
				postBodyURL("https://example.com/w_(x)"),
			},
		},
		{
			// A square bracket the address opened is its own as well. This leaves
			// the bracket surrounding ordinary prose outside the link without
			// taking the balanced brackets of an IPv6 literal away.
			//
			// [Ja] アドレス自身が開いた角括弧もそのアドレスのものです。これにより、通常の
			// 文章を囲む角括弧はリンクの外へ残しつつ、IPv6 リテラルの対応する角括弧は
			// 取り除きません。
			name: "urls around square brackets",
			body: "[https://example.com/a] と http://[::1]",
			want: []viewmodel.PostBodyToken{
				postBodyText("["),
				postBodyURL("https://example.com/a"),
				postBodyText("] と "),
				postBodyURL("http://[::1]"),
			},
		},
		{
			// Japanese runs on without spaces, so the address ends where the
			// sentence resumes rather than at the next space. A link carrying
			// the words after it would ask for a page that is not there.
			//
			// [Ja] 日本語は空白を置かずに続くため、アドレスは次の空白ではなく、文が
			// 再開する場所で終わります。その後ろの言葉を含んだリンクは、そこに無い
			// ページを要求することになります。
			name: "a url a sentence resumes straight after",
			body: "https://example.com/helpを見てください。",
			want: []viewmodel.PostBodyToken{
				postBodyURL("https://example.com/help"),
				postBodyText("を見てください。"),
			},
		},
		{
			// The other side of the same rule: an address written with the
			// characters themselves keeps only its ASCII part inside the link,
			// and the rest reads as the text it is written as. What a browser
			// puts on the clipboard is percent-encoded and stays whole.
			//
			// [Ja] 同じ規則の裏側です。文字そのもので書かれたアドレスは、ASCII の部分
			// だけがリンクに入り、残りは書かれたとおりのテキストとして読まれます。
			// ブラウザがクリップボードへ置くものはパーセントエンコード済みであり、
			// 丸ごと残ります。
			name: "a url written without percent-encoding",
			body: "https://example.com/日本語 と https://example.com/%E6%97%A5",
			want: []viewmodel.PostBodyToken{
				postBodyURL("https://example.com/"),
				postBodyText("日本語 と "),
				postBodyURL("https://example.com/%E6%97%A5"),
			},
		},
		{
			// A scheme with nothing after it addresses nothing, so what is left
			// once the sentence takes its punctuation back is text.
			//
			// [Ja] 後ろに何も無いスキームは何も指さないため、文が句読点を取り戻した後に
			// 残るものはテキストです。
			name: "a scheme with no address",
			body: "https://.",
			want: []viewmodel.PostBodyToken{postBodyText("https://.")},
		},
		{
			// A scheme is case-insensitive, so an address written in capitals is
			// the same address as one written in lower case and leads to the same
			// page.
			//
			// [Ja] スキームは大文字小文字を区別しないため、大文字で書かれたアドレスは
			// 小文字で書かれたものと同じアドレスであり、同じページへ繋がります。
			name: "a url with a capitalized scheme",
			body: "HTTPS://example.com/a と Http://example.com/b",
			want: []viewmodel.PostBodyToken{
				postBodyURL("HTTPS://example.com/a"),
				postBodyText(" と "),
				postBodyURL("Http://example.com/b"),
			},
		},
		{
			// Only the two schemes a page is fetched with become links, so no
			// other scheme reaches an href from a body.
			//
			// [Ja] リンクになるのはページを取得する 2 つのスキームだけであり、本文から
			// href へ到達する他のスキームはありません。
			name: "text that only looks like a url",
			body: "ftp://example.com と mailto:someone@example.com と example.com",
			want: []viewmodel.PostBodyToken{
				postBodyText("ftp://example.com と mailto:someone@example.com と example.com"),
			},
		},
		{
			// A reference written straight after an address is not swallowed by
			// it: the greater-than sign is not one of the characters a URL is
			// made of, so the address ends where the reference begins.
			//
			// [Ja] アドレスの直後に書かれた参照がそれに飲み込まれることはありません。
			// 不等号は URL を構成する文字ではないため、参照が始まる場所でアドレスが
			// 終わります。
			name: "a reference straight after a url",
			body: "https://example.com/a>>1",
			want: []viewmodel.PostBodyToken{postBodyURL("https://example.com/a"), postBodyReference(">>1", 1)},
		},
		{
			name: "references and urls across lines",
			body: ">>1 これです\nhttps://example.com/a\n>>2 も見てください",
			want: []viewmodel.PostBodyToken{
				postBodyReference(">>1", 1),
				postBodyText(" これです\n"),
				postBodyURL("https://example.com/a"),
				postBodyText("\n"),
				postBodyReference(">>2", 2),
				postBodyText(" も見てください"),
			},
		},
		{
			// Markup is text like anything else: the body is stored as what was
			// typed, and nothing here gives any of it a meaning of its own.
			//
			// [Ja] マークアップも他と同じくテキストです。本文は打たれたままのものとして
			// 保存され、ここでそのいずれかに独自の意味が与えられることはありません。
			name: "text that looks like markup",
			body: "<b>タグに見える入力</b> や & のような記号。",
			want: []viewmodel.PostBodyToken{postBodyText("<b>タグに見える入力</b> や & のような記号。")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := viewmodel.NewPostBody(tt.body, thread)

			if !slices.Equal(got.Tokens, tt.want) {
				t.Errorf("NewPostBody(%q).Tokens = %v, want %v", tt.body, got.Tokens, tt.want)
			}

			// Whatever the body is split into has to be the body again once it
			// is put back together. A visitor's words are shown as they were
			// written, so no reading of them may drop or duplicate a character.
			//
			// [Ja] 本文が何に分解されようと、繋ぎ直したものは再び本文でなければなりま
			// せん。訪問者の言葉は書かれたままに表示されるため、その読み取りが文字を
			// 落としたり重複させたりしてはなりません。
			var joined string
			for _, token := range got.Tokens {
				joined += token.Text
			}
			if joined != tt.body {
				t.Errorf("NewPostBody(%q) joined back to %q", tt.body, joined)
			}
		})
	}
}

// TestNewPostBodyWithoutPosts verifies that a thread carrying no numbers at all
// leaves every reference as text. A body is read against the thread it sits in,
// and a caller with nothing to resolve against is not a caller with everything
// resolved.
//
// [Ja] TestNewPostBodyWithoutPosts は、番号を 1 つも持たないスレッドではすべての参照が
// テキストのままになることを検証します。本文はそれが置かれたスレッドに照らして読まれる
// ものであり、照らす先を持たない呼び出し元は、すべてが解決した呼び出し元ではありません。
func TestNewPostBodyWithoutPosts(t *testing.T) {
	t.Parallel()

	body := viewmodel.NewPostBody(">>1 です。", nil)

	want := []viewmodel.PostBodyToken{postBodyText(">>1 です。")}
	if !slices.Equal(body.Tokens, want) {
		t.Errorf("NewPostBody with no post numbers = %v, want %v", body.Tokens, want)
	}
}
