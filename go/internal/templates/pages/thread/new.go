package thread

import (
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/templates/components"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// NewPageDataはスレッドを立てるフォームのデータです。投稿先の掲示板、その掲示板の
// 在り処を示す経路、そして現在のフィールドの値を持ちます。
//
// TitleとBodyは、バリデーションエラー後に再描画されたフォームが書かれたものを保つ
// ようにエコーバックし、Languagesは選択を併せて運びます。これにより3つのいずれも
// 打ち直す・選び直す必要がありません。FormErrorsはメッセージを運びます。フィールド別の
// ものはそのフィールドの下に、フォーム全体のものは、フィールドではなく送信そのものに
// 問題があることを述べるものとして上部に置きます。
//
// BoardSlugはフォームの送信先で、BoardNameはページがどの掲示板にスレッドを立てるのかを
// 述べるものです。両方が要ります。アドレスは名前ではなく、掲示板をslugで名指すページは、
// コミュニティが書いたのではない語を訪問者に見せることになるためです。
type NewPageData struct {
	CSRFToken  string
	BoardSlug  string
	BoardName  string
	Breadcrumb components.BreadcrumbData
	Title      string
	Languages  []NewLanguage
	Body       string

	// PostIntervalSecondsは1人が投稿と投稿の間に置く時間で、整数秒です。フォームが
	// これを述べるのは、前の投稿から間もない送信が拒否される前に、待ち時間が分かるように
	// するためです。
	PostIntervalSeconds int

	FormErrors *model.ValidationError
}

// newFieldsはフォームの入力欄を、読まれる順に並べたものです。キャレットの落ちる欄を
// この順で探し、フォームの上の要約もこの順で並べるため、拒否された送信は、メッセージが
// たまたま組み立てられた順の欄ではなく、訪問者が戻るべき最も上の欄にキャレットを置いて
// 返ってきます。
var newFields = []components.FormErrorSummaryField{
	{Name: "title", LabelKey: "thread_new_title_label"},
	{Name: "language", LabelKey: "thread_new_language_label"},
	{Name: "body", LabelKey: "thread_new_body_label"},
}

// ErrorSummaryは、フォームの上の要約を描くためのデータを返します。このページ自身の
// 見出し、読まれる順の入力欄、そして直前の送信についてのメッセージです。
func (d NewPageData) ErrorSummary() components.FormErrorSummaryData {
	return components.FormErrorSummaryData{
		HeadingKey: "thread_new_errors_heading",
		Fields:     newFields,
		Errors:     d.FormErrors,
	}
}

// AutofocusSummaryは、フォームの上のエラー要約がフォーカスを取るかどうかを返します。
// 前の投稿から間もなく届いた送信や、失敗した保存のように、フィールドの中身ではなく送信
// そのものが拒否されたときにそうします。そのとき訪問者が読むべきものはフォームの上の理由で
// あり、変えるべきものが訪問者のどのフィールドでもないためです。
func (d NewPageData) AutofocusSummary() bool {
	return d.FormErrors.HasGlobalError()
}

// AutofocusFieldは、fieldがページを開いたときにキャレットの置かれる入力欄である
// かどうかを返します。直すところのある最初の欄、まだ何も問題の無いフォームではタイトル
// です。要約に道を譲るため、全体として拒否された送信では、悪いところの無いフィールドでは
// なく理由がフォーカスされたままになります。
func (d NewPageData) AutofocusField(field string) bool {
	if d.AutofocusSummary() {
		return false
	}

	for _, candidate := range newFields {
		if d.FormErrors.HasFieldError(candidate.Name) {
			return candidate.Name == field
		}
	}

	return field == newFields[0].Name
}

// NewLanguageは主言語のselectの1つの選択肢です。送信する値、ページが見せる形の
// 言語、そしてフォームが開いたときに選ばれているかどうかを持ちます。
//
// ValueをLanguageの傍らに持つのは、選択肢が送信するのがスレッド言語そのものであり、
// 表示用の形はそれを保持していないためです。どの表示言語にも解決しない言語は名前もタグも
// 持たず、その選択肢は何も送信しないことになります。
type NewLanguage struct {
	Value    string
	Language viewmodel.ThreadLanguage
	Selected bool
}

// NewHeadingIDはこのページの主見出しのidです。コミュニティレイアウトが
// aria-labelledbyで <main> ランドマークをこれに向けるため、領域のアクセシブルな名前と、
// 目で見る訪問者が読む見出しが同じ文字列になります。
const NewHeadingID = "thread-new-heading"
