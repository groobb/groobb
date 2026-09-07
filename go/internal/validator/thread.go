package validator

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
)

// ThreadTitleMaxLength is the maximum length of a thread title, counted in
// Unicode code points, as PostBodyMaxLength counts a body.
//
// [Ja] ThreadTitleMaxLength はスレッドのタイトルの最大の長さで、PostBodyMaxLength が
// 本文を数えるのと同じく、Unicodeのコードポイント数で数えます。
const ThreadTitleMaxLength = 100

// ThreadCreateValidator validates the thread-creation form: a title that names
// the conversation on one line, a primary language the thread may be written
// in, and a first post that follows the same rules as any other post.
//
// [Ja] ThreadCreateValidator はスレッド作成フォームを検証します。会話を1行で名指す
// タイトル、スレッドを書ける主言語、そして他の投稿と同じ規則に従う最初の投稿です。
type ThreadCreateValidator struct{}

// NewThreadCreateValidator creates a ThreadCreateValidator.
//
// [Ja] NewThreadCreateValidator は ThreadCreateValidator を生成します。
func NewThreadCreateValidator() *ThreadCreateValidator {
	return &ThreadCreateValidator{}
}

// ThreadCreateValidatorInput is the input to ThreadCreateValidator.Validate.
// Language is the raw submitted value rather than a model.ThreadLanguage,
// because a select can be submitted with anything in it and what arrives is not
// yet known to name a language.
//
// [Ja] ThreadCreateValidatorInput は ThreadCreateValidator.Validate の入力です。
// Language が model.ThreadLanguage ではなく送信された生の値なのは、select は何を入れて
// でも送信でき、届いた値が言語を名指すものだとまだ分かっていないためです。
type ThreadCreateValidatorInput struct {
	Title    string
	Language string
	Body     string
}

// ThreadCreateValidateOutput is what a valid thread-creation form amounts to:
// the values as they are stored, with the language now a model.ThreadLanguage.
//
// [Ja] ThreadCreateValidateOutput は、妥当なスレッド作成フォームが行き着く先です。
// 保存される形の値であり、言語は model.ThreadLanguage になっています。
type ThreadCreateValidateOutput struct {
	Title    string
	Language model.ThreadLanguage
	Body     string
}

// Validate checks the submitted title, primary language and first-post body,
// returning them in the form they are stored in, or a *model.ValidationError
// naming what is wrong with them.
//
// Every field is checked before any is reported, so a form comes back saying
// everything that has to change rather than one thing at a time.
//
// [Ja] Validate は送信されたタイトル・主言語・最初の投稿の本文を検証し、保存される形に
// したものを返します。問題があれば、それを名指す *model.ValidationError を返します。
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

// validateThreadTitle checks the submitted title, adding what it finds to ve,
// and returns it in the form it is stored in.
//
// Unlike a body, a title is trimmed rather than kept as it was typed: it is
// read in a list of threads, where leading space would push one row's title out
// of line with the rest, and it names the thread rather than saying anything, so
// the space around it carries nothing. A line break is refused instead of being
// folded away, because a title arriving with one was not typed into the form the
// application drew, and the checks after it are skipped so that one title does not
// come back with two things to fix.
//
// [Ja] validateThreadTitle は送信されたタイトルを検証し、見つけたものを ve に追加して、
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

// validateThreadLanguage checks the submitted primary language against the
// languages a thread may be written in, adding what it finds to ve, and returns
// the language it names.
//
// The form offers exactly these languages, so anything else was not chosen from
// it. Refusing rather than falling back to the language the page is drawn in
// keeps a thread from being labelled with a language nobody chose for it, which
// is what its badge and the lang on its title would then declare.
//
// [Ja] validateThreadLanguage は送信された主言語を、スレッドを書ける言語と照合し、
// 見つけたものを ve に追加して、その値が名指す言語を返します。
//
// フォームが差し出すのはまさにこれらの言語であるため、それ以外はフォームから選ばれた
// ものではありません。ページが描かれている言語へ落とさずに拒否するのは、誰も選んで
// いない言語がスレッドに付くのを防ぐためです。付いてしまえば、バッジとタイトルの lang
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
