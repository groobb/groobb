package viewmodel_test

import (
	"slices"
	"testing"

	"github.com/groobb/groobb/go/internal/viewmodel"
)

// postBodyText・postBodyReference・postBodyURLは、あるケースが期待するトークンを
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

// threadは、下記のケースが照らされるレス番号の集合です。最初の3つの投稿を持つ
// スレッドであり、本文がそこにある番号と無い番号の両方を名指すのに足ります。
var thread = map[int]bool{1: true, 2: true, 3: true}

// TestNewPostBodyは、描画のために本文がどう分解されるのかを検証します。何がリンク
// になり、何がテキストのままであるか、そして断片を繋ぎ直したものが元の本文になることです。
func TestNewPostBody(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want []viewmodel.PostBodyToken
	}{
		{name: "空", body: "", want: nil},
		{name: "ただのテキスト", body: "ふつうの本文です。", want: []viewmodel.PostBodyToken{postBodyText("ふつうの本文です。")}},
		{
			name: "スレッドにある投稿への参照",
			body: ">>1 そのとおりです。",
			want: []viewmodel.PostBodyToken{postBodyReference(">>1", 1), postBodyText(" そのとおりです。")},
		},
		{
			// レス番号が意味を持つ場所はスレッドであるため、まだ誰も書いていない
			// 番号はどこへも繋がらず、読んだとおりの形で残ります。
			name: "スレッドに無い投稿への参照",
			body: ">>999 まだ誰も書いていません。",
			want: []viewmodel.PostBodyToken{postBodyText(">>999 まだ誰も書いていません。")},
		},
		{
			name: "複数の参照",
			body: ">>2 >>3 の話です。",
			want: []viewmodel.PostBodyToken{
				postBodyReference(">>2", 2),
				postBodyText(" "),
				postBodyReference(">>3", 3),
				postBodyText(" の話です。"),
			},
		},
		{
			// 同じ番号を2度書いたものは、1つの参照として保存され、2つのリンク
			// として描画されます。本文がそれを2箇所に書いたためです。
			name: "同じ参照を2度",
			body: ">>1 >>1",
			want: []viewmodel.PostBodyToken{postBodyReference(">>1", 1), postBodyText(" "), postBodyReference(">>1", 1)},
		},
		{
			name: "先頭にゼロを持つ参照",
			body: ">>001 です。",
			want: []viewmodel.PostBodyToken{postBodyReference(">>001", 1), postBodyText(" です。")},
		},
		{
			// 慣習に見えるだけのものはすべてテキストです。不等号1つは引用の書き方
			// であり、全角の形は日本語入力が生むものであり、0は投稿ではありません。
			name: "参照に見えるだけのテキスト",
			body: "> 1 と ＞＞1 と >> 1 と >>0 は参照ではありません。",
			want: []viewmodel.PostBodyToken{postBodyText("> 1 と ＞＞1 と >> 1 と >>0 は参照ではありません。")},
		},
		{
			name: "URLだけ",
			body: "https://example.com/help",
			want: []viewmodel.PostBodyToken{postBodyURL("https://example.com/help")},
		},
		{
			name: "文中のURL",
			body: "詳しくは https://example.com/help をどうぞ。",
			want: []viewmodel.PostBodyToken{
				postBodyText("詳しくは "),
				postBodyURL("https://example.com/help"),
				postBodyText(" をどうぞ。"),
			},
		},
		{
			// 文を閉じる句読点はアドレスの一部ではなく、それを含んだリンクは、そこ
			// に無いページを要求することになります。
			name: "句読点が続くURL",
			body: "http://example.com/help。ほかは https://example.com/faq!",
			want: []viewmodel.PostBodyToken{
				postBodyURL("http://example.com/help"),
				postBodyText("。ほかは "),
				postBodyURL("https://example.com/faq"),
				postBodyText("!"),
			},
		},
		{
			// アドレス自身が開いた括弧はアドレスのものであり、開いていない括弧は、
			// それを囲んだ文のものです。
			name: "丸括弧と隣り合うURL",
			body: "(https://example.com/a) と https://example.com/w_(x)",
			want: []viewmodel.PostBodyToken{
				postBodyText("("),
				postBodyURL("https://example.com/a"),
				postBodyText(") と "),
				postBodyURL("https://example.com/w_(x)"),
			},
		},
		{
			// アドレス自身が開いた角括弧もそのアドレスのものです。これにより、通常の
			// 文章を囲む角括弧はリンクの外へ残しつつ、IPv6リテラルの対応する角括弧は
			// 取り除きません。
			name: "角括弧と隣り合うURL",
			body: "[https://example.com/a] と http://[::1]",
			want: []viewmodel.PostBodyToken{
				postBodyText("["),
				postBodyURL("https://example.com/a"),
				postBodyText("] と "),
				postBodyURL("http://[::1]"),
			},
		},
		{
			// 日本語は空白を置かずに続くため、アドレスは次の空白ではなく、文が
			// 再開する場所で終わります。その後ろの言葉を含んだリンクは、そこに無い
			// ページを要求することになります。
			name: "直後に文が再開するURL",
			body: "https://example.com/helpを見てください。",
			want: []viewmodel.PostBodyToken{
				postBodyURL("https://example.com/help"),
				postBodyText("を見てください。"),
			},
		},
		{
			// 同じ規則の裏側です。文字そのもので書かれたアドレスは、ASCIIの部分
			// だけがリンクに入り、残りは書かれたとおりのテキストとして読まれます。
			// ブラウザがクリップボードへ置くものはパーセントエンコード済みであり、
			// 丸ごと残ります。
			name: "パーセントエンコードされていないURL",
			body: "https://example.com/日本語 と https://example.com/%E6%97%A5",
			want: []viewmodel.PostBodyToken{
				postBodyURL("https://example.com/"),
				postBodyText("日本語 と "),
				postBodyURL("https://example.com/%E6%97%A5"),
			},
		},
		{
			// 後ろに何も無いスキームは何も指さないため、文が句読点を取り戻した後に
			// 残るものはテキストです。
			name: "アドレスの無いスキーム",
			body: "https://.",
			want: []viewmodel.PostBodyToken{postBodyText("https://.")},
		},
		{
			// スキームは大文字小文字を区別しないため、大文字で書かれたアドレスは
			// 小文字で書かれたものと同じアドレスであり、同じページへ繋がります。
			name: "スキームが大文字のURL",
			body: "HTTPS://example.com/a と Http://example.com/b",
			want: []viewmodel.PostBodyToken{
				postBodyURL("HTTPS://example.com/a"),
				postBodyText(" と "),
				postBodyURL("Http://example.com/b"),
			},
		},
		{
			// リンクになるのはページを取得する2つのスキームだけであり、本文から
			// hrefへ到達する他のスキームはありません。
			name: "URLに見えるだけのテキスト",
			body: "ftp://example.com と mailto:someone@example.com と example.com",
			want: []viewmodel.PostBodyToken{
				postBodyText("ftp://example.com と mailto:someone@example.com と example.com"),
			},
		},
		{
			// アドレスの直後に書かれた参照がそれに飲み込まれることはありません。
			// 不等号はURLを構成する文字ではないため、参照が始まる場所でアドレスが
			// 終わります。
			name: "URLの直後の参照",
			body: "https://example.com/a>>1",
			want: []viewmodel.PostBodyToken{postBodyURL("https://example.com/a"), postBodyReference(">>1", 1)},
		},
		{
			name: "複数行にまたがる参照とURL",
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
			// マークアップも他と同じくテキストです。本文は打たれたままのものとして
			// 保存され、ここでそのいずれかに独自の意味が与えられることはありません。
			name: "マークアップに見えるテキスト",
			body: "<b>タグに見える入力</b> や & のような記号。",
			want: []viewmodel.PostBodyToken{postBodyText("<b>タグに見える入力</b> や & のような記号。")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := viewmodel.NewPostBody(tt.body, thread)

			if !slices.Equal(got.Tokens, tt.want) {
				t.Errorf("NewPostBody(%q).Tokens = %v、期待値 = %v", tt.body, got.Tokens, tt.want)
			}

			// 本文が何に分解されようと、繋ぎ直したものは再び本文でなければなりま
			// せん。訪問者の言葉は書かれたままに表示されるため、その読み取りが文字を
			// 落としたり重複させたりしてはなりません。
			var joined string
			for _, token := range got.Tokens {
				joined += token.Text
			}
			if joined != tt.body {
				t.Errorf("NewPostBody(%q)の断片を繋ぎ直した結果 = %q", tt.body, joined)
			}
		})
	}
}

// TestNewPostBodyWithoutPostsは、番号を1つも持たないスレッドではすべての参照が
// テキストのままになることを検証します。本文はそれが置かれたスレッドに照らして読まれる
// ものであり、照らす先を持たない呼び出し元は、すべてが解決した呼び出し元ではありません。
func TestNewPostBodyWithoutPosts(t *testing.T) {
	t.Parallel()

	body := viewmodel.NewPostBody(">>1 です。", nil)

	want := []viewmodel.PostBodyToken{postBodyText(">>1 です。")}
	if !slices.Equal(body.Tokens, want) {
		t.Errorf("レス番号を渡さないNewPostBodyのTokens = %v、期待値 = %v", body.Tokens, want)
	}
}
