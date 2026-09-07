package validator_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/validator"
)

// validThreadCreateInput returns a thread-creation form every field of which is
// valid, so that a test case states the one field it is about.
//
// [Ja] validThreadCreateInput は全フィールドが妥当なスレッド作成フォームを返す。テスト
// ケースが、そのケースの対象である1つのフィールドだけを述べられるようにするため。
func validThreadCreateInput() validator.ThreadCreateValidatorInput {
	return validator.ThreadCreateValidatorInput{
		Title:    "はじめてのスレッド",
		Language: string(model.LocaleJa.ThreadLanguage()),
		Body:     "最初の投稿の本文です。",
	}
}

// TestThreadCreateValidator_Validate covers the title, the primary language and
// the first post together: a form valid in every field is accepted, including
// at the length boundaries and where emoji make the code-point count differ
// from the byte count, while each invalid field is reported against the field
// it belongs to, and a form invalid in several is reported for all of them at
// once.
//
// [Ja] TestThreadCreateValidator_Validate はタイトル・主言語・最初の投稿をまとめて
// 網羅する。全フィールドが妥当なフォームは、長さの境界ちょうどや、絵文字によってコード
// ポイント数とバイト数が食い違う場合も含めて受け付けられ、不正なフィールドはそれぞれが
// 属するフィールドに対して報告され、複数が不正なフォームではそのすべてが一度に報告される。
func TestThreadCreateValidator_Validate(t *testing.T) {
	t.Parallel()

	v := validator.NewThreadCreateValidator()

	tests := []struct {
		name       string
		input      validator.ThreadCreateValidatorInput
		wantFields []string
	}{
		{
			name:  "正常系: タイトル・主言語・本文",
			input: validThreadCreateInput(),
		},
		{
			name: "正常系: 境界の100文字タイトル",
			input: validator.ThreadCreateValidatorInput{
				Title:    strings.Repeat("あ", validator.ThreadTitleMaxLength),
				Language: string(model.LocaleJa.ThreadLanguage()),
				Body:     "本文",
			},
		},
		{
			name: "正常系: 絵文字はコードポイント数で数える",
			input: validator.ThreadCreateValidatorInput{
				Title:    strings.Repeat("😀", validator.ThreadTitleMaxLength),
				Language: string(model.LocaleJa.ThreadLanguage()),
				Body:     "本文",
			},
		},
		{
			name: "正常系: 境界の10,000文字の本文",
			input: validator.ThreadCreateValidatorInput{
				Title:    "タイトル",
				Language: string(model.LocaleJa.ThreadLanguage()),
				Body:     strings.Repeat("あ", validator.PostBodyMaxLength),
			},
		},
		{
			name: "正常系: 主言語が英語",
			input: validator.ThreadCreateValidatorInput{
				Title:    "A new thread",
				Language: string(model.LocaleEn.ThreadLanguage()),
				Body:     "The first post.",
			},
		},
		{
			// A thread written in a language the application has no locale for
			// says so rather than claiming one of the display languages.
			//
			// [Ja] アプリがロケールを持たない言語で書かれたスレッドは、表示言語のどれかを
			// 騙るのではなく、そのことを名乗る。
			name: "正常系: 主言語がその他",
			input: validator.ThreadCreateValidatorInput{
				Title:    "Un nouveau fil",
				Language: string(model.ThreadLanguageOther),
				Body:     "Le premier message.",
			},
		},
		{
			name:       "異常系: タイトルが空",
			input:      validator.ThreadCreateValidatorInput{Title: "", Language: string(model.LocaleJa.ThreadLanguage()), Body: "本文"},
			wantFields: []string{"title"},
		},
		{
			name:       "異常系: タイトルが空白だけ",
			input:      validator.ThreadCreateValidatorInput{Title: " 　\t", Language: string(model.LocaleJa.ThreadLanguage()), Body: "本文"},
			wantFields: []string{"title"},
		},
		{
			// A zero-width space is not whitespace, so trimming leaves it in place. A
			// title made of nothing else names nothing, and it would stand in a list of
			// threads as a row whose title is blank.
			//
			// [Ja] ゼロ幅スペースは空白ではないため、前後を切り詰めても残る。それだけで
			// できたタイトルは何も名指さず、スレッドの一覧にタイトルが空白な行として並ぶ。
			name:       "異常系: タイトルが目に見えない文字だけ",
			input:      validator.ThreadCreateValidatorInput{Title: "\u200b", Language: string(model.LocaleJa.ThreadLanguage()), Body: "本文"},
			wantFields: []string{"title"},
		},
		{
			name:       "異常系: タイトルが目に見えない文字と空白だけ",
			input:      validator.ThreadCreateValidatorInput{Title: "\u200b 　\u200b", Language: string(model.LocaleJa.ThreadLanguage()), Body: "本文"},
			wantFields: []string{"title"},
		},
		{
			name:       "異常系: タイトルが101文字",
			input:      validator.ThreadCreateValidatorInput{Title: strings.Repeat("あ", validator.ThreadTitleMaxLength+1), Language: string(model.LocaleJa.ThreadLanguage()), Body: "本文"},
			wantFields: []string{"title"},
		},
		{
			name:       "異常系: 絵文字のタイトルが101文字",
			input:      validator.ThreadCreateValidatorInput{Title: strings.Repeat("😀", validator.ThreadTitleMaxLength+1), Language: string(model.LocaleJa.ThreadLanguage()), Body: "本文"},
			wantFields: []string{"title"},
		},
		{
			name:       "異常系: タイトルに改行",
			input:      validator.ThreadCreateValidatorInput{Title: "1行目\n2行目", Language: string(model.LocaleJa.ThreadLanguage()), Body: "本文"},
			wantFields: []string{"title"},
		},
		{
			name:       "異常系: タイトルが不正なUTF-8",
			input:      validator.ThreadCreateValidatorInput{Title: "タイトル\xff", Language: string(model.LocaleJa.ThreadLanguage()), Body: "本文"},
			wantFields: []string{"title"},
		},
		{
			name:       "異常系: タイトルにNULを含む",
			input:      validator.ThreadCreateValidatorInput{Title: "タイトル\x00", Language: string(model.LocaleJa.ThreadLanguage()), Body: "本文"},
			wantFields: []string{"title"},
		},
		{
			name:       "異常系: 主言語が空",
			input:      validator.ThreadCreateValidatorInput{Title: "タイトル", Language: "", Body: "本文"},
			wantFields: []string{"language"},
		},
		{
			name:       "異常系: 主言語が選べる言語にない",
			input:      validator.ThreadCreateValidatorInput{Title: "タイトル", Language: "fr", Body: "本文"},
			wantFields: []string{"language"},
		},
		{
			name:       "異常系: 本文が空",
			input:      validator.ThreadCreateValidatorInput{Title: "タイトル", Language: string(model.LocaleJa.ThreadLanguage()), Body: ""},
			wantFields: []string{"body"},
		},
		{
			name:       "異常系: 本文が空白だけ",
			input:      validator.ThreadCreateValidatorInput{Title: "タイトル", Language: string(model.LocaleJa.ThreadLanguage()), Body: " \n"},
			wantFields: []string{"body"},
		},
		{
			name:       "異常系: 本文が10,001文字",
			input:      validator.ThreadCreateValidatorInput{Title: "タイトル", Language: string(model.LocaleJa.ThreadLanguage()), Body: strings.Repeat("あ", validator.PostBodyMaxLength+1)},
			wantFields: []string{"body"},
		},
		{
			name:       "異常系: 本文が不正なUTF-8",
			input:      validator.ThreadCreateValidatorInput{Title: "タイトル", Language: string(model.LocaleJa.ThreadLanguage()), Body: "本文\xff"},
			wantFields: []string{"body"},
		},
		{
			name:       "異常系: 3つのフィールドがすべて不正",
			input:      validator.ThreadCreateValidatorInput{Title: "", Language: "fr", Body: ""},
			wantFields: []string{"title", "language", "body"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

			output, err := v.Validate(ctx, tt.input)

			if len(tt.wantFields) == 0 {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if output == nil {
					t.Fatal("expected an output, got nil")
				}
				return
			}

			if output != nil {
				t.Errorf("expected no output on failure, got %#v", output)
			}
			ve := model.AsValidationError(err)
			if ve == nil {
				t.Fatalf("expected a ValidationError, got %v", err)
			}
			for _, field := range tt.wantFields {
				if !ve.HasFieldError(field) {
					t.Errorf("expected a field error on %s, got %#v", field, ve.Fields)
				}
			}
			if len(ve.Fields) != len(tt.wantFields) {
				t.Errorf("errors on %d fields, want %d: %#v", len(ve.Fields), len(tt.wantFields), ve.Fields)
			}
		})
	}
}

