package testutil_test

import (
	"testing"

	"github.com/groobb/groobb/go/internal/testutil"
)

// page is the markup the cases read, shaped like what a handler test looks at:
// a list of rows, one of which nests a list of its own so that the caveat about
// the first close winning has something to happen on.
//
// [Ja] page は各ケースが読むマークアップで、ハンドラーのテストが見るものと同じ形を
// 持つ。行の一覧であり、そのうち 1 つは自身の中に一覧を入れ子にしている。最初の閉じが
// 採られるという注意点が現れる場所を作るためである。
const page = `<ul class="list">` +
	`<li id="row-1"><a href="/a" class="link">first</a><ul><li id="row-1-1">nested</li></ul></li>` +
	`<li id="row-2"><a href="/b" class="link">second</a></li>` +
	`</ul>`

// TestOpeningTag verifies that the opening tag comes back for the element whose
// opening tag carries the marker, and that it stops at that element's ">"
// rather than running on to what follows.
//
// [Ja] TestOpeningTag は、marker を開始タグに持つ要素の開始タグが返ること、そしてそれが
// 後続へ伸びずにその要素の ">" で止まることを検証する。
func TestOpeningTag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		marker string
		want   string
	}{
		{
			name:   "id を marker にすると、その id を持つ要素の開始タグが返る",
			marker: `id="row-2"`,
			want:   `<li id="row-2">`,
		},
		{
			name:   "属性を marker にすると、その属性を持つ要素の開始タグが返る",
			marker: `href="/b"`,
			want:   `<a href="/b" class="link">`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := testutil.OpeningTag(t, page, tt.marker); got != tt.want {
				t.Errorf("OpeningTag(%q) = %q, want %q", tt.marker, got, tt.want)
			}
		})
	}
}

// TestElement verifies that the slice starts at the marker and stops at the
// first closing after it, which is what keeps an assertion about one row from
// being satisfied by the next one.
//
// The nested case is asserted rather than worked around: a marker naming a row
// that nests a row of its own stops at the inner close, so a caller that needs
// the whole element picks a closing only its end produces.
//
// [Ja] TestElement は、切り出しが marker から始まり、その後の最初の closing で止まる
// ことを検証する。これが、ある行についての検証が次の行で満たされるのを防いでいる。
//
// 入れ子のケースは回避せずそのまま検証する。自身の中に行を入れ子にした行を marker が
// 名指した場合は内側の閉じで止まるため、要素の全体が要る呼び出し側は、その終わりだけが
// 生む closing を選ぶ。
func TestElement(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		marker  string
		closing string
		want    string
	}{
		{
			name:    "marker から最初の closing までを返し、次の行には届かない",
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
			name:    "その要素の終わりだけが生む closing なら要素全体が返る",
			marker:  `<li id="row-1">`,
			closing: "</ul></li>",
			want:    `<li id="row-1"><a href="/a" class="link">first</a><ul><li id="row-1-1">nested</li>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := testutil.Element(t, page, tt.marker, tt.closing); got != tt.want {
				t.Errorf("Element(%q, %q) = %q, want %q", tt.marker, tt.closing, got, tt.want)
			}
		})
	}
}
