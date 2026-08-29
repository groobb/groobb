package model_test

import (
	"slices"
	"testing"

	"github.com/groobb/groobb/go/internal/model"
)

// TestParseLocale verifies which strings name a display language and which do
// not: every locale the application ships translations for is accepted, and
// anything else is refused rather than admitted into the type.
//
// [Ja] TestParseLocale は、どの文字列が表示言語を名指し、どれが名指さないのかを検証
// します。アプリが翻訳を同梱するロケールはいずれも受け付け、それ以外は型に通さず
// 拒否します。
func TestParseLocale(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  string
		want   model.Locale
		wantOK bool
	}{
		{name: "Japanese", input: "ja", want: model.LocaleJa, wantOK: true},
		{name: "English", input: "en", want: model.LocaleEn, wantOK: true},
		{name: "a language with no translations", input: "fr", want: "", wantOK: false},
		{name: "the thread language that resolves to none", input: "other", want: "", wantOK: false},
		{name: "empty", input: "", want: "", wantOK: false},
		// The tag is matched as written: a region subtag is a different string,
		// and it is i18n's job to reduce a header value to its base first.
		//
		// [Ja] タグは書かれたとおりに照合します。地域のサブタグが付いた文字列は別の
		// 文字列であり、ヘッダーの値をその基底へ落とすのは i18n の仕事です。
		{name: "a tag carrying a region", input: "ja-JP", want: "", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := model.ParseLocale(tt.input)
			if ok != tt.wantOK {
				t.Errorf("ParseLocale(%q) ok = %v, want %v", tt.input, ok, tt.wantOK)
			}
			if got != tt.want {
				t.Errorf("ParseLocale(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestLocales verifies that the set holds the display languages the locale files
// are shipped for, and that a caller editing what it receives cannot change what
// the next caller sees.
//
// [Ja] TestLocales は、集合がロケールファイルを同梱している表示言語を保持すること、
// および受け取ったものを書き換える呼び出し側が、次の呼び出し側の見るものを変えられない
// ことを検証します。
func TestLocales(t *testing.T) {
	t.Parallel()

	want := []model.Locale{model.LocaleJa, model.LocaleEn}
	if got := model.Locales(); !slices.Equal(got, want) {
		t.Errorf("Locales() = %v, want %v", got, want)
	}

	model.Locales()[0] = model.LocaleEn
	if got := model.Locales(); !slices.Equal(got, want) {
		t.Errorf("Locales() after an edit = %v, want %v", got, want)
	}
}
