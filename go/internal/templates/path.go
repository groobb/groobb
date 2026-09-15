package templates

import (
	"net/url"
	"strconv"

	"github.com/a-h/templ"

	"github.com/groobb/groobb/go/internal/viewmodel"
)

// ReturnToParamはサインインを終えた訪問者をどこへ送るかを運ぶクエリ / フォーム
// パラメータです。RequireAuthが匿名リクエストを追い返すときに書き込み、サインイン系の
// フォームがセッション発行まで引き継ぎます。値はmiddleware.SanitizeReturnToを通った
// ものだけを信頼します。
const ReturnToParam = "return_to"

// Pathはアプリケーション内のURLパスを表す型です。パス文字列をここに集約
// することで、テンプレートはリテラルをハードコードせずにルートへリンクでき、ルート
// 変更を各テンプレートではなく1箇所で行えます。
type Path string

// Stringはパスを文字列として返します。
func (p Path) String() string {
	return string(p)
}

// SafeURLはパスをhref / action属性で使うためのtempl.SafeURLとして返します。
func (p Path) SafeURL() templ.SafeURL {
	return templ.SafeURL(p)
}

// AbsoluteURLはpをbaseURLの下の絶対URLとして返します。ページが自身の正規
// アドレスを宣言する形であり、パンくずの各段をクローラーへ名指す形でもあります。どちらも
// それが書かれた文書から離れて読まれるため、ホスト相対のパスでは、どのホストのことなのか
// の判断が読み手に委ねられます。
//
// baseURLが空のときは空文字列を返します。インスタンスの公開ベースURLは任意の設定で
// あり、自身のアドレスを教えられていないインスタンスはそれを名指せません。その場合
// 呼び出し側は、絶対URLが期待される場所へ相対URLを出すのではなく、何も描画しません。
func (p Path) AbsoluteURL(baseURL string) string {
	if baseURL == "" {
		return ""
	}

	return baseURL + string(p)
}

// WithReturnToはreturnToをreturn_toクエリパラメータに載せたパスを返します。
// 最初のこのURLはRequireAuthが匿名リクエストを追い返すときに組み立て、以降の各ホップ
// (パスワードのステップからTOTPチャレンジへ、TOTPとリカバリーコードのチャレンジを
// 行き来するリンク、チャレンジが失われたときのサインインへのやり直し) が組み立て直します。
// これにより、訪問者が本来向かっていた遷移先はセッションを発行するステップまで残ります。
// returnToが空のときはパスをそのまま返し、遷移先を持たないフローは素のパスのままになり
// ます。呼び出し側はmiddleware.SanitizeReturnToが受け付け済みの値を渡します。本メソッド
// はURLを組み立てるだけで、値の再検証は行いません。レシーバーは、本ファイルのルート
// ヘルパーが返すとおりクエリを持たないパスであることを前提とします。パラメータはリテラルの
// "?" の後ろに連結するため、既にクエリを持つレシーバーでは "?" が2つ並んでしまいます。
// 本ファイルで唯一これに当てはまらないのがAfterSignInPathで、呼び出し側の遷移先を
// そのまま返すためクエリを含みうります。
func (p Path) WithReturnTo(returnTo string) Path {
	if returnTo == "" {
		return p
	}

	return Path(string(p) + "?" + url.Values{ReturnToParam: {returnTo}}.Encode())
}

// RootPathはトップページのパスを返します。
func RootPath() Path {
	return Path("/")
}

// AfterSignInPathはセッション発行後に訪問者を送る先を返します。サインインフローが
// returnToで運んできた遷移先、運んでこなかったときはホームです。セッションを発行する
// 3つのルート (パスワード・TOTP・リカバリーコード) がこれを共有し、訪問者を同じ場所へ
// 着地させます。トップページではなくホームなのは、トップページがサインイン済みの訪問者を
// 結局ホームへ送るためです。直接ホームへ送ればその1段分のリダイレクトを省けます。
// returnToはmiddleware.SanitizeReturnToが受け付け済みの値です。
func AfterSignInPath(returnTo string) Path {
	if returnTo == "" {
		return HomePath()
	}

	return Path(returnTo)
}

