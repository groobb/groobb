package model

import "slices"

// ThreadLanguageはスレッドが書かれている言語で、スレッドを立てる人が選びます。
// 値は表示言語にThreadLanguageOtherを加えたものです。人が書く言語はアプリケーションが
// 描ける言語に縛られないためで、UIを日本語と英語で提供するコミュニティにも、そのどちら
// でもない言語のスレッドは立ちます。
//
// Localeが持つ集合を別の見方で読むのではなく独立した型にするのは、スレッドだけが
// 持ちうる値が、アカウントへどの言語で書くかを決めるフィールドへ届かないようにするため
// です。User.Localeに入ったThreadLanguageOtherはメールを送る言語を名指さないもので、
// 2つを分けておくことで、それが誰にも読めないメールではなくコンパイルエラーになります。
type ThreadLanguage string

// ThreadLanguageOtherはどの表示言語にも解決しないスレッド言語です。アプリが
// ロケールを持たない言語で書かれたスレッドが、日本語や英語を騙らずにそのことを言える
// ようにします。騙れば、バッジは誤った言語を名乗り、タイトルのlang属性はスクリーン
// リーダーに誤った言語の規則で発音させることになります。
//
// これはBCP 47のタグではなく、ここからタグを導くものもありません。Presentation層は
// これを1つの言語としてではなく、言語が無いこととして読みます。
const ThreadLanguageOther ThreadLanguage = "other"

// ThreadLanguageは、lで書かれたスレッドが持つスレッド言語を返します。表示言語は
// いずれもスレッドを書ける言語であるため、この向きは失敗しません。値が何にも解決しないと
// 判明しうるのはThreadLanguage.Localeのほうです。
func (l Locale) ThreadLanguage() ThreadLanguage {
	return ThreadLanguage(l)
}

// Localeはlが名指す表示言語を、名指せたかどうかとともに返します。
// ThreadLanguageOtherはどれも名指しません。
//
// スレッド言語の表示は、この1つの問いから答えを得ます。バッジにその言語の自称表記を
// 出すか「その他」の訳語を出すか、タイトルがlangを宣言するかどうかは、同じ判断です。
// 1度だけ問うことで、Englishのバッジが付いたスレッドのタイトルが言語を宣言していない、
// といった食い違いが起きません。
func (l ThreadLanguage) Locale() (Locale, bool) {
	return ParseLocale(string(l))
}

// ThreadLanguagesはスレッドを書ける言語をすべて返します。Localesが与える順序の
// 表示言語に、どれにも解決しない値を続けたものです。
//
// 集合をLocalesから導くことが、言語の追加を1箇所の編集に保ちます。2度書き下せば
// 2つの一覧は離れていき、片方にだけある言語は、スレッドを書けない表示言語か、ページを
// 描けないスレッド言語のどちらかになります。
//
// Localesと同じく呼び出しごとに新しいスライスを返すため、ある呼び出し側が他から見える
// 集合を書き換えてしまうことはありません。
func ThreadLanguages() []ThreadLanguage {
	locales := Locales()

	languages := make([]ThreadLanguage, 0, len(locales)+1)
	for _, locale := range locales {
		languages = append(languages, locale.ThreadLanguage())
	}
	return append(languages, ThreadLanguageOther)
}

// IsValidはlがスレッドを書ける言語のいずれかであるかを返します。
//
// threads.language列は値をCHECKで列挙しません。SQLiteはCHECKを変更できず、言語を
// 1つ足すたびにテーブルを作り直して行を移すマイグレーションが要るためです。値域はここに
// 持ち、ThreadRepository.Createが挿入の前にこれを適用します。集合の外の値は、CHECKが
// あれば拒否されていたのと同じ書き込みで拒否されます。
func (l ThreadLanguage) IsValid() bool {
	return slices.Contains(ThreadLanguages(), l)
}
