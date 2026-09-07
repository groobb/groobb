package validator

import (
	"context"
	"unicode/utf8"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
)

// PostBodyMaxLength is the maximum length of a post body, counted in Unicode
// code points.
//
// The count is of code points rather than of what a reader would call
// characters, so a glyph built out of several of them costs what it is made of.
// The form states the limit instead of setting maxlength to it: that attribute
// counts UTF-16 code units, which would stop a body this limit accepts, and the
// count the server refuses on is the one that has to be knowable from the form.
//
// [Ja] PostBodyMaxLength は投稿本文の最大の長さで、Unicodeのコードポイント数で数え
// ます。
//
// 数えるのは読み手が文字と呼ぶものではなくコードポイントであるため、複数から成る1つの
// 字形はその成り立ちの分を消費します。フォームはこの上限を maxlength に設定するのでは
// なく文言で伝えます。属性が数えるのはUTF-16のコード単位であり、この上限が受け付ける
// 本文を止めてしまうためです。フォームから知れる必要があるのは、サーバーが拒否に使う
// 数のほうです。
const PostBodyMaxLength = 10000

// PostCreateValidator validates the reply form: that the body is text the
// application can hold, says something, and is within PostBodyMaxLength.
//
// It has no dependencies because nothing else that decides whether a reply is
// taken can be read from the form: who is posting, whether the thread still
// accepts posts, and how long ago its author last posted are the UseCase's to
// answer, and it answers them where a concurrent post cannot slip past.
//
// [Ja] PostCreateValidator は返信フォームを検証します。本文がアプリケーションの保持
// できるテキストであり、何かを述べており、PostBodyMaxLength 以内であることです。
//
// 依存を持たないのは、返信が受け付けられるかを決める他のこと (誰が投稿しているか、
// スレッドがまだ投稿を受け付けるか、その投稿者が最後に投稿してからどれだけ経ったか) が、
// いずれもフォームから読めないためです。それらはUseCaseが答えるものであり、UseCaseは
// 並行する投稿がすり抜けられない場所でそれに答えます。
type PostCreateValidator struct{}

// NewPostCreateValidator creates a PostCreateValidator.
//
// [Ja] NewPostCreateValidator は PostCreateValidator を生成します。
func NewPostCreateValidator() *PostCreateValidator {
	return &PostCreateValidator{}
}

// PostCreateValidatorInput is the input to PostCreateValidator.Validate.
//
// [Ja] PostCreateValidatorInput は PostCreateValidator.Validate の入力です。
type PostCreateValidatorInput struct {
	Body string
}

// Validate checks the submitted body and returns it in the form it is stored
// in, or a *model.ValidationError naming what is wrong with it.
//
// The normalized body is returned rather than left for the caller to derive,
// so that the text that was measured is the text that is saved.
//
// [Ja] Validate は送信された本文を検証し、保存される形にしたものを返します。問題が
// あれば、それを名指す *model.ValidationError を返します。
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

// validatePostBody checks body under the rules a post body follows wherever it
// is written, as the first post of a new thread and as a reply alike. It adds
// what it finds to ve and returns the body in the form it is stored in; the
// returned value stands only when ve gained no error.
//
// The two forms share this rather than each stating the rule, because the first
// post of a thread is a post: a body one form takes is one the other takes, and
// a rule written down twice ends up saying two things.
//
// [Ja] validatePostBody は、投稿本文がどこに書かれるとき (新しいスレッドの最初の投稿
// でも、返信でも) にも従う規則で body を検証し、見つけたものを ve に追加して、本文を
// 保存される形にして返します。返り値が意味を持つのは、ve にエラーが増えなかったとき
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

	// The whitespace around a body is kept, because indentation and the blank
	// lines between paragraphs are part of how it reads. A body with nothing
	// visible in it is refused as an empty one: it is a post that says nothing,
	// and it would take a reply number that is never given back.
	//
	// [Ja] 本文の前後の空白を保つのは、字下げや段落の間の空行が本文の読まれ方の一部で
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
