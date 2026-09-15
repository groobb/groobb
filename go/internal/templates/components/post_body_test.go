package components_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/templates/components"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// renderPostBodyは、最初の2つの投稿を持つスレッドに照らして本文を描画し、その
// マークアップを返します。
func renderPostBody(t *testing.T, body string) string {
	t.Helper()

	ctx := context.Background()

	var buf bytes.Buffer
	if err := components.PostBody(viewmodel.NewPostBody(body, map[int]bool{1: true, 2: true})).Render(ctx, &buf); err != nil {
		t.Fatalf("描画に失敗: %v", err)
	}

	return buf.String()
}

// TestPostBodyは、本文がマークアップとしてではなくテキストとリンクとしてページへ
// 届くことを検証します。参照は名指した投稿へ繋がり、アドレスはコミュニティの外へ繋がって
// 訪問者のものとして印が付き、訪問者が書いたものは、それが何に見えようとエスケープ
// されます。
//
// PostBodyは翻訳に触れないため、background contextで十分です。
func TestPostBody(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		body         string
		wantContains []string
		wantOmits    []string
	}{
		{
			name:         "スレッドにある投稿への参照",
			body:         ">>1 そのとおりです。",
			wantContains: []string{`<a href="#p1"`, `&gt;&gt;1</a>`, " そのとおりです。"},
		},
		{
			// スレッドが持たない番号はどこへも繋がらないため、存在しないアンカー
			// へのリンクではなく、そのままのテキストとして書き出されます。
			name:         "スレッドに無い投稿への参照",
			body:         ">>9 まだ誰も書いていません。",
			wantContains: []string{"&gt;&gt;9 まだ誰も書いていません。"},
			wantOmits:    []string{"<a "},
		},
		{
			// 本文から外へ出るリンクはコミュニティのものではなく訪問者のもので
			// あるため、検索エンジンがそれを読み取るrelを伴います。
			name: "アドレス",
			body: "詳しくは https://example.com/help をどうぞ。",
			wantContains: []string{
				`<a href="https://example.com/help"`,
				`rel="nofollow ugc"`,
				`>https://example.com/help</a>`,
			},
		},
		{
			// 本文は打たれたままの平文として保存されるため、マークアップのように
			// 見えるものは打たれた文字として届きます。エスケープしなければ、誰もが
			// 他の全員のページにスクリプトを差し込めることになります。
			name: "マークアップのように見えるテキスト",
			body: `<script>alert("x")</script> や & のような入力。`,
			wantContains: []string{
				"&lt;script&gt;alert(&#34;x&#34;)&lt;/script&gt; や &amp; のような入力。",
			},
			wantOmits: []string{"<script>"},
		},
		{
			// 改行は書き手のものであるため、マークアップはそれを保ち、スタイルが
			// それを描画します。本文を1行に畳んでしまうことはありません。
			name:         "改行",
			body:         "一行目\n二行目",
			wantContains: []string{"whitespace-pre-wrap", "一行目\n二行目"},
		},
		{
			name:      "空の本文",
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
					t.Errorf("描画された本文に %q が含まれていない:\n%s", want, html)
				}
			}
			for _, omit := range tt.wantOmits {
				if strings.Contains(html, omit) {
					t.Errorf("描画された本文に含まれてはならない %q が含まれている:\n%s", omit, html)
				}
			}
		})
	}
}
