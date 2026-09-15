package testutil_test

import (
	"testing"

	"github.com/groobb/groobb/go/internal/testutil"
)

// pageは各ケースが読むマークアップで、ハンドラーのテストが見るものと同じ形を
// 持つ。行の一覧であり、そのうち1つは自身の中に一覧を入れ子にしている。最初の閉じが
// 採られるという注意点が現れる場所を作るためである。
const page = `<ul class="list">` +
	`<li id="row-1"><a href="/a" class="link">first</a><ul><li id="row-1-1">nested</li></ul></li>` +
	`<li id="row-2"><a href="/b" class="link">second</a></li>` +
	`</ul>`

// TestOpeningTagは、markerを開始タグに持つ要素の開始タグが返ること、そしてそれが
// 後続へ伸びずにその要素の ">" で止まることを検証する。
func TestOpeningTag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		marker string
		want   string
	}{
		{
			name:   "idをmarkerにすると、そのidを持つ要素の開始タグが返る",
			marker: `id="row-2"`,
			want:   `<li id="row-2">`,
		},
		{
			name:   "属性をmarkerにすると、その属性を持つ要素の開始タグが返る",
			marker: `href="/b"`,
			want:   `<a href="/b" class="link">`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := testutil.OpeningTag(t, page, tt.marker); got != tt.want {
				t.Errorf("OpeningTag(%q) = %q、期待値 = %q", tt.marker, got, tt.want)
			}
		})
	}
}

// TestElementは、切り出しがmarkerから始まり、その後の最初のclosingで止まる
// ことを検証する。これが、ある行についての検証が次の行で満たされるのを防いでいる。
//
// 入れ子のケースは回避せずそのまま検証する。自身の中に行を入れ子にした行をmarkerが
// 名指した場合は内側の閉じで止まるため、要素の全体が要る呼び出し側は、その終わりだけが
// 生むclosingを選ぶ。
func TestElement(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		marker  string
		closing string
		want    string
	}{
		{
			name:    "markerから最初のclosingまでを返し、次の行には届かない",
			marker:  `id="row-2"`,
			closing: "</li>",
			want:    `id="row-2"><a href="/b" class="link">second</a>`,
		},
		{
			name:    "同じタグを入れ子にする要素では内側の閉じで止まる",
			marker:  `id="row-1"`,
			closing: "</li>",
			want:    `id="row-1"><a href="/a" class="link">first</a><ul><li id="row-1-1">nested`,
		},
		{
			name:    "その要素の終わりだけが生むclosingなら要素全体が返る",
			marker:  `<li id="row-1">`,
			closing: "</ul></li>",
			want:    `<li id="row-1"><a href="/a" class="link">first</a><ul><li id="row-1-1">nested</li>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := testutil.Element(t, page, tt.marker, tt.closing); got != tt.want {
				t.Errorf("Element(%q, %q) = %q、期待値 = %q", tt.marker, tt.closing, got, tt.want)
			}
		})
	}
}
