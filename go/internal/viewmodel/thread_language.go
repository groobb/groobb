package viewmodel

import "github.com/groobb/groobb/go/internal/model"

// ThreadLanguageは、ページが見せる形のスレッドの主言語です。バッジが載せる名前と、
// スレッドのタイトルが自身を宣言するBCP 47のタグを持ちます。どの表示言語にも解決しない
// 言語のスレッドではその両方が空になります。ページが言語を名指しも宣言もしない唯一の
// 場合がこれです。
//
// model.ThreadLanguage.Localeをここで1度だけ解きます。そうしなければ各ページが
// それぞれ問い直すことになります。バッジの文言とタイトルのlang属性は同じ判断であるため、
// ここで答えておくことで、Englishのバッジが付いたスレッドのタイトルが何も宣言していない、
// という状態が生まれません。
type ThreadLanguage struct {
	// Nameはその言語自身の名前による表記で、表示言語に解決する言語のスレッドで
	// バッジが見せるものです。どれにも解決しない言語では "" になります。そのスレッドには
	// 見せるべき自称表記が無く、バッジは代わりに「その他」の訳語を描きます。
	Name string

	// Tagはスレッドのタイトルが宣言されるBCP 47の言語タグで、スレッドの言語が
	// どの表示言語にも解決しないときは "" です。その場合にタグを導くものはありません。
	// 宣言されないタイトルはページ自身の言語で読まれますが、でっち上げたタグは、その
	// タイトルが書かれていない言語の規則でスクリーンリーダーに発音させることになります。
	Tag string
}

// languageNamesは表示言語をその言語自身の名前で持つものです。バッジがページの
// 描かれている言語での呼び名ではなくこちらを載せるのは、UIがどの言語で描かれていても
// 話者が一覧の中に自分の言語を見つけられるようにするためであり、また言語を足すことが
// その名前を全ロケールへ翻訳することを意味しないようにするためです。
//
// このmapに無いロケールは自身のタグへフォールバックします。model.Localesに足して
// ここへ足し忘れた言語は、何も表示されないバッジではなく "fr" のバッジになります。
var languageNames = map[model.Locale]string{
	model.LocaleJa: "日本語",
	model.LocaleEn: "English",
}

// NewThreadLanguageはスレッドの言語を、ページがそれについて見せるものへ変換します。
func NewThreadLanguage(language model.ThreadLanguage) ThreadLanguage {
	locale, ok := language.Locale()
	if !ok {
		return ThreadLanguage{}
	}

	name, ok := languageNames[locale]
	if !ok {
		name = string(locale)
	}
	return ThreadLanguage{Name: name, Tag: string(locale)}
}

// Declaredは、スレッドがページの宣言できる言語を名指しているかどうかを返します。
// バッジの文言とタイトルのlang属性が、ともにここから答えを得る1つの問いです。
func (l ThreadLanguage) Declared() bool {
	return l.Tag != ""
}
