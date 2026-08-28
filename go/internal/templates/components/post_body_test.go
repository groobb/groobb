package components_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/templates/components"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// renderPostBody renders a body against a thread holding the first two posts and
// returns the markup.
//
// [Ja] renderPostBody は、最初の 2 つの投稿を持つスレッドに照らして本文を描画し、その
// マークアップを返します。
func renderPostBody(t *testing.T, body string) string {
	t.Helper()

	ctx := context.Background()

	var buf bytes.Buffer
	if err := components.PostBody(viewmodel.NewPostBody(body, map[int]bool{1: true, 2: true})).Render(ctx, &buf); err != nil {
		t.Fatalf("failed to render: %v", err)
	}

	return buf.String()
}

// TestPostBody verifies that a body reaches the page as text and links rather
// than as markup: a reference leads to the post it names, an address leads out
// of the community and is marked as a visitor's, and everything a visitor wrote
// is escaped no matter what it looks like.
//
// PostBody touches no translations, so a background context is enough.
//
// [Ja] TestPostBody は、本文がマークアップとしてではなくテキストとリンクとしてページへ
// 届くことを検証します。参照は名指した投稿へ繋がり、アドレスはコミュニティの外へ繋がって
// 訪問者のものとして印が付き、訪問者が書いたものは、それが何に見えようとエスケープ
// されます。
//
// PostBody は翻訳に触れないため、background context で十分です。
func TestPostBody(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		body         string
		wantContains []string
		wantOmits    []string
	}{
		{
			name:         "a reference to a post the thread carries",
			body:         ">>1 そのとおりです。",
			wantContains: []string{`<a href="#p1"`, `&gt;&gt;1</a>`, " そのとおりです。"},
		},
		{
			// A number the thread does not carry leads nowhere, so it is written
			// out as the text it is rather than as a link to a missing anchor.
			//
			// [Ja] スレッドが持たない番号はどこへも繋がらないため、存在しないアンカー
			// へのリンクではなく、そのままのテキストとして書き出されます。
			name:         "a reference to a post the thread does not carry",
			body:         ">>9 まだ誰も書いていません。",
			wantContains: []string{"&gt;&gt;9 まだ誰も書いていません。"},
			wantOmits:    []string{"<a "},
		},
		{
			// A link out of a body is a visitor's, not the community's, so it
			// carries the rel a search engine reads that from.
			//
			// [Ja] 本文から外へ出るリンクはコミュニティのものではなく訪問者のもので
			// あるため、検索エンジンがそれを読み取る rel を伴います。
			name: "an address",
			body: "詳しくは https://example.com/help をどうぞ。",
			wantContains: []string{
				`<a href="https://example.com/help"`,
				`rel="nofollow ugc"`,
				`>https://example.com/help</a>`,
			},
		},
		{
			// A body is stored as the plain text it was typed as, so what looks
			// like markup arrives as the characters that were typed. Were it not
			// escaped, anyone could put a script into everyone else's page.
			//
			// [Ja] 本文は打たれたままの平文として保存されるため、マークアップのように
			// 見えるものは打たれた文字として届きます。エスケープしなければ、誰もが
			// 他の全員のページにスクリプトを差し込めることになります。
			name: "text that looks like markup",
			body: `<script>alert("x")</script> や & のような入力。`,
			wantContains: []string{
				"&lt;script&gt;alert(&#34;x&#34;)&lt;/script&gt; や &amp; のような入力。",
			},
			wantOmits: []string{"<script>"},
		},
		{
			// The line breaks are the author's, so the markup keeps them and the
			// style renders them rather than collapsing the body into one line.
			//
			// [Ja] 改行は書き手のものであるため、マークアップはそれを保ち、スタイルが
			// それを描画します。本文を 1 行に畳んでしまうことはありません。
			name:         "line breaks",
			body:         "一行目\n二行目",
			wantContains: []string{"whitespace-pre-wrap", "一行目\n二行目"},
		},
		{
			name:      "an empty body",
			body:      "",
			wantOmits: []string{"<a "},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			html := renderPostBody(t, tt.body)

			for _, want := range tt.wantContains {
				if !strings.Contains(html, want) {
					t.Errorf("rendered body does not contain %q:\n%s", want, html)
				}
			}
			for _, omit := range tt.wantOmits {
				if strings.Contains(html, omit) {
					t.Errorf("rendered body contains %q, which it must not:\n%s", omit, html)
				}
			}
		})
	}
}
