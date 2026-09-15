package layouts

import (
	"github.com/a-h/templ"

	"github.com/groobb/groobb/go/internal/viewmodel"
)

// CommunityLayoutDataはコミュニティレイアウトへ渡す1つの引数で、文書のメタ
// データ・共通サイドバー・ページが所有する2つのコンテンツカラムを持ちます。シェルの入力を
// まとめることで、レイアウトが発展してもテンプレート呼び出しのシグネチャをすべて変えずに
// 済みます。
type CommunityLayoutData struct {
	Meta    viewmodel.PageMeta
	Sidebar viewmodel.Sidebar
	Columns CommunityColumns
}

// CommunityColumnはコミュニティのシェルが持つ2つのコンテンツカラムの一方を
// 指します。
type CommunityColumn int

const (
	// CommunityCenterColumnはサイドバーの隣の一覧カラムです。ページが並べるもの
	// (カテゴリーの掲示板、掲示板のスレッド) を持ちます。
	CommunityCenterColumn CommunityColumn = iota

	// CommunityRightColumnは最も広い、読むためのカラムです。ページを開いて読む
	// もの (スレッドの投稿) を持ちます。
	CommunityRightColumn
)

// CommunityColumnsはページがコミュニティレイアウトへ渡す2つのコンテンツ
// カラムと、そのページが本当に扱っているのがどちらか、そしてそれぞれの領域をどう名付ける
// かを表します。
//
// Mainはページが自分では表せない2つのことを決めます。どちらのカラムがスキップ
// リンクの飛び先である <main> ランドマークになるか、そして3カラムが横に並ばない
// 狭いビューポートでどちらが落とされるかです。
//
// 主カラムは文字列ではなく自身の見出しで名付けます。アクセシブルな名前と、目で見る訪問者が
// 読む見出しが、1箇所で決まる同じ文字列になるためです。もう一方のカラムはページの主題の
// 見出しを持たないため、それが何を表す場所なのかを述べるラベルで名付けます。カラムを識別
// するのは、それがどこにあるかではなく何を持つかであり、どちらの名前も左右に触れないのは
// そのためです。
//
// もう一方に見せるものを持たない主題のページ — 掲示板そのものを並べるコミュニティの
// ホーム — はRightをnilのままにし、レイアウトはサイドバーと中央のカラムだけを
// 描きます。そのようなページは定義上そのカラム自身であるため、レイアウトはMainを
// 参照せずにそのカラムを主カラムとし、ComplementaryLabelは何も名付けません。
type CommunityColumns struct {
	Center templ.Component
	Right  templ.Component

	// MainLabelledByは主カラムを名付ける見出しのidです。ページはその見出しを
	// Mainが指すカラムの中に描画します。
	MainLabelledBy string

	// ComplementaryLabelは主ではないほうのカラムを名付けます。
	ComplementaryLabel string

	Main CommunityColumn
}
