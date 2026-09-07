package components

import (
	"fmt"
	"strings"

	"github.com/groobb/groobb/go/internal/model"
)

// fieldErrorID returns the DOM id for the i-th error message of a field, e.g.
// "email-error-0". FieldErrors stamps it on each <p>, and the control lists
// every id in aria-describedby (via FieldErrorsDescribedBy) so that all
// messages, not just the first, are exposed as the control's description.
//
// [Ja] fieldErrorID はフィールドの i 番目のエラーメッセージ用の DOM id を返します
// (例: "email-error-0")。FieldErrors が各 <p> に付与し、入力欄は
// (FieldErrorsDescribedBy 経由で) aria-describedby に全 id を並べるため、先頭だけでなく
// すべてのメッセージが入力欄の説明として公開されます。
func fieldErrorID(field string, i int) string {
	return fmt.Sprintf("%s-error-%d", field, i)
}

// FieldErrorsDescribedBy returns the space-separated list of a field's
// error-message ids, for use as a control's aria-describedby value. It returns
// "" when the field has no errors, so callers render aria-describedby only when
// HasFieldError reports an error.
//
// [Ja] FieldErrorsDescribedBy はフィールドのエラーメッセージ id を空白区切りで並べた
// 文字列を返し、入力欄の aria-describedby の値として使います。フィールドにエラーが無い
// ときは "" を返すため、呼び出し側は HasFieldError がエラーを報告するときだけ
// aria-describedby を描画します。
func FieldErrorsDescribedBy(field string, formErrors *model.ValidationError) string {
	messages := formErrors.GetFieldErrors(field)
	if len(messages) == 0 {
		return ""
	}

	ids := make([]string, len(messages))
	for i := range messages {
		ids[i] = fieldErrorID(field, i)
	}
	return strings.Join(ids, " ")
}

// FormErrorSummaryField is one of a form's controls as the summary addresses it:
// the name it submits under, which is also the id the summary links to, and the
// key of the label naming it.
//
// The label key is carried here rather than derived from the name, so that every
// key a page renders is written out as a literal. A key built by concatenation
// is not found by searching the locale files, and a control added without its
// label would render the key itself, since a missing translation falls back to
// the message id.
//
// [Ja] FormErrorSummaryField は、要約が扱う形でのフォームの入力欄の 1 つです。送信する
// ときの名前 (要約がリンクする id でもあります) と、それを名指すラベルのキーを持ちます。
//
// ラベルのキーを名前から導かずここに持つのは、ページが描画するどのキーもリテラルとして
// 書き出されるようにするためです。連結で組み立てたキーはロケールファイルを検索しても
// 見つからず、ラベルを伴わずに追加された入力欄はキーそのものを描画します。翻訳が
// 見つからないときはメッセージ id にフォールバックするためです。
type FormErrorSummaryField struct {
	Name     string
	LabelKey string
}

// FormErrorSummaryData is what the summary above a form is drawn from: the
// heading naming it, the form's controls, and the messages about the last
// submission.
//
// [Ja] FormErrorSummaryData は、フォームの上の要約を描くためのデータです。それを名指す
// 見出し、フォームの入力欄、そして直前の送信についてのメッセージを持ちます。
type FormErrorSummaryData struct {
	// HeadingKey is the key of the heading the summary opens with. Each form names
	// its own, so what a visitor reads belongs to the form in front of them rather
	// than to forms in general.
	//
	// [Ja] HeadingKey は、要約が始まる見出しのキーです。フォームはそれぞれ自身の見出しを
	// 名指すため、訪問者が読むものは、フォーム一般ではなく目の前のフォームのものになります。
	HeadingKey string

	// Fields are the form's controls in the order they are read. The list follows
	// this order rather than the errors, so it reads down the form instead of in
	// whichever order the messages happened to be built.
	//
	// [Ja] Fields はフォームの入力欄を、読まれる順に並べたものです。一覧はエラーではなく
	// この順に従うため、メッセージがたまたま組み立てられた順ではなく、フォームを上から
	// 下へ辿る形で読まれます。
	Fields []FormErrorSummaryField

	// Errors are the messages about the last submission. It is nil on a form that
	// has not been submitted.
	//
	// [Ja] Errors は直前の送信についてのメッセージです。まだ送信されていないフォームでは
	// nil です。
	Errors *model.ValidationError
}

// HasEntries reports whether any of the listed fields has something to say, which
// is what decides whether the summary is drawn at all. A form-wide message is not
// one of them: it stands above the summary on its own, and a summary drawn for it
// would be a heading over an empty list.
//
// [Ja] HasEntries は、並べる対象のフィールドのいずれかが述べることを持つかどうかを返し
// ます。そもそも要約を描くかどうかを決めるものがこれです。フォーム全体のメッセージは
// この対象ではありません。それは要約の上に単独で立つものであり、そのために描かれた要約は
// 空の一覧に載った見出しになってしまいます。
func (d FormErrorSummaryData) HasEntries() bool {
	for _, field := range d.Fields {
		if d.Errors.HasFieldError(field.Name) {
			return true
		}
	}
	return false
}
