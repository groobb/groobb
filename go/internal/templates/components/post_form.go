package components

import (
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/templates"
)

// PostFormDataは投稿を書くフォームです。どこへ送信するか、そこに何が入っているか、
// そして直前の送信の何が問題だったかを持ちます。
//
// 同じフォームが、読まれているスレッドの末尾に立ち、拒否された送信が戻ってくるページにも
// 立ちます。1つのコンポーネントから描くことで、入力欄・そのヒント・訪問者に伝える間隔が
// 両方で同一に保たれ、拒否された返信は、それが書かれたのと同じフォームで直されます。
type PostFormData struct {
	CSRFToken string

	// Actionはフォームの送信先です。返信はそれが書かれたスレッドに属するため、
	// アドレスがそのスレッドを名指し、フォームが別のスレッドへ投稿を加えることは
	// できません。
	Action templates.Path

	// Bodyは入力欄に入っているものです。スレッドの末尾に立つフォームでは空で、
	// 拒否されて戻ってきたフォームでは送信された内容です。同じものを2度打たずに
	// 済むようにするためです。
	Body string

	// PostIntervalSecondsは1人が投稿と投稿の間に置く時間で、整数秒です。フォームが
	// これを述べるのは、前の投稿から間もない送信が拒否される前に、待ち時間が分かるように
	// するためです。
	PostIntervalSeconds int

	// Errorsは直前の送信についてのメッセージです。本文の中に直すものがあるときは
	// 本文自身のメッセージを、拒否されたのがフィールドではなく送信そのものであるときは
	// フォーム全体のメッセージを持ちます。まだ送信されていないフォームではnilです。
	Errors *model.ValidationError
}

// AutofocusBodyは、入力欄がキャレットを置いた状態で開くかどうかを返します。本文
// そのものを変える必要があるときにそうし、それ以外ではフォームの上の要約に道を譲ります。
// 全体として拒否された送信は、落ち度の無いフィールドを直すことでは解決せず、そもそも
// 送信されていないフォームの前にいるのは、拒否に答えている人ではなくスレッドを読んで
// いる訪問者だからです。
func (d PostFormData) AutofocusBody() bool {
	return !d.Errors.HasGlobalError() && d.Errors.HasErrors()
}

// postFormFieldsはこのフォームの唯一の入力欄です。それでも一覧として書き出すのは、
// フォームの上の要約が読むものがこれであり、返信フォームが2つ目のフィールドを持ったとき
// に最初の1つしか並ばなくなることを避けるためです。
var postFormFields = []FormErrorSummaryField{
	{Name: "body", LabelKey: "post_body_label"},
}

// ErrorSummaryは、フォームの上の要約を描くためのデータを返します。返信フォーム自身の
// 見出し、その入力欄、そして直前の送信についてのメッセージです。
func (d PostFormData) ErrorSummary() FormErrorSummaryData {
	return FormErrorSummaryData{
		HeadingKey: "post_errors_heading",
		Fields:     postFormFields,
		Errors:     d.Errors,
	}
}
