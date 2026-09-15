package validator

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
)

// ThreadTitleMaxLengthはスレッドのタイトルの最大の長さで、PostBodyMaxLengthが
// 本文を数えるのと同じく、Unicodeのコードポイント数で数えます。
const ThreadTitleMaxLength = 100

// ThreadCreateValidatorはスレッド作成フォームを検証します。会話を1行で名指す
// タイトル、スレッドを書ける主言語、そして他の投稿と同じ規則に従う最初の投稿です。
type ThreadCreateValidator struct{}

// NewThreadCreateValidatorはThreadCreateValidatorを生成します。
func NewThreadCreateValidator() *ThreadCreateValidator {
	return &ThreadCreateValidator{}
}

// ThreadCreateValidatorInputはThreadCreateValidator.Validateの入力です。
// Languageがmodel.ThreadLanguageではなく送信された生の値なのは、selectは何を入れて
// でも送信でき、届いた値が言語を名指すものだとまだ分かっていないためです。
type ThreadCreateValidatorInput struct {
	Title    string
	Language string
	Body     string
}

// ThreadCreateValidateOutputは、妥当なスレッド作成フォームが行き着く先です。
// 保存される形の値であり、言語はmodel.ThreadLanguageになっています。
type ThreadCreateValidateOutput struct {
	Title    string
	Language model.ThreadLanguage
	Body     string
}

// Validateは送信されたタイトル・主言語・最初の投稿の本文を検証し、保存される形に
// したものを返します。問題があれば、それを名指す *model.ValidationErrorを返します。
//
// どれか1つを報告する前にすべてのフィールドを検証するため、フォームは直すべきことを
// 1つずつではなく、まとめて伝えて戻ってきます。
func (v *ThreadCreateValidator) Validate(ctx context.Context, input ThreadCreateValidatorInput) (*ThreadCreateValidateOutput, error) {
	ve := model.NewValidationError()

	title := validateThreadTitle(ctx, ve, input.Title)
	language := validateThreadLanguage(ctx, ve, input.Language)
	body := validatePostBody(ctx, ve, input.Body)

	if ve.HasErrors() {
		return nil, ve
	}

	return &ThreadCreateValidateOutput{
		Title:    title,
		Language: language,
		Body:     body,
	}, nil
}

// validateThreadTitleは送信されたタイトルを検証し、見つけたものをveに追加して、
// 保存される形にしたものを返します。
//
// 本文と違ってタイトルを打たれたまま保たずに前後を切り詰めるのは、タイトルがスレッドの
// 一覧の中で読まれるものであり、先頭の空白があるとその行のタイトルだけが他とずれるため
// です。またタイトルはスレッドを名指すものであって何かを述べるものではないため、周りの
// 空白は何も運びません。改行は畳み込まずに拒否します。改行を伴って届いたタイトルは、
// アプリケーションが描いたフォームに打ち込まれたものではないためです。それ以降の検査は
// 行いません。1つのタイトルが直すべきことを2つ抱えて戻ってくるのを避けるためです。
func validateThreadTitle(ctx context.Context, ve *model.ValidationError, title string) string {
	normalized := model.NormalizeLineBreaks(title)

	if !isWellFormedText(normalized) {
		ve.AddField("title", i18n.T(ctx, "validation_text_invalid_characters"))
		return ""
	}

	if strings.Contains(normalized, "\n") {
		ve.AddField("title", i18n.T(ctx, "validation_thread_title_single_line"))
		return ""
	}

	normalized = strings.TrimSpace(normalized)
	if !hasVisibleChar(normalized) {
		ve.AddField("title", i18n.T(ctx, "validation_required"))
		return ""
	}

	if utf8.RuneCountInString(normalized) > ThreadTitleMaxLength {
		ve.AddField("title", i18n.T(ctx, "validation_thread_title_too_long"))
	}

	return normalized
}

// validateThreadLanguageは送信された主言語を、スレッドを書ける言語と照合し、
// 見つけたものをveに追加して、その値が名指す言語を返します。
//
// フォームが差し出すのはまさにこれらの言語であるため、それ以外はフォームから選ばれた
// ものではありません。ページが描かれている言語へ落とさずに拒否するのは、誰も選んで
// いない言語がスレッドに付くのを防ぐためです。付いてしまえば、バッジとタイトルのlang
// 属性がその言語を名乗ることになります。
func validateThreadLanguage(ctx context.Context, ve *model.ValidationError, language string) model.ThreadLanguage {
	if language == "" {
		ve.AddField("language", i18n.T(ctx, "validation_required"))
		return ""
	}

	threadLanguage := model.ThreadLanguage(language)
	if !threadLanguage.IsValid() {
		ve.AddField("language", i18n.T(ctx, "validation_thread_language_invalid"))
		return ""
	}

	return threadLanguage
}
