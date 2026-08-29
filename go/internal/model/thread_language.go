package model

import "slices"

// ThreadLanguage is the language a thread is written in, chosen by whoever
// starts it. Its values are the display languages plus ThreadLanguageOther,
// because what people write in is not bounded by what the application can be
// drawn in: a community offering its UI in Japanese and English still receives
// threads written in neither.
//
// It is a type of its own rather than the set Locale holds read a second way, so
// that a value only a thread may carry cannot reach a field deciding which
// language an account is written to. ThreadLanguageOther in User.Locale would
// name no language to send mail in, and keeping the two apart makes that a
// compile error instead of a mail nobody can read.
//
// [Ja] ThreadLanguage はスレッドが書かれている言語で、スレッドを立てる人が選びます。
// 値は表示言語に ThreadLanguageOther を加えたものです。人が書く言語はアプリケーションが
// 描ける言語に縛られないためで、UI を日本語と英語で提供するコミュニティにも、そのどちら
// でもない言語のスレッドは立ちます。
//
// Locale が持つ集合を別の見方で読むのではなく独立した型にするのは、スレッドだけが
// 持ちうる値が、アカウントへどの言語で書くかを決めるフィールドへ届かないようにするため
// です。User.Locale に入った ThreadLanguageOther はメールを送る言語を名指さないもので、
// 2 つを分けておくことで、それが誰にも読めないメールではなくコンパイルエラーになります。
type ThreadLanguage string

// ThreadLanguageOther is the thread language that resolves to no display
// language. It lets a thread written in a language the application has no
// locale for say so, instead of claiming to be Japanese or English: a claim
// would put the wrong language on the badge, and the lang attribute on the title
// would have a screen reader pronounce it by the wrong language's rules.
//
// It is not a BCP 47 tag and nothing derives one from it. Presentation reads it
// as the absence of a language rather than as a language of its own.
//
// [Ja] ThreadLanguageOther はどの表示言語にも解決しないスレッド言語です。アプリが
// ロケールを持たない言語で書かれたスレッドが、日本語や英語を騙らずにそのことを言える
// ようにします。騙れば、バッジは誤った言語を名乗り、タイトルの lang 属性はスクリーン
// リーダーに誤った言語の規則で発音させることになります。
//
// これは BCP 47 のタグではなく、ここからタグを導くものもありません。Presentation 層は
// これを 1 つの言語としてではなく、言語が無いこととして読みます。
const ThreadLanguageOther ThreadLanguage = "other"

// ThreadLanguage returns the thread language a thread written in l carries.
// Every display language is a language a thread can be written in, so this
// direction never fails; ThreadLanguage.Locale is the one where a value can turn
// out to resolve to nothing.
//
// [Ja] ThreadLanguage は、l で書かれたスレッドが持つスレッド言語を返します。表示言語は
// いずれもスレッドを書ける言語であるため、この向きは失敗しません。値が何にも解決しないと
// 判明しうるのは ThreadLanguage.Locale のほうです。
func (l Locale) ThreadLanguage() ThreadLanguage {
	return ThreadLanguage(l)
}

// Locale returns the display language l names, reporting whether it named one.
// ThreadLanguageOther names none.
//
// It is the single question the presentation of a thread language is answered
// from: whether the badge carries the language's own name or the translated word
// for "other", and whether the title declares a lang, are the same decision.
// Asking it once keeps a thread from being badged English while its title goes
// undeclared.
//
// [Ja] Locale は l が名指す表示言語を、名指せたかどうかとともに返します。
// ThreadLanguageOther はどれも名指しません。
//
// スレッド言語の表示は、この 1 つの問いから答えを得ます。バッジにその言語の自称表記を
// 出すか「その他」の訳語を出すか、タイトルが lang を宣言するかどうかは、同じ判断です。
// 1 度だけ問うことで、English のバッジが付いたスレッドのタイトルが言語を宣言していない、
// といった食い違いが起きません。
func (l ThreadLanguage) Locale() (Locale, bool) {
	return ParseLocale(string(l))
}

// ThreadLanguages returns every language a thread may be written in: the display
// languages in the order Locales gives them, followed by the value that resolves
// to none.
//
// Deriving the set from Locales is what keeps adding a language a single edit.
// Written down twice, the two lists would drift apart, and the language present
// in only one of them would be either a display language no thread can be
// written in or a thread language no page can be drawn in.
//
// A fresh slice is returned per call, as Locales does, so a caller cannot edit
// the set out from under the others.
//
// [Ja] ThreadLanguages はスレッドを書ける言語をすべて返します。Locales が与える順序の
// 表示言語に、どれにも解決しない値を続けたものです。
//
// 集合を Locales から導くことが、言語の追加を 1 箇所の編集に保ちます。2 度書き下せば
// 2 つの一覧は離れていき、片方にだけある言語は、スレッドを書けない表示言語か、ページを
// 描けないスレッド言語のどちらかになります。
//
// Locales と同じく呼び出しごとに新しいスライスを返すため、ある呼び出し側が他から見える
// 集合を書き換えてしまうことはありません。
func ThreadLanguages() []ThreadLanguage {
	locales := Locales()

	languages := make([]ThreadLanguage, 0, len(locales)+1)
	for _, locale := range locales {
		languages = append(languages, locale.ThreadLanguage())
	}
	return append(languages, ThreadLanguageOther)
}

// IsValid reports whether l is one of the languages a thread may be written in.
//
// The threads.language column lists no values in a CHECK: SQLite cannot alter
// one, so every language added would take a migration that rebuilds the table
// and copies its rows. The set is held here instead, and ThreadRepository.Create
// applies this before the insert, so a value outside it is refused at the write
// a CHECK would have refused it at.
//
// [Ja] IsValid は l がスレッドを書ける言語のいずれかであるかを返します。
//
// threads.language 列は値を CHECK で列挙しません。SQLite は CHECK を変更できず、言語を
// 1 つ足すたびにテーブルを作り直して行を移すマイグレーションが要るためです。値域はここに
// 持ち、ThreadRepository.Create が挿入の前にこれを適用します。集合の外の値は、CHECK が
// あれば拒否されていたのと同じ書き込みで拒否されます。
func (l ThreadLanguage) IsValid() bool {
	return slices.Contains(ThreadLanguages(), l)
}
