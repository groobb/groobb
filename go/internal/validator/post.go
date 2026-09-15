package validator

import (
	"context"
	"unicode/utf8"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
)

// PostBodyMaxLengthは投稿本文の最大の長さで、Unicodeのコードポイント数で数え
// ます。
//
// 数えるのは読み手が文字と呼ぶものではなくコードポイントであるため、複数から成る1つの
// 字形はその成り立ちの分を消費します。フォームはこの上限をmaxlengthに設定するのでは
// なく文言で伝えます。属性が数えるのはUTF-16のコード単位であり、この上限が受け付ける
// 本文を止めてしまうためです。フォームから知れる必要があるのは、サーバーが拒否に使う
// 数のほうです。
const PostBodyMaxLength = 10000

// PostCreateValidatorは返信フォームを検証します。本文がアプリケーションの保持
// できるテキストであり、何かを述べており、PostBodyMaxLength以内であることです。
//
// 依存を持たないのは、返信が受け付けられるかを決める他のこと (誰が投稿しているか、
// スレッドがまだ投稿を受け付けるか、その投稿者が最後に投稿してからどれだけ経ったか) が、
// いずれもフォームから読めないためです。それらはUseCaseが答えるものであり、UseCaseは
// 並行する投稿がすり抜けられない場所でそれに答えます。
type PostCreateValidator struct{}

// NewPostCreateValidatorはPostCreateValidatorを生成します。
func NewPostCreateValidator() *PostCreateValidator {
	return &PostCreateValidator{}
}

// PostCreateValidatorInputはPostCreateValidator.Validateの入力です。
type PostCreateValidatorInput struct {
	Body string
}

// Validateは送信された本文を検証し、保存される形にしたものを返します。問題が
// あれば、それを名指す *model.ValidationErrorを返します。
//
// 正規化した本文を呼び出し元に導かせず返すのは、測られたテキストと保存されるテキストを
// 同じものにするためです。
func (v *PostCreateValidator) Validate(ctx context.Context, input PostCreateValidatorInput) (string, error) {
	ve := model.NewValidationError()

	body := validatePostBody(ctx, ve, input.Body)

	if ve.HasErrors() {
		return "", ve
	}

	return body, nil
}

// validatePostBodyは、投稿本文がどこに書かれるとき (新しいスレッドの最初の投稿
// でも、返信でも) にも従う規則でbodyを検証し、見つけたものをveに追加して、本文を
// 保存される形にして返します。返り値が意味を持つのは、veにエラーが増えなかったとき
// だけです。
//
// 2つのフォームがそれぞれ規則を述べるのではなくこれを共有するのは、スレッドの最初の
// 投稿が投稿であるからです。一方のフォームが受け付ける本文は他方も受け付けるものであり、
// 2度書き下された規則はやがて2つのことを述べます。
func validatePostBody(ctx context.Context, ve *model.ValidationError, body string) string {
	normalized := model.NormalizeLineBreaks(body)

	if !isWellFormedText(normalized) {
		ve.AddField("body", i18n.T(ctx, "validation_text_invalid_characters"))
		return ""
	}

	// 本文の前後の空白を保つのは、字下げや段落の間の空行が本文の読まれ方の一部で
	// あるためです。見える文字が1つも無い本文は空のものとして拒否します。それは何も述べない
	// 投稿であり、返ってこないレス番号を1つ消費してしまいます。
	switch {
	case !hasVisibleChar(normalized):
		ve.AddField("body", i18n.T(ctx, "validation_required"))
	case utf8.RuneCountInString(normalized) > PostBodyMaxLength:
		ve.AddField("body", i18n.T(ctx, "validation_post_body_too_long"))
	}

	return normalized
}