// SignUpPathはサインアップフォームのパスを返します。
func SignUpPath() Path {
	return Path("/sign_up")
}

// SignInPathはサインインフォームのパスを返します。
func SignInPath() Path {
	return Path("/sign_in")
}

// SignInTwoFactorNewPathはサインイン時のTOTPチャレンジフォーム (2FA有効な
// アカウントがパスワードのステップの後に認証アプリのコードを入力してサインインを完了する)
// のパスを返します。
func SignInTwoFactorNewPath() Path {
	return Path("/sign_in/two_factor/new")
}

// SignInTwoFactorPathはサインイン2段階認証チャレンジのリソースのパスを返します。
// コードの送信はPOST /sign_in/two_factorでこれを対象とします。
func SignInTwoFactorPath() Path {
	return Path("/sign_in/two_factor")
}

// SignInTwoFactorRecoveryNewPathはサインイン時のリカバリーコードチャレンジフォーム
// (認証アプリを使えないとき、2FA有効なアカウントが保存済みのリカバリーコードを入力して
// サインインを完了する) のパスを返します。
func SignInTwoFactorRecoveryNewPath() Path {
	return Path("/sign_in/two_factor/recovery/new")
}

// SignInTwoFactorRecoveryPathはサインイン時のリカバリーコードチャレンジのリソースの
// パスを返します。コードの送信はPOST /sign_in/two_factor/recoveryでこれを対象とします。
func SignInTwoFactorRecoveryPath() Path {
	return Path("/sign_in/two_factor/recovery")
}

// HomePathはサインイン済みユーザーのホームページのパスを返します。
func HomePath() Path {
	return Path("/home")
}

// UserSessionPathはユーザーセッションリソースのパスを返します。サインアウトは
// DELETE /user_sessionでこれを対象とします (フォームは _methodオーバーライドで到達します)。
func UserSessionPath() Path {
	return Path("/user_session")
}

// SettingsPathは設定ハブのパスを返します。
func SettingsPath() Path {
	return Path("/settings")
}

// SettingsEmailEditPathはメールアドレス変更フォームのパスを返します。
func SettingsEmailEditPath() Path {
	return Path("/settings/email/edit")
}

// SettingsEmailPathは設定配下のemailリソースのパスを返します。変更申請は
// PATCH /settings/emailでこれを対象とします (フォームは _methodオーバーライドで到達します)。
func SettingsEmailPath() Path {
	return Path("/settings/email")
}

// SettingsEmailConfirmationNewPathはメールアドレス変更の確認コード入力フォームの
// パスを返します。
func SettingsEmailConfirmationNewPath() Path {
	return Path("/settings/email/confirmation/new")
}

// SettingsEmailConfirmationPathはメールアドレス変更の確認リソースのパスを返します。
// コードの送信はPOST /settings/email/confirmationでこれを対象とします。
func SettingsEmailConfirmationPath() Path {
	return Path("/settings/email/confirmation")
}

// SettingsTwoFactorAuthNewPathは2段階認証の設定フォーム (登録用secretを発行し
// QRコードを表示する) のパスを返します。
func SettingsTwoFactorAuthNewPath() Path {
	return Path("/settings/two_factor_auth/new")
}

// SettingsTwoFactorAuthPathは設定配下の2段階認証リソースのパスを返します。
// 有効化はPOST /settings/two_factor_authでこれを対象とします。
func SettingsTwoFactorAuthPath() Path {
	return Path("/settings/two_factor_auth")
}

// SettingsWithdrawalNewPathは退会確認フォームのパスを返します。
func SettingsWithdrawalNewPath() Path {
	return Path("/settings/withdrawal/new")
}

