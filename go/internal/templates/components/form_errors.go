package components

import (
	"fmt"
	"strings"

	"github.com/groobb/groobb/go/internal/model"
)

// fieldErrorIDはフィールドのi番目のエラーメッセージ用のDOM idを返します
// (例: "email-error-0")。FieldErrorsが各 <p> に付与し、入力欄は
// (FieldErrorsDescribedBy経由で) aria-describedbyに全idを並べるため、先頭だけでなく
// すべてのメッセージが入力欄の説明として公開されます。
func fieldErrorID(field string, i int) string {
	return fmt.Sprintf("%s-error-%d", field, i)
}

// FieldErrorsDescribedByはフィールドのエラーメッセージidを空白区切りで並べた
// 文字列を返し、入力欄のaria-describedbyの値として使います。フィールドにエラーが無い
// ときは "" を返すため、呼び出し側はHasFieldErrorがエラーを報告するときだけ
// aria-describedbyを描画します。
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

// FormErrorSummaryFieldは、要約が扱う形でのフォームの入力欄の1つです。送信する
// ときの名前 (要約がリンクするidでもあります) と、それを名指すラベルのキーを持ちます。
//
// ラベルのキーを名前から導かずここに持つのは、ページが描画するどのキーもリテラルとして
// 書き出されるようにするためです。連結で組み立てたキーはロケールファイルを検索しても
// 見つからず、ラベルを伴わずに追加された入力欄はキーそのものを描画します。翻訳が
// 見つからないときはメッセージidにフォールバックするためです。
type FormErrorSummaryField struct {
	Name     string
	LabelKey string
}

// FormErrorSummaryDataは、フォームの上の要約を描くためのデータです。それを名指す
// 見出し、フォームの入力欄、そして直前の送信についてのメッセージを持ちます。
type FormErrorSummaryData struct {
	// HeadingKeyは、要約が始まる見出しのキーです。フォームはそれぞれ自身の見出しを
	// 名指すため、訪問者が読むものは、フォーム一般ではなく目の前のフォームのものになります。
	HeadingKey string

	// Fieldsはフォームの入力欄を、読まれる順に並べたものです。一覧はエラーではなく
	// この順に従うため、メッセージがたまたま組み立てられた順ではなく、フォームを上から
	// 下へ辿る形で読まれます。
	Fields []FormErrorSummaryField

	// Errorsは直前の送信についてのメッセージです。まだ送信されていないフォームでは
	// nilです。
	Errors *model.ValidationError
}

// HasEntriesは、並べる対象のフィールドのいずれかが述べることを持つかどうかを返し
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
