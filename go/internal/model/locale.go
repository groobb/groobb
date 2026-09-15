package model

// LocaleはGroobbがUIとメールを描く言語です。値はアプリが翻訳を同梱する言語で
// あり、それが値域を閉じたものにしています。この外の値は何も描けない言語を名指すもので、
// それを持つアカウントはどの言語でもないメールを受け取ることになります。
//
// 整数enumではなくusers.locale列に合わせたstringとするのは、
// EmailConfirmationEventと同じ選択です。DB上で照合なしに行が自己記述的になり、保存する
// 値はページとメールが既に載せているBCP 47の言語タグそのものになります。
//
// internal/i18nではなく本パッケージに置くのは、アカウントへどの言語で書くかがユーザーの
// 属性であり、internal/i18nがDomain層からは依存できないPresentation層のパッケージで
// あるためです。i18nはリクエストからロケールを解決してcontextで運びますが、解決先と
// なりうる値の集合は本パッケージが持つものです。
type Locale string

// アプリが翻訳を同梱する表示言語です。
const (
	LocaleJa Locale = "ja"
	LocaleEn Locale = "en"
)

// DefaultLocaleはロケールが解決されていない場面で使うロケールです。Accept-Language
// がどの表示言語も名指さないリクエストと、ロケールを尋ねる前に作られるアカウントが
// それにあたります。日本語を充てるのは、Groobb自身のコミュニティが日本語話者のもので
// あり、言語を告げずに訪れる人の多くが読む言語であるためです。
const DefaultLocale = LocaleJa

// Localesはすべての表示言語を返します。値域を書き下す唯一の場所であり、i18nは
// 埋め込んだ翻訳ファイルの読み込みとリクエストの言語の解決の両方でこれを読みます。
//
// 言語を増やすには4つの変更が揃う必要があります。ここへ足す値、そのロケールファイル、
// internal/emailのその言語の本文テンプレート、そしてinternal/viewmodelに置くその言語
// 自身の名前です。最後のものは、スレッドの言語のバッジが載せるものです。メールのSenderは
// HTMLとテキストの本文をdefault節が英語であるswitchで選ぶため、前の2つだけを与えた
// 言語は、その言語に翻訳された件名に英語の本文が付いたメールになります。最後の1つを
// 欠いた言語は、自身のタグでバッジに示されます。
//
// 呼び出しごとに新しいスライスを返すため、ある呼び出し側が他から見える集合を書き換えて
// しまうことはありません。
func Locales() []Locale {
	return []Locale{LocaleJa, LocaleEn}
}

// ParseLocaleはsが名指すLocaleを、名指せたかどうかとともに返します。
// アプリケーションの外から来た値が型に入る経路であり、Accept-Languageのタグや、
// JSONとしてキューを渡ってきたジョブ引数がそれにあたります。どちらでも素の型変換では
// どの文字列も通ってしまい、入力がそもそもアプリケーションのものではない場所で、閉じた
// 値域が守られなくなります。
//
// users.localeの読み戻しはこの境界に含みません。この列はこの型を通してしか書かれない
// ため、repositoryはEmailConfirmationEventと同じく行の値をそのまま型変換します。
func ParseLocale(s string) (Locale, bool) {
	for _, locale := range Locales() {
		if string(locale) == s {
			return locale, true
		}
	}
	return "", false
}