// SettingsWithdrawalPathは設定配下の退会リソースのパスを返します。退会の実行は
// DELETE /settings/withdrawalでこれを対象とします (フォームは _methodオーバーライドで
// 到達します)。
func SettingsWithdrawalPath() Path {
	return Path("/settings/withdrawal")
}

// CategoryPathは指定slugのカテゴリーのパスを返します。カテゴリーは掲示板の
// アドレスの一部ではなく、サイドバーで掲示板をまとめるものであるため、掲示板のパスが
// その下に並ぶ接頭辞ではなく自身のアドレスを持ちます。
//
// slugはそのままパスへ置きます。前提はBoardPathが記すものと同じで、
// model.IsValidSlugが受理する値であり、その規則が許す文字はいずれもURLの
// パスの中でそれ自身を表します。
func CategoryPath(slug string) Path {
	return Path("/c/" + slug)
}

// BoardPathは指定slugの掲示板のパスを返します。slugはカテゴリー配下のパスでは
// なく掲示板自身の識別子であるため、掲示板がカテゴリー間で移されてもアドレスは
// 保たれます。
//
// slugはそのままパスへ置くため、model.IsValidSlugが受理する値であることを前提と
// します。その規則が許す文字はいずれもURLのパスの中でそれ自身を表すためです。パスや
// クエリの文字を含むslugは掲示板ではないどこかを指すリンクを作ってしまいます。ここで
// エスケープするのではなく掲示板を作る側で規則を適用しているのはそのためです。
func BoardPath(slug string) Path {
	return Path("/b/" + slug)
}

// PostElementIDは、指定されたレス番号の投稿を描画する要素のidを返します。レス
// 番号はスレッドの中でのその投稿の永久アドレス (スレッドは全投稿を載せた1つのURLで
// 応答します。ADR 0009) であるため、本文の中の >>Nも、#p12で終わる共有されたリンクも、
// ブラウザがスクロールする先の要素も、これに解決します。
//
// これへのリンクを組み立てるPostAnchorの隣に置き、idとそれを指すリンクが1つの規則
// から導かれるようにしています。2箇所に書けば離れていくことがあり、もう一方の規則で
// アンカーを組み立てたページは、正しく見えてどこへもスクロールしません。
func PostElementID(number int) string {
	return "p" + strconv.Itoa(number)
}

// PostAnchorは、指定されたレス番号の投稿へのリンクを、同一文書内の参照として
// 返します。投稿は自身のアドレスを持ちません。スレッドは丸ごと配信されるため、投稿は
// それへのリンクが書かれるページの上に既にあります。
func PostAnchor(number int) Path {
	return Path("#" + PostElementID(number))
}

// ThreadPathは指定idのスレッドのパスを返します。slugではなくidで指すのは
// タイトルが編集されうるためで、掲示板配下のパスではなく自身のidで指すのは、
// モデレーターがスレッドを別の掲示板へ移してもアドレスが保たれるようにするためです。
//
// Presentation層のid型を受け取るため、呼び出し側が別のエンティティのidを渡して
// 別のスレッドへ繋がるパスを得ることはできません。
func ThreadPath(id viewmodel.ThreadID) Path {
	return Path("/t/" + id.String())
}

// ThreadPostAnchorPathは、スレッドの投稿1件への、そのスレッドではないページからの
// リンクを返します。そのスレッドであるページのための同じリンクはPostAnchorが書き、そこでは
// フラグメントだけでその投稿へ届きます。別の場所に立つページは、フラグメントが意味を持つ前に、
// 投稿が一緒に配信されるスレッドを名指さなければなりません。
func ThreadPostAnchorPath(id viewmodel.ThreadID, number int) Path {
	return ThreadPath(id) + PostAnchor(number)
}

