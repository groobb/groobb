package viewmodel_test

import (
	"testing"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// TestNewThreadLanguage verifies that a thread's language reaches the page as
// the name a badge carries and the tag a title declares, and that the language
// resolving to no display language arrives carrying neither.
//
// [Ja] TestNewThreadLanguage は、スレッドの言語が、バッジが載せる名前とタイトルが宣言
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
			name:         "Japanese",
			language:     model.LocaleJa.ThreadLanguage(),
			wantName:     "日本語",
			wantTag:      "ja",
			wantDeclared: true,
		},
		{
			name:         "English",
			language:     model.LocaleEn.ThreadLanguage(),
			wantName:     "English",
			wantTag:      "en",
			wantDeclared: true,
		},
		{
			name:         "other",
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
				t.Errorf("Name = %q, want %q", got.Name, tt.wantName)
			}
			if got.Tag != tt.wantTag {
				t.Errorf("Tag = %q, want %q", got.Tag, tt.wantTag)
			}
			if got.Declared() != tt.wantDeclared {
				t.Errorf("Declared() = %v, want %v", got.Declared(), tt.wantDeclared)
			}
		})
	}
}

// TestNewThreadLanguage_NamesEveryDisplayLanguage verifies that every display
// language has a name of its own to be badged with. A language added to
// model.Locales but not given one falls back to its tag, which is a badge
// reading "fr" rather than a missing badge, so nothing else would fail.
//
// [Ja] TestNewThreadLanguage_NamesEveryDisplayLanguage は、どの表示言語にもバッジに
// 載せる自身の名前があることを検証します。model.Locales へ足して名前を与えなかった言語は
// タグへフォールバックしますが、それは「fr」と読めるバッジであってバッジの欠落ではない
// ため、他には何も失敗しません。
func TestNewThreadLanguage_NamesEveryDisplayLanguage(t *testing.T) {
	t.Parallel()

	for _, locale := range model.Locales() {
		got := viewmodel.NewThreadLanguage(locale.ThreadLanguage())

		if got.Tag != string(locale) {
			t.Errorf("%s: Tag = %q, want %q", locale, got.Tag, string(locale))
		}
		if got.Name == string(locale) {
			t.Errorf("%s: Name = %q, want その言語自身の名前 (タグへのフォールバックになっている)", locale, got.Name)
		}
	}
}
