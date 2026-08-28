package layouts

import (
	"github.com/a-h/templ"

	"github.com/groobb/groobb/go/internal/viewmodel"
)

// CommunityLayoutData is the single argument handed to the community layout:
// the document metadata, the shared sidebar, and the two page-owned content
// columns. Keeping the shell's inputs together lets it evolve without changing
// every template call signature.
//
// [Ja] CommunityLayoutData はコミュニティレイアウトへ渡す 1 つの引数で、文書のメタ
// データ・共通サイドバー・ページが所有する 2 つのコンテンツカラムを持ちます。シェルの入力を
// まとめることで、レイアウトが発展してもテンプレート呼び出しのシグネチャをすべて変えずに
// 済みます。
type CommunityLayoutData struct {
	Meta    viewmodel.PageMeta
	Sidebar viewmodel.Sidebar
	Columns CommunityColumns
}

// CommunityColumn names one of the two content columns of the community shell.
//
// [Ja] CommunityColumn はコミュニティのシェルが持つ 2 つのコンテンツカラムの一方を
// 指します。
type CommunityColumn int

const (
	// CommunityCenterColumn is the list column, next to the sidebar. It holds
	// what a page lists: the boards of a category, or the threads of a board.
	//
	// [Ja] CommunityCenterColumn はサイドバーの隣の一覧カラムです。ページが並べるもの
	// (カテゴリーの掲示板、掲示板のスレッド) を持ちます。
	CommunityCenterColumn CommunityColumn = iota

	// CommunityRightColumn is the reading column, the widest one. It holds what a
	// page is opened to read: the posts of a thread.
	//
	// [Ja] CommunityRightColumn は最も広い、読むためのカラムです。ページを開いて読む
	// もの (スレッドの投稿) を持ちます。
	CommunityRightColumn
)

// CommunityColumns is the pair of content columns a page hands to the community
// layout, together with which of them the page is really about and how each of
// the two resulting regions is named.
//
// Main decides two things the page cannot express by itself: which column
// becomes the <main> landmark the skip link jumps to, and which one is dropped
// on a narrow viewport, where the three columns do not fit side by side.
//
// The main column is named by its own heading rather than by a string, so its
// accessible name and the heading a sighted visitor reads are the same text
// decided in one place. The other column holds no heading of the page's subject,
// so it is named by a label describing what it stands for. A column is
// identified by what it holds rather than by where it sits, which is why neither
// name mentions a side.
//
// A page about something that has no second side to show — the community's home,
// which lists the boards themselves — leaves Right nil, and the layout then draws
// the sidebar and the center column alone. Such a page is the center column by
// definition, so the layout makes that column the main one without consulting
// Main, and ComplementaryLabel names nothing.
//
// [Ja] CommunityColumns はページがコミュニティレイアウトへ渡す 2 つのコンテンツ
// カラムと、そのページが本当に扱っているのがどちらか、そしてそれぞれの領域をどう名付ける
// かを表します。
//
// Main はページが自分では表せない 2 つのことを決めます。どちらのカラムがスキップ
// リンクの飛び先である <main> ランドマークになるか、そして 3 カラムが横に並ばない
// 狭いビューポートでどちらが落とされるかです。
//
// 主カラムは文字列ではなく自身の見出しで名付けます。アクセシブルな名前と、目で見る訪問者が
// 読む見出しが、1 箇所で決まる同じ文字列になるためです。もう一方のカラムはページの主題の
// 見出しを持たないため、それが何を表す場所なのかを述べるラベルで名付けます。カラムを識別
// するのは、それがどこにあるかではなく何を持つかであり、どちらの名前も左右に触れないのは
// そのためです。
//
// もう一方に見せるものを持たない主題のページ — 掲示板そのものを並べるコミュニティの
// ホーム — は Right を nil のままにし、レイアウトはサイドバーと中央のカラムだけを
// 描きます。そのようなページは定義上そのカラム自身であるため、レイアウトは Main を
// 参照せずにそのカラムを主カラムとし、ComplementaryLabel は何も名付けません。
type CommunityColumns struct {
	Center templ.Component
	Right  templ.Component

	// MainLabelledBy is the id of the heading that names the main column. The
	// page renders that heading inside the column Main points at.
	//
	// [Ja] MainLabelledBy は主カラムを名付ける見出しの id です。ページはその見出しを
	// Main が指すカラムの中に描画します。
	MainLabelledBy string

	// ComplementaryLabel names the column that is not the main one.
	//
	// [Ja] ComplementaryLabel は主ではないほうのカラムを名付けます。
	ComplementaryLabel string

	Main CommunityColumn
}
