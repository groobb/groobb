package testutil

import (
	"encoding/json"
	"strings"
	"testing"
)

// jsonLDOpeningは、ページが構造化データを公開するscript要素の開始タグです。
// そのブロックが実行するスクリプトではなくJSON-LDであることをクローラーに伝えるのは
// typeであるため、素の <script> ではなくこれを照合します。
const jsonLDOpening = `<script type="application/ld+json">`

// AssertBreadcrumbListは、ページがクローラーへ公開する経路を検証します。
// BreadcrumbListの各段がwantNamesを順に持ち、リンクを持つ各段がwantItemsの同じ位置に
// ある絶対アドレスで名指されることを確かめます。wantItemsの空の要素は今描画しているページ
// を表す段であり、自身のアドレスを持たないことを求めます。
//
// これをパンくずコンポーネント自身のテストだけでなくハンドラーのテストでも検証するのは、
// インスタンスの公開ベースURLがコンポーネントへ届くのがハンドラーだからです。ハンドラーが
// それを渡さなくなっても、コンポーネントは正しいままページだけが黙ります。
func AssertBreadcrumbList(t *testing.T, body string, wantNames, wantItems []string) {
	t.Helper()

	if len(wantNames) != len(wantItems) {
		t.Fatalf("テストの期待値の名前数 = %d、アドレス数 = %d", len(wantNames), len(wantItems))
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
		t.Fatalf("BreadcrumbListをデコードできない: %v (%s)", err, structuredData)
	}

	if got, want := decoded.Type, "BreadcrumbList"; got != want {
		t.Errorf("構造化データの @type = %q、期待値 = %q", got, want)
	}
	if got, want := len(decoded.Elements), len(wantNames); got != want {
		t.Fatalf("BreadcrumbListの段数 = %d、期待値 = %d", got, want)
	}

	for i, element := range decoded.Elements {
		if got, want := element.Type, "ListItem"; got != want {
			t.Errorf("BreadcrumbList[%d].@type = %q、期待値 = %q", i, got, want)
		}
		if got, want := element.Position, i+1; got != want {
			t.Errorf("BreadcrumbList[%d].position = %d、期待値 = %d", i, got, want)
		}
		if got, want := element.Name, wantNames[i]; got != want {
			t.Errorf("BreadcrumbList[%d].name = %q、期待値 = %q", i, got, want)
		}
		switch want := wantItems[i]; {
		case want == "":
			if element.Item != nil {
				t.Errorf("BreadcrumbList[%d].item = %q、期待値は省略 (現在地の段)", i, *element.Item)
			}
		case element.Item == nil:
			t.Errorf("BreadcrumbList[%d].itemが無い、期待値 = %q", i, want)
		default:
			if got := *element.Item; got != want {
				t.Errorf("BreadcrumbList[%d].item = %q、期待値 = %q", i, got, want)
			}
		}
	}
}
