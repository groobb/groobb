package components

import (
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/templates"
)

// PostFormData is the form a post is written in: where it is submitted, what
// stands in it, and what is wrong with what was submitted last.
//
// The same form ends a thread that is being read and stands on the page a
// refused submission comes back on. Drawing it from one component keeps the
// field, its hints and the interval a visitor is told about identical in both
// places, so a reply that was refused is corrected in the form it was written
// in.
//
// [Ja] PostFormData は投稿を書くフォームです。どこへ送信するか、そこに何が入っているか、
// そして直前の送信の何が問題だったかを持ちます。
//
// 同じフォームが、読まれているスレッドの末尾に立ち、拒否された送信が戻ってくるページにも
// 立ちます。1 つのコンポーネントから描くことで、入力欄・そのヒント・訪問者に伝える間隔が
// 両方で同一に保たれ、拒否された返信は、それが書かれたのと同じフォームで直されます。
type PostFormData struct {
	CSRFToken string

	// Action is where the form posts. A reply belongs to the thread it is
	// written in, so the address names that thread and the form cannot add a
	// post to another one.
	//
	// [Ja] Action はフォームの送信先です。返信はそれが書かれたスレッドに属するため、
	// アドレスがそのスレッドを名指し、フォームが別のスレッドへ投稿を加えることは
	// できません。
	Action templates.Path

	// Body is what stands in the field: empty on the form that ends a thread, and
	// what was submitted on a form that came back refused, so nothing has to be
	// typed twice.
	//
	// [Ja] Body は入力欄に入っているものです。スレッドの末尾に立つフォームでは空で、
	// 拒否されて戻ってきたフォームでは送信された内容です。同じものを 2 度打たずに
	// 済むようにするためです。
	Body string

	// PostIntervalSeconds is how long one person waits between posts, in whole
	// seconds. The form states it so that the wait is known before a submission
	// is refused for arriving too soon after the last one.
	//
	// [Ja] PostIntervalSeconds は 1 人が投稿と投稿の間に置く時間で、整数秒です。フォームが
	// これを述べるのは、前の投稿から間もない送信が拒否される前に、待ち時間が分かるように
	// するためです。
	PostIntervalSeconds int

	// Errors are the messages about the last submission: the body's own when
	// there is something in it to fix, and form-wide ones when what was refused
	// is the submission rather than the field. It is nil on a form that has not
	// been submitted.
	//
	// [Ja] Errors は直前の送信についてのメッセージです。本文の中に直すものがあるときは
	// 本文自身のメッセージを、拒否されたのがフィールドではなく送信そのものであるときは
	// フォーム全体のメッセージを持ちます。まだ送信されていないフォームでは nil です。
	Errors *model.ValidationError
}

// AutofocusBody reports whether the field opens with the caret in it. It does
// when the body itself is what has to change, and yields to the summary above
// the form otherwise: a submission refused as a whole is not corrected by
// editing a field that is not at fault, and one that was never submitted is a
// visitor reading a thread rather than answering a refusal.
//
// [Ja] AutofocusBody は、入力欄がキャレットを置いた状態で開くかどうかを返します。本文
// そのものを変える必要があるときにそうし、それ以外ではフォームの上の要約に道を譲ります。
// 全体として拒否された送信は、落ち度の無いフィールドを直すことでは解決せず、そもそも
// 送信されていないフォームの前にいるのは、拒否に答えている人ではなくスレッドを読んで
// いる訪問者だからです。
func (d PostFormData) AutofocusBody() bool {
	return !d.Errors.HasGlobalError() && d.Errors.HasErrors()
}

// postFormFields is the form's single control. It is written out as a list all
// the same, because that is what the summary above the form reads, and a reply
// form that grew a second field would otherwise list only the first.
//
// [Ja] postFormFields はこのフォームの唯一の入力欄です。それでも一覧として書き出すのは、
// フォームの上の要約が読むものがこれであり、返信フォームが 2 つ目のフィールドを持ったとき
// に最初の 1 つしか並ばなくなることを避けるためです。
var postFormFields = []FormErrorSummaryField{
	{Name: "body", LabelKey: "post_body_label"},
}

// ErrorSummary returns what the summary above the form is drawn from: the reply
// form's own heading, its control, and the messages about the last submission.
//
// [Ja] ErrorSummary は、フォームの上の要約を描くためのデータを返します。返信フォーム自身の
// 見出し、その入力欄、そして直前の送信についてのメッセージです。
func (d PostFormData) ErrorSummary() FormErrorSummaryData {
	return FormErrorSummaryData{
		HeadingKey: "post_errors_heading",
		Fields:     postFormFields,
		Errors:     d.Errors,
	}
}
