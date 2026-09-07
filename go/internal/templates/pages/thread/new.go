package thread

import (
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/templates/components"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// NewPageData is the data for the form a thread is started from: the board it
// will be posted in, the trail naming where that board sits, and the fields as
// they stand.
//
// Title and Body are echoed back so a form re-rendered after a validation error
// keeps what was written, and Languages carries the selection along with it, so
// none of the three has to be typed or chosen twice. FormErrors carries the
// messages: per-field ones under the field they belong to, and form-wide ones at
// the top for what is wrong with the submission rather than with a field.
//
// BoardSlug is what the form posts to, and BoardName is what the page says the
// thread is being started in. Both are needed: the address is not the name, and
// a page naming the board by its slug would show the visitor a word the
// community did not write.
//
// [Ja] NewPageData はスレッドを立てるフォームのデータです。投稿先の掲示板、その掲示板の
// 在り処を示す経路、そして現在のフィールドの値を持ちます。
//
// Title と Body は、バリデーションエラー後に再描画されたフォームが書かれたものを保つ
// ようにエコーバックし、Languages は選択を併せて運びます。これにより 3 つのいずれも
// 打ち直す・選び直す必要がありません。FormErrors はメッセージを運びます。フィールド別の
// ものはそのフィールドの下に、フォーム全体のものは、フィールドではなく送信そのものに
// 問題があることを述べるものとして上部に置きます。
//
// BoardSlug はフォームの送信先で、BoardName はページがどの掲示板にスレッドを立てるのかを
// 述べるものです。両方が要ります。アドレスは名前ではなく、掲示板を slug で名指すページは、
// コミュニティが書いたのではない語を訪問者に見せることになるためです。
type NewPageData struct {
	CSRFToken  string
	BoardSlug  string
	BoardName  string
	Breadcrumb components.BreadcrumbData
	Title      string
	Languages  []NewLanguage
	Body       string

	// PostIntervalSeconds is how long one person waits between posts, in whole
	// seconds. The form states it so that the wait is known before a submission
	// is refused for arriving too soon after the last one.
	//
	// [Ja] PostIntervalSeconds は 1 人が投稿と投稿の間に置く時間で、整数秒です。フォームが
	// これを述べるのは、前の投稿から間もない送信が拒否される前に、待ち時間が分かるように
	// するためです。
	PostIntervalSeconds int

	FormErrors *model.ValidationError
}

// newFields are the form's controls in the order they are read. The field the
// caret lands on is looked for in this order, and the summary above the form
// lists them in it, so a refused submission comes back at the top-most one the
// visitor has to return to rather than at whichever one the messages happened to
// be built from.
//
// [Ja] newFields はフォームの入力欄を、読まれる順に並べたものです。キャレットの落ちる欄を
// この順で探し、フォームの上の要約もこの順で並べるため、拒否された送信は、メッセージが
// たまたま組み立てられた順の欄ではなく、訪問者が戻るべき最も上の欄にキャレットを置いて
// 返ってきます。
var newFields = []components.FormErrorSummaryField{
	{Name: "title", LabelKey: "thread_new_title_label"},
	{Name: "language", LabelKey: "thread_new_language_label"},
	{Name: "body", LabelKey: "thread_new_body_label"},
}

// ErrorSummary returns what the summary above the form is drawn from: this
// page's own heading, its controls in reading order, and the messages about the
// last submission.
//
// [Ja] ErrorSummary は、フォームの上の要約を描くためのデータを返します。このページ自身の
// 見出し、読まれる順の入力欄、そして直前の送信についてのメッセージです。
func (d NewPageData) ErrorSummary() components.FormErrorSummaryData {
	return components.FormErrorSummaryData{
		HeadingKey: "thread_new_errors_heading",
		Fields:     newFields,
		Errors:     d.FormErrors,
	}
}

// AutofocusSummary reports whether the error summary above the form takes focus.
// It does when the submission was refused as a whole rather than over something
// in a field — arriving too soon after the last post, or a save that failed —
// because what the visitor has to read is then the reason above the form, and no
// field of theirs is what has to change.
//
// [Ja] AutofocusSummary は、フォームの上のエラー要約がフォーカスを取るかどうかを返します。
// 前の投稿から間もなく届いた送信や、失敗した保存のように、フィールドの中身ではなく送信
// そのものが拒否されたときにそうします。そのとき訪問者が読むべきものはフォームの上の理由で
// あり、変えるべきものが訪問者のどのフィールドでもないためです。
func (d NewPageData) AutofocusSummary() bool {
	return d.FormErrors.HasGlobalError()
}

// AutofocusField reports whether field is the control the page opens with the
// caret in: the first one with something to fix, or the title on a form that has
// nothing wrong with it yet. It yields to the summary, so a submission refused as
// a whole leaves the reason focused rather than a field that is not at fault.
//
// [Ja] AutofocusField は、field がページを開いたときにキャレットの置かれる入力欄である
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

// NewLanguage is one choice in the primary-language select: the value it
// submits, the language as a page shows it, and whether it is the selection the
// form opens with.
//
// Value is kept beside Language because what the option submits is the thread
// language itself, which the presentation form no longer holds: a language
// resolving to no display language carries neither a name nor a tag, and the
// option would then submit nothing.
//
// [Ja] NewLanguage は主言語の select の 1 つの選択肢です。送信する値、ページが見せる形の
// 言語、そしてフォームが開いたときに選ばれているかどうかを持ちます。
//
// Value を Language の傍らに持つのは、選択肢が送信するのがスレッド言語そのものであり、
// 表示用の形はそれを保持していないためです。どの表示言語にも解決しない言語は名前もタグも
// 持たず、その選択肢は何も送信しないことになります。
type NewLanguage struct {
	Value    string
	Language viewmodel.ThreadLanguage
	Selected bool
}

// NewHeadingID is the id of this page's main heading. The community layout
// points the <main> landmark at it with aria-labelledby, so the region's
// accessible name and the heading a sighted visitor reads are the same text.
//
// [Ja] NewHeadingID はこのページの主見出しの id です。コミュニティレイアウトが
// aria-labelledby で <main> ランドマークをこれに向けるため、領域のアクセシブルな名前と、
// 目で見る訪問者が読む見出しが同じ文字列になります。
const NewHeadingID = "thread-new-heading"
