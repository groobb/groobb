package validator

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
)

// ModerationReasonMaxLength is the maximum length of the reason recorded with a
// moderation operation, counted in Unicode code points.
//
// The reason is read by the administrators who come after the one who wrote it,
// in a table listing one operation per row. The limit leaves room for what the
// operation was about without letting a row grow into a document that the table
// can no longer show alongside the others.
//
// [Ja] ModerationReasonMaxLengthは、モデレーションの操作に記録する理由の最大の長さで、
// Unicodeのコードポイント数で数えます。
//
// 理由を読むのは、それを書いた管理者の後に来る管理者であり、読む場所は1行に1件の操作を
// 並べた表です。この上限は、その操作が何についてのものだったかを述べる余地を残しつつ、
// 1行が、表が他の行と並べて示せなくなるような文書に育つことを防ぎます。
const ModerationReasonMaxLength = 500

// ModerationLogCreateValidator validates the reason submitted with a moderation
// operation: that it is text the application can hold and is within
// ModerationReasonMaxLength.
//
// It has no dependencies because nothing else that decides whether the operation
// is carried out can be read from the form: whether the actor may perform it,
// and whether the target is still in the state the operation is about, are the
// UseCase's to answer.
//
// [Ja] ModerationLogCreateValidatorは、モデレーションの操作とともに送信された理由を
// 検証します。アプリケーションの保持できるテキストであり、ModerationReasonMaxLength
// 以内であることです。
//
// 依存を持たないのは、その操作が実行されるかを決める他のこと (操作者がそれを行ってよいか、
// 対象がまだその操作の対象となる状態にあるか) が、いずれもフォームから読めないためです。
// それらはUseCaseが答えるものです。
type ModerationLogCreateValidator struct{}

// NewModerationLogCreateValidator creates a ModerationLogCreateValidator.
//
// [Ja] NewModerationLogCreateValidatorはModerationLogCreateValidatorを生成します。
func NewModerationLogCreateValidator() *ModerationLogCreateValidator {
	return &ModerationLogCreateValidator{}
}

// ModerationLogCreateValidatorInput is the input to
// ModerationLogCreateValidator.Validate.
//
// [Ja] ModerationLogCreateValidatorInputはModerationLogCreateValidator.Validateの
// 入力です。
type ModerationLogCreateValidatorInput struct {
	Reason string
}

// Validate checks the submitted reason and returns it in the form it is recorded
// in, or a *model.ValidationError naming what is wrong with it.
//
// An empty reason is admitted, and so is one holding nothing but whitespace,
// which is recorded as empty. The reason says why an operation was performed,
// and requiring one would be answered by a full stop as readily as by a reason:
// what the history has to carry is the operation, which it carries either way.
//
// The surrounding whitespace is removed, unlike in a post body, where the blank
// lines between paragraphs are part of how it reads. A reason is a note in a
// table cell, so leading blank lines would only push its first word out of view.
//
// [Ja] Validateは送信された理由を検証し、記録される形にしたものを返します。問題があれば、
// それを名指す*model.ValidationErrorを返します。
//
// 空の理由を受け付け、空白だけの理由も受け付けて空として記録します。理由が述べるのは
// 操作が行われた理由であり、必須にしても、理由と同じ手軽さで句点ひとつが返ってくるだけです。
// 履歴が運ばなければならないのは操作そのものであり、それはどちらの場合も運ばれます。
//
// 前後の空白は、段落の間の空行が読まれ方の一部である投稿本文と違い、取り除きます。理由は
// 表のセルに置かれる短い注記であるため、先頭の空行は最初の語を見えない場所へ押しやる
// だけになります。
func (v *ModerationLogCreateValidator) Validate(ctx context.Context, input ModerationLogCreateValidatorInput) (string, error) {
	ve := model.NewValidationError()

	reason := strings.TrimSpace(model.NormalizeLineBreaks(input.Reason))

	switch {
	case !isWellFormedText(reason):
		ve.AddField("reason", i18n.T(ctx, "validation_text_invalid_characters"))
	case utf8.RuneCountInString(reason) > ModerationReasonMaxLength:
		ve.AddField("reason", i18n.T(ctx, "validation_moderation_reason_too_long"))
	}

	if ve.HasErrors() {
		return "", ve
	}

	return reason, nil
}
