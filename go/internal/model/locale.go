package model

// Locale is a language Groobb draws its UI and its mail in. Its values are the
// languages the application ships translations for, which is what makes the set
// closed: a value outside it names a language nothing can be rendered in, and an
// account carrying one would receive mail in no language at all.
//
// It is a string matching the users.locale column rather than an integer enum,
// the same choice as EmailConfirmationEvent: a row stays self-describing in the
// database without a lookup, and the stored value is the BCP 47 tag the pages
// and the mails already carry.
//
// It lives in this package rather than in internal/i18n because the language an
// account is written to is an attribute of the user, and internal/i18n is a
// Presentation-layer package the Domain layer cannot depend on. i18n resolves a
// locale from a request and carries it in the context; which values it may
// resolve to is what this package holds.
//
// [Ja] Locale は Groobb が UI とメールを描く言語です。値はアプリが翻訳を同梱する言語で
// あり、それが値域を閉じたものにしています。この外の値は何も描けない言語を名指すもので、
// それを持つアカウントはどの言語でもないメールを受け取ることになります。
//
// 整数 enum ではなく users.locale 列に合わせた string とするのは、
// EmailConfirmationEvent と同じ選択です。DB 上で照合なしに行が自己記述的になり、保存する
// 値はページとメールが既に載せている BCP 47 の言語タグそのものになります。
//
// internal/i18n ではなく本パッケージに置くのは、アカウントへどの言語で書くかがユーザーの
// 属性であり、internal/i18n が Domain 層からは依存できない Presentation 層のパッケージで
// あるためです。i18n はリクエストからロケールを解決して context で運びますが、解決先と
// なりうる値の集合は本パッケージが持つものです。
type Locale string

// The display languages the application ships translations for.
//
// [Ja] アプリが翻訳を同梱する表示言語です。
const (
	LocaleJa Locale = "ja"
	LocaleEn Locale = "en"
)

// DefaultLocale is the locale used where none has been resolved: a request whose
// Accept-Language names no display language, and an account created before one
// is asked for. Japanese is that value because Groobb's own community is
// Japanese-speaking, so it is the language most of the visitors who arrive
// without stating one read.
//
// [Ja] DefaultLocale はロケールが解決されていない場面で使うロケールです。Accept-Language
// がどの表示言語も名指さないリクエストと、ロケールを尋ねる前に作られるアカウントが
// それにあたります。日本語を充てるのは、Groobb 自身のコミュニティが日本語話者のもので
// あり、言語を告げずに訪れる人の多くが読む言語であるためです。
const DefaultLocale = LocaleJa

// Locales returns every display language. It is the one place the set is written
// down: i18n reads it to load the embedded translation files and to resolve a
// request's language.
//
// Adding a language takes four changes together: the value here, its locale
// file, body templates for it in internal/email, and the language's own name in
// internal/viewmodel, which is what a thread's language badge carries. The mail
// senders pick their HTML and text bodies with a switch whose default branch is
// English, so a language given only the first two would be mailed an English
// body under a subject translated into that language; one missing the last is
// badged with its tag.
//
// A fresh slice is returned per call so a caller cannot edit the set out from
// under the others.
//
// [Ja] Locales はすべての表示言語を返します。値域を書き下す唯一の場所であり、i18n は
// 埋め込んだ翻訳ファイルの読み込みとリクエストの言語の解決の両方でこれを読みます。
//
// 言語を増やすには 4 つの変更が揃う必要があります。ここへ足す値、そのロケールファイル、
// internal/email のその言語の本文テンプレート、そして internal/viewmodel に置くその言語
// 自身の名前です。最後のものは、スレッドの言語のバッジが載せるものです。メールの Sender は
// HTML とテキストの本文を default 節が英語である switch で選ぶため、前の 2 つだけを与えた
// 言語は、その言語に翻訳された件名に英語の本文が付いたメールになります。最後の 1 つを
// 欠いた言語は、自身のタグでバッジに示されます。
//
// 呼び出しごとに新しいスライスを返すため、ある呼び出し側が他から見える集合を書き換えて
// しまうことはありません。
func Locales() []Locale {
	return []Locale{LocaleJa, LocaleEn}
}

// ParseLocale returns the Locale s names, reporting whether it named one. It is
// how a value from outside the application enters the type: an Accept-Language
// tag, or a job argument that crossed the queue as JSON. A bare conversion at
// either would admit any string and leave the closed set unenforced where the
// input was not the application's to begin with.
//
// Reading users.locale back is not one of those boundaries. The column is only
// ever written through this type, so repository converts the row outright, as it
// does for EmailConfirmationEvent.
//
// [Ja] ParseLocale は s が名指す Locale を、名指せたかどうかとともに返します。
// アプリケーションの外から来た値が型に入る経路であり、Accept-Language のタグや、
// JSON としてキューを渡ってきたジョブ引数がそれにあたります。どちらでも素の型変換では
// どの文字列も通ってしまい、入力がそもそもアプリケーションのものではない場所で、閉じた
// 値域が守られなくなります。
//
// users.locale の読み戻しはこの境界に含みません。この列はこの型を通してしか書かれない
// ため、repository は EmailConfirmationEvent と同じく行の値をそのまま型変換します。
func ParseLocale(s string) (Locale, bool) {
	for _, locale := range Locales() {
		if string(locale) == s {
			return locale, true
		}
	}
	return "", false
}
