package model_test

import (
	"slices"
	"testing"

	"github.com/groobb/groobb/go/internal/model"
)

// TestThreadLanguages verifies that the set is the display languages followed by
// the value resolving to none, and that a caller editing what it receives cannot
// change what the next caller sees.
//
// The expectation is written against Locales rather than spelled out, because
// the point of the derivation is that the two cannot disagree: a set written out
// here would keep passing after Locales gained a language the threads could not
// be written in.
//
// [Ja] TestThreadLanguages は、集合が表示言語にどれにも解決しない値を続けたものであること、
// および受け取ったものを書き換える呼び出し側が、次の呼び出し側の見るものを変えられない
// ことを検証します。
//
// 期待値を書き下さず Locales から組み立てるのは、導出の眼目が「2 つが食い違えないこと」に
// あるためです。ここに書き下した集合は、Locales がスレッドを書けない言語を得た後も通り
// 続けてしまいます。
func TestThreadLanguages(t *testing.T) {
	t.Parallel()

	var want []model.ThreadLanguage
	for _, locale := range model.Locales() {
		want = append(want, model.ThreadLanguage(locale))
	}
	want = append(want, model.ThreadLanguageOther)

	if got := model.ThreadLanguages(); !slices.Equal(got, want) {
		t.Errorf("ThreadLanguages() = %v, want %v", got, want)
	}

	model.ThreadLanguages()[0] = model.ThreadLanguageOther
	if got := model.ThreadLanguages(); !slices.Equal(got, want) {
		t.Errorf("ThreadLanguages() after an edit = %v, want %v", got, want)
	}
}

// TestThreadLanguage_Locale verifies the question the badge and the lang
// attribute are both decided from: a thread language either names a display
// language or names none.
//
// [Ja] TestThreadLanguage_Locale は、バッジと lang 属性の双方を決める問いを検証します。
// スレッド言語は表示言語を名指すか、どれも名指さないかのいずれかです。
func TestThreadLanguage_Locale(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		language model.ThreadLanguage
		want     model.Locale
		wantOK   bool
	}{
		{name: "Japanese", language: model.ThreadLanguage("ja"), want: model.LocaleJa, wantOK: true},
		{name: "English", language: model.ThreadLanguage("en"), want: model.LocaleEn, wantOK: true},
		{name: "the value that resolves to none", language: model.ThreadLanguageOther, want: "", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := tt.language.Locale()
			if ok != tt.wantOK {
				t.Errorf("ThreadLanguage(%q).Locale() ok = %v, want %v", tt.language, ok, tt.wantOK)
			}
			if got != tt.want {
				t.Errorf("ThreadLanguage(%q).Locale() = %q, want %q", tt.language, got, tt.want)
			}
		})
	}
}

// TestLocale_ThreadLanguage verifies that every display language reaches a
// thread language that resolves back to it, so the two directions describe one
// relationship rather than two mappings that can drift.
//
// [Ja] TestLocale_ThreadLanguage は、表示言語がいずれも、そこへ解決し返すスレッド言語に
// 届くことを検証します。2 つの向きが、離れうる 2 つの対応ではなく 1 つの関係を表している
// ことを固定します。
func TestLocale_ThreadLanguage(t *testing.T) {
	t.Parallel()

	for _, locale := range model.Locales() {
		language := locale.ThreadLanguage()
		if !language.IsValid() {
			t.Errorf("Locale(%q).ThreadLanguage() = %q, which is not a thread language", locale, language)
		}

		got, ok := language.Locale()
		if !ok {
			t.Errorf("Locale(%q).ThreadLanguage().Locale() ok = false, want true", locale)
			continue
		}
		if got != locale {
			t.Errorf("Locale(%q).ThreadLanguage().Locale() = %q, want %q", locale, got, locale)
		}
	}
}

// TestThreadLanguage_IsValid verifies which values the repository lets into the
// column: the display languages and the value that resolves to none are
// accepted, and everything else is refused. The zero value is covered because it
// is what a caller that forgets the field passes.
//
// [Ja] TestThreadLanguage_IsValid は、リポジトリがどの値を列へ通すのかを検証します。
// 表示言語とどれにも解決しない値は受け付け、それ以外は拒否します。ゼロ値を覆うのは、
// フィールドを書き忘れた呼び出し側が渡すものであるためです。
func TestThreadLanguage_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		language model.ThreadLanguage
		want     bool
	}{
		{name: "Japanese", language: model.ThreadLanguage("ja"), want: true},
		{name: "English", language: model.ThreadLanguage("en"), want: true},
		{name: "the value that resolves to none", language: model.ThreadLanguageOther, want: true},
		{name: "a language with no locale", language: "fr", want: false},
		{name: "a tag carrying a region", language: "ja-JP", want: false},
		{name: "the zero value", language: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.language.IsValid(); got != tt.want {
				t.Errorf("ThreadLanguage(%q).IsValid() = %v, want %v", tt.language, got, tt.want)
			}
		})
	}
}
