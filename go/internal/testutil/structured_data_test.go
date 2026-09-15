package testutil_test

import (
	"testing"

	"github.com/groobb/groobb/go/internal/testutil"
)

// trailは本ケースが読むマークアップで、ハンドラーのテストが見るものと同じ形を
// 持つ。表示されるパンくずの後ろに、同じ段を記述するJSON-LDが続き、その最後の段は今
// 描画しているページを表すためアドレスを持たない。
const trail = `<nav class="breadcrumb"><ol><li><a href="/c/music">音楽</a></li><li>ジャズ・ファンク</li></ol></nav>` +
	`<script type="application/ld+json">` +
	`{"@context":"https://schema.org","@type":"BreadcrumbList","itemListElement":[` +
	`{"@type":"ListItem","position":1,"name":"音楽","item":"https://groobb.example.com/c/music"},` +
	`{"@type":"ListItem","position":2,"name":"ジャズ・ファンク"}]}` +
	`</script>`

// TestAssertBreadcrumbListは、呼び出し側の期待と一致する経路が受け入れられること
// を検証する。各段はJSON-LDから順に読まれ、リンクを持つ段は絶対アドレスで、現在地の段は
// アドレスを持たないことで照合される。拒否する側は検証しない。渡した *testing.Tを通じて
// 報告される失敗は、検証しているテスト自身の失敗になるためである。
func TestAssertBreadcrumbList(t *testing.T) {
	t.Parallel()

	testutil.AssertBreadcrumbList(t, trail,
		[]string{"音楽", "ジャズ・ファンク"},
		[]string{"https://groobb.example.com/c/music", ""},
	)
}
