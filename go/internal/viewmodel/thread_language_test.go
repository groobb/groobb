package viewmodel_test

import (
	"testing"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// TestNewThreadLanguageは、スレッドの言語が、バッジが載せる名前とタイトルが宣言
// するタグとしてページへ届くこと、そしてどの表示言語にも解決しない言語がそのどちらも
// 持たずに届くことを検証します。
func TestNewThreadLanguage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		language     model.ThreadLanguage
		wantName     string
		wantTag      string
		wantDeclared bool
	}{
		{
			name:         "日本語",
			language:     model.LocaleJa.ThreadLanguage(),
			wantName:     "日本語",
			wantTag:      "ja",
			wantDeclared: true,
		},
		{
			name:         "英語",
			language:     model.LocaleEn.ThreadLanguage(),
			wantName:     "English",
			wantTag:      "en",
			wantDeclared: true,
		},
		{
			name:         "その他",
			language:     model.ThreadLanguageOther,
			wantName:     "",
			wantTag:      "",
			wantDeclared: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := viewmodel.NewThreadLanguage(tt.language)

			if got.Name != tt.wantName {
				t.Errorf("Name = %q、期待値 = %q", got.Name, tt.wantName)
			}
			if got.Tag != tt.wantTag {
				t.Errorf("Tag = %q、期待値 = %q", got.Tag, tt.wantTag)
			}
			if got.Declared() != tt.wantDeclared {
				t.Errorf("Declared() = %v、期待値 = %v", got.Declared(), tt.wantDeclared)
			}
		})
	}
}

// TestNewThreadLanguage_NamesEveryDisplayLanguageは、どの表示言語にもバッジに
// 載せる自身の名前があることを検証します。model.Localesへ足して名前を与えなかった言語は
// タグへフォールバックしますが、それは「fr」と読めるバッジであってバッジの欠落ではない
// ため、他には何も失敗しません。
func TestNewThreadLanguage_NamesEveryDisplayLanguage(t *testing.T) {
	t.Parallel()

	for _, locale := range model.Locales() {
		got := viewmodel.NewThreadLanguage(locale.ThreadLanguage())

		if got.Tag != string(locale) {
			t.Errorf("%s: Tag = %q、期待値 = %q", locale, got.Tag, string(locale))
		}
		if got.Name == string(locale) {
			t.Errorf("%s: Name = %q、期待値はその言語自身の名前 (タグへのフォールバックになっている)", locale, got.Name)
		}
	}
}
