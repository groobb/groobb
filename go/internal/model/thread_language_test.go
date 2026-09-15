package model_test

import (
	"slices"
	"testing"

	"github.com/groobb/groobb/go/internal/model"
)

// TestThreadLanguagesは、集合が表示言語にどれにも解決しない値を続けたものであること、
// および受け取ったものを書き換える呼び出し側が、次の呼び出し側の見るものを変えられない
// ことを検証します。
//
// 期待値を書き下さずLocalesから組み立てるのは、導出の眼目が「2つが食い違えないこと」に
// あるためです。ここに書き下した集合は、Localesがスレッドを書けない言語を得た後も通り
// 続けてしまいます。
func TestThreadLanguages(t *testing.T) {
	t.Parallel()

	var want []model.ThreadLanguage
	for _, locale := range model.Locales() {
		want = append(want, model.ThreadLanguage(locale))
	}
	want = append(want, model.ThreadLanguageOther)

	if got := model.ThreadLanguages(); !slices.Equal(got, want) {
		t.Errorf("ThreadLanguages() = %v、期待値 = %v", got, want)
	}

	model.ThreadLanguages()[0] = model.ThreadLanguageOther
	if got := model.ThreadLanguages(); !slices.Equal(got, want) {
		t.Errorf("書き換えた後のThreadLanguages() = %v、期待値 = %v", got, want)
	}
}

// TestThreadLanguage_Localeは、バッジとlang属性の双方を決める問いを検証します。
// スレッド言語は表示言語を名指すか、どれも名指さないかのいずれかです。
func TestThreadLanguage_Locale(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		language model.ThreadLanguage
		want     model.Locale
		wantOK   bool
	}{
		{name: "日本語", language: model.ThreadLanguage("ja"), want: model.LocaleJa, wantOK: true},
		{name: "英語", language: model.ThreadLanguage("en"), want: model.LocaleEn, wantOK: true},
		{name: "どの表示言語にも解決されない値", language: model.ThreadLanguageOther, want: "", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := tt.language.Locale()
			if ok != tt.wantOK {
				t.Errorf("ThreadLanguage(%q).Locale()のok = %v、期待値 = %v", tt.language, ok, tt.wantOK)
			}
			if got != tt.want {
				t.Errorf("ThreadLanguage(%q).Locale() = %q、期待値 = %q", tt.language, got, tt.want)
			}
		})
	}
}

// TestLocale_ThreadLanguageは、表示言語がいずれも、そこへ解決し返すスレッド言語に
// 届くことを検証します。2つの向きが、離れうる2つの対応ではなく1つの関係を表している
// ことを固定します。
func TestLocale_ThreadLanguage(t *testing.T) {
	t.Parallel()

	for _, locale := range model.Locales() {
		language := locale.ThreadLanguage()
		if !language.IsValid() {
			t.Errorf("Locale(%q).ThreadLanguage() = %q、スレッドの言語でない", locale, language)
		}

		got, ok := language.Locale()
		if !ok {
			t.Errorf("Locale(%q).ThreadLanguage().Locale()のok = false、期待値 = true", locale)
			continue
		}
		if got != locale {
			t.Errorf("Locale(%q).ThreadLanguage().Locale() = %q、期待値 = %q", locale, got, locale)
		}
	}
}

// TestThreadLanguage_IsValidは、リポジトリがどの値を列へ通すのかを検証します。
// 表示言語とどれにも解決しない値は受け付け、それ以外は拒否します。ゼロ値を覆うのは、
// フィールドを書き忘れた呼び出し側が渡すものであるためです。
func TestThreadLanguage_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		language model.ThreadLanguage
		want     bool
	}{
		{name: "日本語", language: model.ThreadLanguage("ja"), want: true},
		{name: "英語", language: model.ThreadLanguage("en"), want: true},
		{name: "どの表示言語にも解決されない値", language: model.ThreadLanguageOther, want: true},
		{name: "表示言語の無い言語", language: "fr", want: false},
		{name: "地域が付いたタグ", language: "ja-JP", want: false},
		{name: "ゼロ値", language: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.language.IsValid(); got != tt.want {
				t.Errorf("ThreadLanguage(%q).IsValid() = %v、期待値 = %v", tt.language, got, tt.want)
			}
		})
	}
}
