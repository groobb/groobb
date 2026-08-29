package viewmodel

import "github.com/groobb/groobb/go/internal/model"

// ThreadLanguage is a thread's primary language as a page shows it: the name a
// badge carries, and the BCP 47 tag the thread's title declares itself in. Both
// are empty for a thread whose language resolves to no display language, which
// is the one case a page neither names a language nor declares one.
//
// It resolves model.ThreadLanguage.Locale once, where each page would otherwise
// ask it again. The badge's wording and the title's lang attribute are the same
// decision, so answering it here keeps a thread from being badged English while
// its title declares nothing.
//
// [Ja] ThreadLanguage は、ページが見せる形のスレッドの主言語です。バッジが載せる名前と、
// スレッドのタイトルが自身を宣言する BCP 47 のタグを持ちます。どの表示言語にも解決しない
// 言語のスレッドではその両方が空になります。ページが言語を名指しも宣言もしない唯一の
// 場合がこれです。
//
// model.ThreadLanguage.Locale をここで 1 度だけ解きます。そうしなければ各ページが
// それぞれ問い直すことになります。バッジの文言とタイトルの lang 属性は同じ判断であるため、
// ここで答えておくことで、English のバッジが付いたスレッドのタイトルが何も宣言していない、
// という状態が生まれません。
type ThreadLanguage struct {
	// Name is the language under its own name, which is what the badge shows for
	// a thread whose language resolves to a display language. It is "" for the
	// language that resolves to none: that thread has no name of its own to show,
	// and the badge draws the translated word for "other" instead.
	//
	// [Ja] Name はその言語自身の名前による表記で、表示言語に解決する言語のスレッドで
	// バッジが見せるものです。どれにも解決しない言語では "" になります。そのスレッドには
	// 見せるべき自称表記が無く、バッジは代わりに「その他」の訳語を描きます。
	Name string

	// Tag is the BCP 47 language tag the thread's title is declared with, and ""
	// when the thread's language resolves to no display language. Nothing derives
	// a tag for that case: a title left undeclared is read by the page's own
	// language, where an invented tag would have a screen reader pronounce it by
	// the rules of a language it is not written in.
	//
	// [Ja] Tag はスレッドのタイトルが宣言される BCP 47 の言語タグで、スレッドの言語が
	// どの表示言語にも解決しないときは "" です。その場合にタグを導くものはありません。
	// 宣言されないタイトルはページ自身の言語で読まれますが、でっち上げたタグは、その
	// タイトルが書かれていない言語の規則でスクリーンリーダーに発音させることになります。
	Tag string
}

// languageNames are the display languages under their own names. A badge carries
// these rather than the word for the language in the language the page is drawn
// in, so that a speaker finds their own language on a listing whatever the UI
// language is, and so that adding a language does not mean translating its name
// into every locale.
//
// A locale absent from this map falls back to its own tag, so a language added
// to model.Locales and forgotten here is badged "fr" rather than badged with
// nothing at all.
//
// [Ja] languageNames は表示言語をその言語自身の名前で持つものです。バッジがページの
// 描かれている言語での呼び名ではなくこちらを載せるのは、UI がどの言語で描かれていても
// 話者が一覧の中に自分の言語を見つけられるようにするためであり、また言語を足すことが
// その名前を全ロケールへ翻訳することを意味しないようにするためです。
//
// この map に無いロケールは自身のタグへフォールバックします。model.Locales に足して
// ここへ足し忘れた言語は、何も表示されないバッジではなく "fr" のバッジになります。
var languageNames = map[model.Locale]string{
	model.LocaleJa: "日本語",
	model.LocaleEn: "English",
}

// NewThreadLanguage converts a thread's language into what a page shows for it.
//
// [Ja] NewThreadLanguage はスレッドの言語を、ページがそれについて見せるものへ変換します。
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

// Declared reports whether the thread names a language the page can declare. It
// is the one question the badge's wording and the title's lang attribute are
// both answered from.
//
// [Ja] Declared は、スレッドがページの宣言できる言語を名指しているかどうかを返します。
// バッジの文言とタイトルの lang 属性が、ともにここから答えを得る 1 つの問いです。
func (l ThreadLanguage) Declared() bool {
	return l.Tag != ""
}