// BoardThreadsPathは指定slugの掲示板のスレッドのパス、すなわち新しいスレッドが
// 加えられるコレクションを返します。このアドレスへは書き込むだけです。作られたスレッドは
// 自身のidのThreadPathで読まれ、別の掲示板へ移してもそこへのリンクが保たれます。
func BoardThreadsPath(slug string) Path {
	return BoardPath(slug) + "/threads"
}

// BoardThreadsNewPathは指定slugの掲示板でスレッドを立てるフォームのパスを
// 返します。掲示板はフォームで選ぶのではなくアドレスが名指すため、フォームは投稿先の
// 掲示板から開かれ、それ以外の場所にスレッドを立てることはできません。
func BoardThreadsNewPath(slug string) Path {
	return BoardThreadsPath(slug) + "/new"
}

// ThreadPostsPathは指定idのスレッドの投稿のパス、すなわち返信が加えられる
// コレクションを返します。このアドレスへは書き込むだけです。保存された投稿はスレッド
// 自身のページで読まれ、そこでは与えられたレス番号で名指されます (ADR 0009)。
func ThreadPostsPath(id viewmodel.ThreadID) Path {
	return ThreadPath(id) + "/posts"
}

// ThreadLockPathは指定idのスレッドのロック、すなわちモデレーターがそのスレッドに
// 掛け、また外すもののパスを返します。掛けることと外すことが同じアドレスであるのは、どちらも
// 1つの同じロックに対して行われるためです。スレッドが持つのは、ロックが作られまた取り払われる
// 1つの場所であって、それ自身の2つの動詞ではありません。
func ThreadLockPath(id viewmodel.ThreadID) Path {
	return ThreadPath(id) + "/lock"
}

// ThreadLockNewPathは、スレッドのロックを確認するページのパスを返します。スレッドは
// ページ上で選ぶのではなくアドレスが名指すため、このページは自身が閉じるスレッドから開かれ、
// それ以外のスレッドを閉じることはできません。
func ThreadLockNewPath(id viewmodel.ThreadID) Path {
	return ThreadLockPath(id) + "/new"
}

// ThreadUnpublicationPathは、指定idのスレッドの非公開、すなわちスレッドをコミュニティ
// の視界から外す印のパスを返します。印の下でもスレッドはタイトルと投稿を保つため、これが
// 名指すのはスレッドの削除ではなく印です。
func ThreadUnpublicationPath(id viewmodel.ThreadID) Path {
	return ThreadPath(id) + "/unpublication"
}

// ThreadUnpublicationNewPathは、スレッドの非公開を確認するページのパスを返します。
// スレッドはページ上で選ぶのではなくアドレスが名指すため、このページは自身が視界から外す
// スレッドから開かれ、それ以外のスレッドを外すことはできません。
func ThreadUnpublicationNewPath(id viewmodel.ThreadID) Path {
	return ThreadUnpublicationPath(id) + "/new"
}

// PostUnpublicationPathは、スレッドの投稿1件の非公開のパスを返します。投稿はスレッド
// とレス番号で名指されます。それが、投稿が参照されるあらゆる場所でのその投稿の指し方である
// ためです (ADR 0009)。投稿はどのアドレスにも自身のidを持たないため、>>Nの中でそれを名指す
// 組が、ここでもそれを名指します。
func PostUnpublicationPath(id viewmodel.ThreadID, number int) Path {
	return ThreadPostsPath(id) + Path("/"+strconv.Itoa(number)) + "/unpublication"
}

// PostUnpublicationNewPathは、投稿の非公開を確認するページのパスを返します。投稿を
// アドレスが名指すのは、その上のページでスレッドがそうされるのと同じ理由です。このページは
// 自身が視界から外す投稿から開かれ、それ以外の投稿を外すことはできません。
func PostUnpublicationNewPath(id viewmodel.ThreadID, number int) Path {
	return PostUnpublicationPath(id, number) + "/new"
}

