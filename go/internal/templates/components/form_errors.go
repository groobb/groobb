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
