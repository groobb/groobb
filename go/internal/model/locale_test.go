package model_test

import (
	"slices"
	"testing"

	"github.com/groobb/groobb/go/internal/model"
)

// TestParseLocaleは、どの文字列が表示言語を名指し、どれが名指さないのかを検証
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
		{name: "日本語", input: "ja", want: model.LocaleJa, wantOK: true},
		{name: "英語", input: "en", want: model.LocaleEn, wantOK: true},
		{name: "翻訳の無い言語", input: "fr", want: "", wantOK: false},
		{name: "どの表示言語にも解決されないスレッドの言語", input: "other", want: "", wantOK: false},
		{name: "空文字列", input: "", want: "", wantOK: false},
		// タグは書かれたとおりに照合します。地域のサブタグが付いた文字列は別の
		// 文字列であり、ヘッダーの値をその基底へ落とすのはi18nの仕事です。
		{name: "地域が付いたタグ", input: "ja-JP", want: "", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := model.ParseLocale(tt.input)
			if ok != tt.wantOK {
				t.Errorf("ParseLocale(%q)のok = %v、期待値 = %v", tt.input, ok, tt.wantOK)
			}
			if got != tt.want {
				t.Errorf("ParseLocale(%q) = %q、期待値 = %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestLocalesは、集合がロケールファイルを同梱している表示言語を保持すること、
// および受け取ったものを書き換える呼び出し側が、次の呼び出し側の見るものを変えられない
// ことを検証します。
func TestLocales(t *testing.T) {
	t.Parallel()

	want := []model.Locale{model.LocaleJa, model.LocaleEn}
	if got := model.Locales(); !slices.Equal(got, want) {
		t.Errorf("Locales() = %v、期待値 = %v", got, want)
	}

	model.Locales()[0] = model.LocaleEn
	if got := model.Locales(); !slices.Equal(got, want) {
		t.Errorf("書き換えた後のLocales() = %v、期待値 = %v", got, want)
	}
}