// TestThreadCreateValidator_ValidateRejectsTitleLineBreaks verifies that a line
// break is reported even at the edges of a title, where trimming whitespace
// would otherwise hide it, and that it is the only thing reported: the checks
// after it are skipped, so a title that is nothing but a line break, or one that
// is over the limit as well, comes back with a single thing to fix.
//
// [Ja] TestThreadCreateValidator_ValidateRejectsTitleLineBreaks は、空白除去で
// 見落としやすいタイトルの先頭・末尾でも改行がエラーになること、そしてそれが報告される
// 唯一のものであることを検証する。改行より後の検査は行われないため、改行だけのタイトルも、
// 長さも超過しているタイトルも、直すべきこと1つを抱えて戻ってくる。
func TestThreadCreateValidator_ValidateRejectsTitleLineBreaks(t *testing.T) {
	t.Parallel()

	v := validator.NewThreadCreateValidator()
	lineBreaks := []struct {
		name  string
		value string
	}{
		{name: "LF", value: "\n"},
		{name: "CR", value: "\r"},
		{name: "CRLF", value: "\r\n"},
	}

	for _, lineBreak := range lineBreaks {
		titles := []struct {
			name  string
			value string
		}{
			{name: "先頭", value: lineBreak.value + "タイトル"},
			{name: "中間", value: "1行目" + lineBreak.value + "2行目"},
			{name: "末尾", value: "タイトル" + lineBreak.value},
			{name: "改行のみ", value: lineBreak.value},
			{name: "長さ超過", value: strings.Repeat("あ", validator.ThreadTitleMaxLength+1) + lineBreak.value},
		}
		for _, title := range titles {
			t.Run(lineBreak.name+"/"+title.name, func(t *testing.T) {
				t.Parallel()

				ctx := i18n.SetLocale(context.Background(), model.LocaleJa)
				input := validThreadCreateInput()
				input.Title = title.value

				output, err := v.Validate(ctx, input)
				if output != nil {
					t.Errorf("expected no output on failure, got %#v", output)
				}
				ve := model.AsValidationError(err)
				if ve == nil {
					t.Fatalf("expected a ValidationError, got %v", err)
				}
				want := []string{i18n.T(ctx, "validation_thread_title_single_line")}
				if messages := ve.GetFieldErrors("title"); !slices.Equal(messages, want) {
					t.Errorf("title errors = %q, want %q", messages, want)
				}
			})
		}
	}
}

