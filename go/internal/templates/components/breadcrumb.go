package components

import "github.com/groobb/groobb/go/internal/templates"

// BreadcrumbDataは、ページがコミュニティのどこに位置するかを、外側の段からページ
// 自身まで順に示す経路です。Groobbのアドレスは意図的に平坦であり、/b/{slug} は掲示板を
// それを並べるカテゴリーを言わずに名指しします (掲示板を移してもリンクが保たれるように
// するためです)。そのため、訪問者が自分の居場所を知る手立てはこの経路だけになります。
//
// これを描画するコンポーネントの隣で定義するのは、コミュニティレイアウトがページから
// 渡されるカラムを定義しているのと同じ形です。ハンドラーが中身を埋め、その形はマーク
// アップのある1箇所で決まります。
type BreadcrumbData struct {
	// Itemsは経路の各段を順に持ち、最後がページ自身です。空の経路は何も描画
	// しません。上位の場所を持たないページが1段だけのパンくずを持たずに済むのは
	// このためです。
	Items []BreadcrumbItem

	// BaseURLはインスタンスの公開ベースURLであり、構造化データの中で経路の各段は
	// この下の絶対URLとして名指されます。クローラーはそのデータを、それが書かれていた
	// ページから離れて読むため、パスだけで名指した段は、どのサイトのものかが定まりません。
	// BaseURLが空のときはインスタンスが自身のアドレスを教えられていない状態であり、
	// その場合、解決できないアドレスを載せた構造化データを出す代わりに、構造化データ自体を
	// 描画しません。
	BaseURL string
}

// BreadcrumbItemは経路の1段です。今描画しているページを表す段ではPathが空に
// なり、リンクではなく現在地の印を付けます。訪問者が既に居る場所へのリンクには、
// 連れて行く先が無いためです。
type BreadcrumbItem struct {
	Name string
	Path templates.Path

	// Langはこの段の文言を表すBCP 47言語タグです。空の値ではページから言語を
	// 継承します。
	Lang string
}

// IsCurrentは、この段が今描画しているページを表すかどうかを返します。
func (i BreadcrumbItem) IsCurrent() bool {
	return i.Path == ""
}

// schemaOrgContextは、以下の構造化データが用いる語彙です。
const schemaOrgContext = "https://schema.org"

// breadcrumbListは、ページが描く経路の傍らに公開するBreadcrumbListです。検索
// 結果が、素のURLではなくページの在り処を示せるようにするためのものです。
//
// マークアップと同じBreadcrumbDataから組み立て、その隣に描画するため、両者が違うことを
// 述べるようにはなりません。記述の対象であるページと食い違う構造化データは、そのために
// 書かれたリッチリザルトの資格をサイトから奪います。
type breadcrumbList struct {
	Context  string               `json:"@context"`
	Type     string               `json:"@type"`
	Elements []breadcrumbListItem `json:"itemListElement"`
}

// breadcrumbListItemは公開する経路の1段です。Positionは各段を読む順に1から
// 数えます。Itemはその段のアドレスであり、今描画しているページを表す段では省きます。
// そこは読み手が既に居る場所であり、ページはそのアドレスを自身の正規URLとして名指して
// いるためです。
type breadcrumbListItem struct {
	Type     string `json:"@type"`
	Position int    `json:"position"`
	Name     string `json:"name"`
	Item     string `json:"item,omitempty"`
}

// structuredDataは経路をschema.orgの語彙で記述します。呼び出し側がこれを描画
// するのは、データがベースURLを持つときだけです。リンクを持つ各段は、その下で名指され
// ます。
func (d BreadcrumbData) structuredData() breadcrumbList {
	elements := make([]breadcrumbListItem, len(d.Items))
	for i, item := range d.Items {
		elements[i] = breadcrumbListItem{
			Type:     "ListItem",
			Position: i + 1,
			Name:     item.Name,
		}
		if !item.IsCurrent() {
			elements[i].Item = item.Path.AbsoluteURL(d.BaseURL)
		}
	}

	return breadcrumbList{
		Context:  schemaOrgContext,
		Type:     "BreadcrumbList",
		Elements: elements,
	}
}
