package components_test

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/templates"
	"github.com/groobb/groobb/go/internal/templates/components"
	"github.com/groobb/groobb/go/internal/testutil"
)

// breadcrumbBaseURLは本テストでのインスタンスの公開ベースURLであり、構造化
// データの中でリンクを持つ各段がその下で名指される値です。
const breadcrumbBaseURL = "https://groobb.example.com"

// renderBreadcrumbは経路を描画し、そのマークアップを返します。
func renderBreadcrumb(t *testing.T, data components.BreadcrumbData) string {
	t.Helper()

	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

	var buf bytes.Buffer
	if err := components.Breadcrumb(data).Render(ctx, &buf); err != nil {
		t.Fatalf("描画に失敗: %v", err)
	}

	return buf.String()
}

// structuredDataOfは、マークアップが持つJSON-LDをデコードして返します。持って
// いないときはテストを失敗させ、クローラーが読む経路について検証する呼び出し側が、経路が
// 無い状態で通ってしまわないようにします。
func structuredDataOf(t *testing.T, markup string) map[string]any {
	t.Helper()

	const opening = `<script type="application/ld+json">`
	structuredData := testutil.Element(t, markup, opening, "</script>")

	var decoded map[string]any
	if err := json.Unmarshal([]byte(strings.TrimPrefix(structuredData, opening)), &decoded); err != nil {
		t.Fatalf("JSON-LDをデコードできない: %v (%s)", err, structuredData)
	}

	return decoded
}

// TestBreadcrumb_StructuredDataは、訪問者が読む経路がBreadcrumbListとしても
// 公開されることを検証します。同じ名前が同じ順で並び、各段はインスタンスの公開ベースURL
// の下で名指され、今描画しているページを表す段は自身のアドレスを持ちません。
func TestBreadcrumb_StructuredData(t *testing.T) {
	t.Parallel()

	markup := renderBreadcrumb(t, components.BreadcrumbData{
		Items: []components.BreadcrumbItem{
			{Name: "音楽", Path: templates.CategoryPath("music")},
			{Name: "ジャズ・ファンク", Path: templates.BoardPath("jazz")},
			{Name: "枯葉の名演"},
		},
		BaseURL: breadcrumbBaseURL,
	})

	decoded := structuredDataOf(t, markup)
	if got, want := decoded["@context"], "https://schema.org"; got != want {
		t.Errorf("@context = %v、期待値 = %v", got, want)
	}
	if got, want := decoded["@type"], "BreadcrumbList"; got != want {
		t.Errorf("@type = %v、期待値 = %v", got, want)
	}

	elements, ok := decoded["itemListElement"].([]any)
	if !ok {
		t.Fatalf("itemListElement = %v、期待値はリスト", decoded["itemListElement"])
	}
	if got, want := len(elements), 3; got != want {
		t.Fatalf("itemListElementの数 = %d、期待値 = %d", got, want)
	}

	wants := []struct {
		name string
		item string
	}{
		{name: "音楽", item: breadcrumbBaseURL + "/c/music"},
		{name: "ジャズ・ファンク", item: breadcrumbBaseURL + "/b/jazz"},
		{name: "枯葉の名演", item: ""},
	}
	for i, want := range wants {
		element, ok := elements[i].(map[string]any)
		if !ok {
			t.Fatalf("itemListElement[%d] = %v、期待値はオブジェクト", i, elements[i])
		}
		if got, want := element["@type"], "ListItem"; got != want {
			t.Errorf("itemListElement[%d].@type = %v、期待値 = %v", i, got, want)
		}
		// positionはJSONの数値としてデコードされるため、数値として比較する。
		if got, want := element["position"], float64(i+1); got != want {
			t.Errorf("itemListElement[%d].position = %v、期待値 = %v", i, got, want)
		}
		if got := element["name"]; got != want.name {
			t.Errorf("itemListElement[%d].name = %v、期待値 = %v", i, got, want.name)
		}
		if want.item == "" {
			if got, ok := element["item"]; ok {
				t.Errorf("itemListElement[%d].item = %v、期待値は現在地の段がアドレスを持たないこと", i, got)
			}
			continue
		}
		if got := element["item"]; got != want.item {
			t.Errorf("itemListElement[%d].item = %v、期待値 = %v", i, got, want.item)
		}
	}
}

// TestBreadcrumb_ItemLanguagesは、リンクの段と現在地の段のどちらでも、それぞれの
// 文言の言語を宣言でき、言語が空なら引き続きページから継承することを検証します。
func TestBreadcrumb_ItemLanguages(t *testing.T) {
	t.Parallel()

	markup := renderBreadcrumb(t, components.BreadcrumbData{
		Items: []components.BreadcrumbItem{
			{Name: "音楽", Path: templates.CategoryPath("music")},
			{Name: "English board", Path: templates.BoardPath("english"), Lang: "en"},
			{Name: "Records I picked up", Lang: "en"},
		},
	})

	inherited := testutil.OpeningTag(t, markup, ">音楽<")
	if strings.Contains(inherited, "lang=") {
		t.Errorf("言語を持たない段 = %s、期待値はlang属性なし", inherited)
	}
	linked := testutil.OpeningTag(t, markup, ">English board<")
	if !strings.Contains(linked, `lang="en"`) {
		t.Errorf("言語を持つリンクの段 = %s、期待値 = lang=\"en\"", linked)
	}
	current := testutil.OpeningTag(t, markup, `aria-current="page"`)
	if !strings.Contains(current, `lang="en"`) {
		t.Errorf("言語を持つ現在地の段 = %s、期待値 = lang=\"en\"", current)
	}
}

// TestBreadcrumb_WithoutABaseURLは、自身の公開アドレスを教えられていない
// インスタンスでも訪問者向けの経路は描くが、構造化データは公開しないことを検証します。
// そこでリンクを持つ各段は絶対アドレスであり、組み立てる元がありません。
func TestBreadcrumb_WithoutABaseURL(t *testing.T) {
	t.Parallel()

	markup := renderBreadcrumb(t, components.BreadcrumbData{
		Items: []components.BreadcrumbItem{
			{Name: "音楽", Path: templates.CategoryPath("music")},
			{Name: "ジャズ・ファンク"},
		},
	})

	if !strings.Contains(markup, `href="/c/music"`) {
		t.Errorf("経路が描画されていない: %s", markup)
	}
	if strings.Contains(markup, "ld+json") {
		t.Errorf("ベースURLが無いのに構造化データが描画されている: %s", markup)
	}
}

// TestBreadcrumb_WithoutItemsは、上位の場所を持たないページが、空のランドマークや
// 段を1つも持たない一覧ではなく、経路もそれを記述する構造化データも描画しないことを
// 検証します。
func TestBreadcrumb_WithoutItems(t *testing.T) {
	t.Parallel()

	if markup := renderBreadcrumb(t, components.BreadcrumbData{BaseURL: breadcrumbBaseURL}); markup != "" {
		t.Errorf("段を持たない経路が描画されている: %s", markup)
	}
}
