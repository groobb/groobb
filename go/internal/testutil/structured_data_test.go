package testutil_test

import (
	"testing"

	"github.com/groobb/groobb/go/internal/testutil"
)

// trail is the markup the case reads, shaped like what a handler test looks at:
// the visible breadcrumb followed by the JSON-LD describing the same steps, the
// last of which stands for the page being rendered and carries no address.
//
// [Ja] trail は本ケースが読むマークアップで、ハンドラーのテストが見るものと同じ形を
// 持つ。表示されるパンくずの後ろに、同じ段を記述する JSON-LD が続き、その最後の段は今
// 描画しているページを表すためアドレスを持たない。
const trail = `<nav class="breadcrumb"><ol><li><a href="/c/music">音楽</a></li><li>ジャズ・ファンク</li></ol></nav>` +
	`<script type="application/ld+json">` +
	`{"@context":"https://schema.org","@type":"BreadcrumbList","itemListElement":[` +
	`{"@type":"ListItem","position":1,"name":"音楽","item":"https://groobb.example.com/c/music"},` +
	`{"@type":"ListItem","position":2,"name":"ジャズ・ファンク"}]}` +
	`</script>`

// TestAssertBreadcrumbList verifies that a trail matching what the caller
// expects is accepted: the steps are read out of the JSON-LD in order, the
// linked one by its absolute address and the current one by the absence of any.
// The rejecting side is not exercised, since a failure reported through the
// *testing.T handed in is the failure of the test doing the exercising.
//
// [Ja] TestAssertBreadcrumbList は、呼び出し側の期待と一致する経路が受け入れられること
// を検証する。各段は JSON-LD から順に読まれ、リンクを持つ段は絶対アドレスで、現在地の段は
// アドレスを持たないことで照合される。拒否する側は検証しない。渡した *testing.T を通じて
// 報告される失敗は、検証しているテスト自身の失敗になるためである。
func TestAssertBreadcrumbList(t *testing.T) {
	t.Parallel()

	testutil.AssertBreadcrumbList(t, trail,
		[]string{"音楽", "ジャズ・ファンク"},
		[]string{"https://groobb.example.com/c/music", ""},
	)
}
