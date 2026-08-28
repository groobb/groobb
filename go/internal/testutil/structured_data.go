package testutil

import (
	"encoding/json"
	"strings"
	"testing"
)

// jsonLDOpening opens the script element a page publishes its structured data
// in. The type is what tells a crawler the block is JSON-LD rather than a
// script to run, so it is matched here rather than a bare <script>.
//
// [Ja] jsonLDOpening は、ページが構造化データを公開する script 要素の開始タグです。
// そのブロックが実行するスクリプトではなく JSON-LD であることをクローラーに伝えるのは
// type であるため、素の <script> ではなくこれを照合します。
const jsonLDOpening = `<script type="application/ld+json">`

// AssertBreadcrumbList verifies the trail the page publishes to a crawler: a
// BreadcrumbList whose steps carry wantNames in order, each linked step named by
// the absolute address at the same index in wantItems. An empty wantItems entry
// is the step standing for the page being rendered, which must carry no address
// of its own.
//
// It is asserted in a handler's test, and not only in the breadcrumb component's
// own, because the handler is where the instance's public base URL reaches the
// component. A handler that stopped passing it would leave the component correct
// and the page silent.
//
// [Ja] AssertBreadcrumbList は、ページがクローラーへ公開する経路を検証します。
// BreadcrumbList の各段が wantNames を順に持ち、リンクを持つ各段が wantItems の同じ位置に
// ある絶対アドレスで名指されることを確かめます。wantItems の空の要素は今描画しているページ
// を表す段であり、自身のアドレスを持たないことを求めます。
//
// これをパンくずコンポーネント自身のテストだけでなくハンドラーのテストでも検証するのは、
// インスタンスの公開ベース URL がコンポーネントへ届くのがハンドラーだからです。ハンドラーが
// それを渡さなくなっても、コンポーネントは正しいままページだけが黙ります。
func AssertBreadcrumbList(t *testing.T, body string, wantNames, wantItems []string) {
	t.Helper()

	if len(wantNames) != len(wantItems) {
		t.Fatalf("テストの期待値の名前数 = %d, アドレス数 = %d", len(wantNames), len(wantItems))
	}

	structuredData := Element(t, body, jsonLDOpening, "</script>")
	var decoded struct {
		Type     string `json:"@type"`
		Elements []struct {
			Type     string  `json:"@type"`
			Position int     `json:"position"`
			Name     string  `json:"name"`
			Item     *string `json:"item"`
		} `json:"itemListElement"`
	}
	if err := json.Unmarshal([]byte(strings.TrimPrefix(structuredData, jsonLDOpening)), &decoded); err != nil {
		t.Fatalf("BreadcrumbList をデコードできない: %v (%s)", err, structuredData)
	}

	if got, want := decoded.Type, "BreadcrumbList"; got != want {
		t.Errorf("構造化データの @type = %q, want %q", got, want)
	}
	if got, want := len(decoded.Elements), len(wantNames); got != want {
		t.Fatalf("BreadcrumbList の段数 = %d, want %d", got, want)
	}

	for i, element := range decoded.Elements {
		if got, want := element.Type, "ListItem"; got != want {
			t.Errorf("BreadcrumbList[%d].@type = %q, want %q", i, got, want)
		}
		if got, want := element.Position, i+1; got != want {
			t.Errorf("BreadcrumbList[%d].position = %d, want %d", i, got, want)
		}
		if got, want := element.Name, wantNames[i]; got != want {
			t.Errorf("BreadcrumbList[%d].name = %q, want %q", i, got, want)
		}
		switch want := wantItems[i]; {
		case want == "":
			if element.Item != nil {
				t.Errorf("BreadcrumbList[%d].item = %q, want 現在地の段では省略", i, *element.Item)
			}
		case element.Item == nil:
			t.Errorf("BreadcrumbList[%d].item が無い, want %q", i, want)
		default:
			if got := *element.Item; got != want {
				t.Errorf("BreadcrumbList[%d].item = %q, want %q", i, got, want)
			}
		}
	}
}