// TestThreadCreateValidator_ValidateNormalizes verifies what a valid form is
// stored as: a title with the whitespace around it removed, a body with its
// whitespace kept and its line endings unified to LF, and the submitted value
// resolved to the thread language it names.
//
// [Ja] TestThreadCreateValidator_ValidateNormalizes は、妥当なフォームが何として保存
// されるかを検証する。周りの空白を取り除いたタイトル、空白を保ち改行をLFに統一した本文、
// そして送信された値が名指すスレッド言語である。
func TestThreadCreateValidator_ValidateNormalizes(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

	v := validator.NewThreadCreateValidator()

	output, err := v.Validate(ctx, validator.ThreadCreateValidatorInput{
		Title:    "　 はじめてのスレッド 　",
		Language: string(model.ThreadLanguageOther),
		Body:     "  1行目\r\n2行目  ",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if want := "はじめてのスレッド"; output.Title != want {
		t.Errorf("Title = %q, want %q", output.Title, want)
	}
	if want := model.ThreadLanguageOther; output.Language != want {
		t.Errorf("Language = %q, want %q", output.Language, want)
	}
	if want := "  1行目\n2行目  "; output.Body != want {
		t.Errorf("Body = %q, want %q", output.Body, want)
	}
}

// TestThreadCreateValidator_ValidateTranslatesMessages verifies that every
// message this form can report is translated in every display language: a
// message that came back as its own message ID is one whose translation is
// missing from a locale file.
//
// [Ja] TestThreadCreateValidator_ValidateTranslatesMessages は、このフォームが報告
// しうるメッセージがどの表示言語でも翻訳されていることを検証する。メッセージ ID のまま
// 返ってきたメッセージは、どこかのロケールファイルで翻訳が欠けているものである。
func TestThreadCreateValidator_ValidateTranslatesMessages(t *testing.T) {
	t.Parallel()

	v := validator.NewThreadCreateValidator()

	inputs := map[string]validator.ThreadCreateValidatorInput{
		"必須":           {Title: "", Language: "", Body: ""},
		"タイトルが長すぎる":    {Title: strings.Repeat("あ", validator.ThreadTitleMaxLength+1), Language: string(model.LocaleJa.ThreadLanguage()), Body: "本文"},
		"タイトルが1行でない":   {Title: "1行目\n2行目", Language: string(model.LocaleJa.ThreadLanguage()), Body: "本文"},
		"使用できない文字":     {Title: "タイトル\x00", Language: string(model.LocaleJa.ThreadLanguage()), Body: "本文"},
		"主言語が選べる言語にない": {Title: "タイトル", Language: "fr", Body: "本文"},
		"本文が長すぎる":      {Title: "タイトル", Language: string(model.LocaleJa.ThreadLanguage()), Body: strings.Repeat("あ", validator.PostBodyMaxLength+1)},
	}

	for _, locale := range model.Locales() {
		for name, input := range inputs {
			t.Run(string(locale)+"/"+name, func(t *testing.T) {
				t.Parallel()

				ctx := i18n.SetLocale(context.Background(), locale)

				_, err := v.Validate(ctx, input)

				ve := model.AsValidationError(err)
				if ve == nil {
					t.Fatalf("expected a ValidationError, got %v", err)
				}
				fieldErrors := ve.FieldErrors()
				if len(fieldErrors) == 0 {
					t.Fatal("expected at least one field error")
				}
				for _, fieldError := range fieldErrors {
					if strings.HasPrefix(fieldError.Message, "validation_") {
						t.Errorf("%s は翻訳されていない (メッセージ ID のまま): %q", fieldError.Field, fieldError.Message)
					}
				}
			})
		}
	}
}

// TestThreadCreateValidator_ValidateVisibleText applies the required check to
// both the title and first-post body without losing text that draws something,
// whether it is a combining sequence or braille.
//
// [Ja] TestThreadCreateValidator_ValidateVisibleText はタイトルと最初の投稿本文の
// 両方に必須チェックを適用し、結合文字列でも点字でも、何かを描くテキストは失わない
// ことを検証する。
func TestThreadCreateValidator_ValidateVisibleText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		text    string
		wantErr bool
	}{
		{name: "異体字セレクター1のみ", text: "\ufe00", wantErr: true},
		{name: "異体字セレクター16のみ", text: "\ufe0f", wantErr: true},
		{name: "結合書記素接合子のみ", text: "\u034f", wantErr: true},
		{name: "異体字セレクター1と空白", text: " \ufe00\t　", wantErr: true},
		{name: "異体字セレクター16と空白", text: " \ufe0f\t　", wantErr: true},
		{name: "結合書記素接合子と空白", text: " \u034f\t　", wantErr: true},
		{name: "点のない点字のマスのみ", text: "\u2800", wantErr: true},
		{name: "点のない点字のマスと空白", text: " \u2800\t　", wantErr: true},
		{name: "非表示文字の組み合わせ", text: "\ufe00\ufe0f\u034f\u200b\u2800", wantErr: true},
		{name: "異体字セレクターを含む絵文字", text: "\u2764\ufe0f"},
		{name: "結合アクセント", text: "e\u0301"},
		{name: "ZWJを含む絵文字", text: "👨‍👩‍👧‍👦"},
		{name: "異体字セレクターを含む漢字", text: "葛\U000e0100"},
		{name: "可視文字の前後に結合書記素接合子", text: "\u034fあ\u034f"},
		{name: "点のあるマスを含む点字", text: "⠓⠊\u2800⠞⠓⠑⠗⠑"},
	}

	v := validator.NewThreadCreateValidator()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := i18n.SetLocale(context.Background(), model.LocaleJa)
			input := validThreadCreateInput()
			input.Title = tt.text
			input.Body = tt.text

			output, err := v.Validate(ctx, input)
			if !tt.wantErr {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if output == nil {
					t.Fatal("expected an output, got nil")
				}
				if output.Title != tt.text || output.Body != tt.text {
					t.Errorf("Title = %q, Body = %q, want both %q", output.Title, output.Body, tt.text)
				}
				return
			}

			ve := model.AsValidationError(err)
			if ve == nil {
				t.Fatalf("expected a ValidationError, got %v", err)
			}
			want := []string{i18n.T(ctx, "validation_required")}
			for _, field := range []string{"title", "body"} {
				if messages := ve.GetFieldErrors(field); !slices.Equal(messages, want) {
					t.Errorf("%s errors = %q, want %q", field, messages, want)
				}
			}
			if output != nil {
				t.Errorf("expected no output on failure, got %#v", output)
			}
		})
	}
}
