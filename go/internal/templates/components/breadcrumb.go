package components

import "github.com/groobb/groobb/go/internal/templates"

// BreadcrumbData is the trail naming where inside the community a page sits, from
// the outermost step down to the page itself. Groobb's addresses are flat on
// purpose — /b/{slug} names a board without naming the category that lists it,
// so that moving a board leaves its links intact — which leaves the trail as the
// only place a visitor learns where they are.
//
// It is defined here, beside the component that renders it, the way the
// community layout defines the columns a page hands it: a handler fills it in,
// and its shape is decided in the one place the markup is.
//
// [Ja] BreadcrumbData は、ページがコミュニティのどこに位置するかを、外側の段からページ
// 自身まで順に示す経路です。Groobb のアドレスは意図的に平坦であり、/b/{slug} は掲示板を
// それを並べるカテゴリーを言わずに名指しします (掲示板を移してもリンクが保たれるように
// するためです)。そのため、訪問者が自分の居場所を知る手立てはこの経路だけになります。
//
// これを描画するコンポーネントの隣で定義するのは、コミュニティレイアウトがページから
// 渡されるカラムを定義しているのと同じ形です。ハンドラーが中身を埋め、その形はマーク
// アップのある 1 箇所で決まります。
type BreadcrumbData struct {
	// Items are the steps of the trail in order, the page itself last. An empty
	// trail renders nothing, which is how a page with no place above it stays
	// free of a one-step breadcrumb.
	//
	// [Ja] Items は経路の各段を順に持ち、最後がページ自身です。空の経路は何も描画
	// しません。上位の場所を持たないページが 1 段だけのパンくずを持たずに済むのは
	// このためです。
	Items []BreadcrumbItem

	// BaseURL is the instance's public base URL, under which each step of the
	// trail is named absolutely in the structured data. A crawler reads that data
	// away from the page it was found on, so a step named by its path alone would
	// leave which site it belongs to open. An empty BaseURL is an instance that
	// has not been told its own address, and the structured data is then left out
	// rather than published with addresses that cannot be resolved.
	//
	// [Ja] BaseURL はインスタンスの公開ベース URL であり、構造化データの中で経路の各段は
	// この下の絶対 URL として名指されます。クローラーはそのデータを、それが書かれていた
	// ページから離れて読むため、パスだけで名指した段は、どのサイトのものかが定まりません。
	// BaseURL が空のときはインスタンスが自身のアドレスを教えられていない状態であり、
	// その場合、解決できないアドレスを載せた構造化データを出す代わりに、構造化データ自体を
	// 描画しません。
	BaseURL string
}

// BreadcrumbItem is one step of the trail. Path is empty for the step standing
// for the page being rendered, which is marked as the current one instead of
// being linked: a link to where the visitor already is has nowhere to take them.
//
// [Ja] BreadcrumbItem は経路の 1 段です。今描画しているページを表す段では Path が空に
// なり、リンクではなく現在地の印を付けます。訪問者が既に居る場所へのリンクには、
// 連れて行く先が無いためです。
type BreadcrumbItem struct {
	Name string
	Path templates.Path
}

// IsCurrent reports whether this step stands for the page being rendered.
//
// [Ja] IsCurrent は、この段が今描画しているページを表すかどうかを返します。
func (i BreadcrumbItem) IsCurrent() bool {
	return i.Path == ""
}

// schemaOrgContext is the vocabulary the structured data below is written in.
//
// [Ja] schemaOrgContext は、以下の構造化データが用いる語彙です。
const schemaOrgContext = "https://schema.org"

// breadcrumbList is the BreadcrumbList the page publishes beside the trail it
// draws, so that a search result can show where the page sits instead of its
// bare URL.
//
// It is built from the same BreadcrumbData the markup is built from, and
// rendered next to it, so the two cannot come to say different things: a
// structured description that contradicts the page it describes costs the site
// the rich result it was written for.
//
// [Ja] breadcrumbList は、ページが描く経路の傍らに公開する BreadcrumbList です。検索
// 結果が、素の URL ではなくページの在り処を示せるようにするためのものです。
//
// マークアップと同じ BreadcrumbData から組み立て、その隣に描画するため、両者が違うことを
// 述べるようにはなりません。記述の対象であるページと食い違う構造化データは、そのために
// 書かれたリッチリザルトの資格をサイトから奪います。
type breadcrumbList struct {
	Context  string               `json:"@context"`
	Type     string               `json:"@type"`
	Elements []breadcrumbListItem `json:"itemListElement"`
}

// breadcrumbListItem is one step of the published trail. Position counts from 1
// in the order the steps are read, and Item is the address of the step, left out
// on the step standing for the page being rendered: it is where the reader
// already is, and the page names that address as its canonical URL.
//
// [Ja] breadcrumbListItem は公開する経路の 1 段です。Position は各段を読む順に 1 から
// 数えます。Item はその段のアドレスであり、今描画しているページを表す段では省きます。
// そこは読み手が既に居る場所であり、ページはそのアドレスを自身の正規 URL として名指して
// いるためです。
type breadcrumbListItem struct {
	Type     string `json:"@type"`
	Position int    `json:"position"`
	Name     string `json:"name"`
	Item     string `json:"item,omitempty"`
}

// structuredData describes the trail in the schema.org vocabulary. The caller
// renders it only when the data carries a base URL, which is what every linked
// step is named under.
//
// [Ja] structuredData は経路を schema.org の語彙で記述します。呼び出し側がこれを描画
// するのは、データがベース URL を持つときだけです。リンクを持つ各段は、その下で名指され
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