// AdminPathは管理ハブ、すなわちコミュニティの管理画面を並べるページのパスを
// 返します。管理画面をコミュニティのページの隣ではなく専用の接頭辞の下に置くのは、
// 管理者にだけ許されるものと誰でも開けるものを、アドレスだけで見分けられるようにする
// ためです。
func AdminPath() Path {
	return Path("/admin")
}

// AdminUsersPathは管理画面の利用者一覧、すなわちコミュニティの利用者を読み、
// その人たちにロールを渡す場所のパスを返します。
func AdminUsersPath() Path {
	return AdminPath() + "/users"
}

// AdminUsersQueryParamは利用者一覧を絞り込むクエリパラメータで、atnameの先頭
// 部分を運びます。パスヘルパーの傍らに置くのは、検索フォームがこれをフィールドとして
// 名指し、ハンドラーがアドレスから読み戻すためで、両者は同じ綴りである必要があります。
const AdminUsersQueryParam = "q"

// PageParamは一覧のどのページを読んでいるかを名指すクエリパラメータで、1から
// 数えます。1つの一覧に閉じないのは、同じ形でページを送る後続の一覧も、同じ名前で
// ページ番号を運ぶためです。
const PageParam = "page"

// AdminUserRoleNameParamは、付与が渡すロールを名指すフォームのフィールドです。
// パスヘルパーの傍らに置く理由はAdminUsersQueryParamと同じで、行の付与フォームが
// これをフィールドとして名指し、ハンドラーが送信から読み戻すためです。両者は同じ綴りで
// ある必要があります。
//
// 剥奪はこのフィールドを持ちません。取り除かれる割当を名指すのは、送信ではなくアドレスで
// あるためです。
const AdminUserRoleNameParam = "role_name"

// AdminUsersPagePathは管理画面の利用者一覧の1ページ、すなわちatnameが
// atnamePrefixで始まるアカウントに絞り込んだ一覧のパスを返します。ページ送りのリンクが
// これで組み立てられるため、次のページへ移っても、全アカウントから始め直すのではなく
// 一覧の絞り込みが保たれます。
//
// 最初のページはページ番号を綴らずに表します。それが一覧を開くアドレスであり、検索
// フォームの送信先でもあるためです。空のprefixも同様にパラメータを落とします。それは
// 一覧を何も絞り込んでおらず、空の値を運べば同じ一覧が2つのアドレスを持つことになる
// ためです。
func AdminUsersPagePath(atnamePrefix string, page int) Path {
	query := url.Values{}
	if atnamePrefix != "" {
		query.Set(AdminUsersQueryParam, atnamePrefix)
	}
	if page > 1 {
		query.Set(PageParam, strconv.Itoa(page))
	}
	if len(query) == 0 {
		return AdminUsersPath()
	}

	return AdminUsersPath() + Path("?"+query.Encode())
}

// AdminUserRolesPathは指定idの利用者のロール、すなわちロールが加えられる
// コレクションのパスを返します。ロールの付与はこのアドレスへ書き込むだけで、付与される
// ロールはアドレスではなく送信が名指します。どのロールを加えてもコレクションは同じで
// あるためです。
func AdminUserRolesPath(id viewmodel.UserID) Path {
	return AdminUsersPath() + Path("/"+id.String()) + "/roles"
}

// AdminUserRolePathは指定idの利用者が持つ1つのロール、すなわち剥奪が取り除く
// 割当のパスを返します。ロールをアドレスで名指すのは、取り除かれるのがその1つの割当で
// あり、アドレスが表すのがアカウントのロール全体ではなくそれであるためです。
//
// 名前は、掲示板やカテゴリのパスがスラッグをそうするのと同じく、そのままパスへ書きます。
// ロールに名前を与えるのはこのインスタンスが同梱したマイグレーションであり、ここでアドレスが
// 運ぶ名前は、コードが既に綴っているものであるためです。
func AdminUserRolePath(id viewmodel.UserID, roleName string) Path {
	return AdminUserRolesPath(id) + Path("/"+roleName)
}
