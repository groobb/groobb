package components_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/templates/components"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// TestHead_CanonicalLink verifies which pages declare a canonical address. A
// page that asks not to be indexed does not, even when one was set on it: the
// two together would ask a search engine to consolidate onto an address the same
// page just asked it to leave out, and the pair is refused here so no page can
// publish it by filling in both fields.
//
// [Ja] TestHead_CanonicalLink は、どのページが正規アドレスを宣言するかを検証します。
// インデックスされないよう求めるページは、値が設定されていても宣言しません。2 つを併記
// することは、同じページがたった今除外するよう求めたアドレスへ集約せよと検索エンジンに
// 述べることであり、その組み合わせをここで拒むことで、両方のフィールドを埋めたページが
// それを公開できないようにしています。
func TestHead_CanonicalLink(t *testing.T) {
	t.Parallel()

	const canonical = "https://groobb.example.com/b/jazz"

	tests := []struct {
		name          string
		meta          viewmodel.PageMeta
		wantCanonical bool
	}{
		{
			name:          "インデックスを求めるページは自身のアドレスを宣言する",
			meta:          viewmodel.PageMeta{CanonicalURL: canonical},
			wantCanonical: true,
		},
		{
			name:          "インデックスを求めないページは宣言しない",
			meta:          viewmodel.PageMeta{CanonicalURL: canonical, NoIndex: true},
			wantCanonical: false,
		},
		{
			name:          "アドレスを持たないページは宣言しない",
			meta:          viewmodel.PageMeta{},
			wantCanonical: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()

			var buf bytes.Buffer
			if err := components.Head(tt.meta).Render(ctx, &buf); err != nil {
				t.Fatalf("failed to render: %v", err)
			}

			markup := buf.String()
			if got := strings.Contains(markup, `rel="canonical"`); got != tt.wantCanonical {
				t.Errorf("canonical のリンクの有無 = %v, want %v: %s", got, tt.wantCanonical, markup)
			}
			if tt.meta.NoIndex && !strings.Contains(markup, `<meta name="robots" content="noindex">`) {
				t.Errorf("インデックスを求めないページに noindex が無い: %s", markup)
			}
		})
	}
}

// TestHead_Title verifies that the <title> element carries what the page
// metadata composed, so the element the whole document is named by is written in
// the one place every page passes through.
//
// [Ja] TestHead_Title は、<title> 要素がページのメタ情報の組み立てたものを運ぶことを
// 検証します。文書全体を名付ける要素が、どのページも通る 1 箇所で書かれるためです。
func TestHead_Title(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		meta viewmodel.PageMeta
		want string
	}{
		{
			name: "サイトの名前で終わる",
			meta: viewmodel.PageMeta{Title: "ジャズ・ファンク", SiteName: "ジャズ喫茶"},
			want: "<title>ジャズ・ファンク - ジャズ喫茶</title>",
		},
		{
			name: "コミュニティを持たないインスタンスはページの名前だけを運ぶ",
			meta: viewmodel.PageMeta{Title: "ジャズ・ファンク"},
			want: "<title>ジャズ・ファンク</title>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

			var buf bytes.Buffer
			if err := components.Head(tt.meta).Render(ctx, &buf); err != nil {
				t.Fatalf("failed to render: %v", err)
			}

			if markup := buf.String(); !strings.Contains(markup, tt.want) {
				t.Errorf("マークアップに %q が含まれない: %s", tt.want, markup)
			}
		})
	}
}
