package templates

import (
	"net/url"

	"github.com/a-h/templ"
)

// ReturnToParam is the query and form parameter that carries where to send a
// visitor once they finish signing in. RequireAuth writes it when it turns an
// anonymous request away, and the sign-in forms hand it along until a session is
// issued. Its value is only ever trusted after middleware.SanitizeReturnTo has
// accepted it.
//
// [Ja] ReturnToParam はサインインを終えた訪問者をどこへ送るかを運ぶクエリ / フォーム
// パラメータです。RequireAuth が匿名リクエストを追い返すときに書き込み、サインイン系の
// フォームがセッション発行まで引き継ぎます。値は middleware.SanitizeReturnTo を通った
// ものだけを信頼します。
const ReturnToParam = "return_to"

// Path represents a URL path within the application. Centralizing path strings
// here lets templates link to routes without hard-coding literals, so a route
// change is made in one place instead of across every template.
//
// [Ja] Path はアプリケーション内の URL パスを表す型です。パス文字列をここに集約
// することで、テンプレートはリテラルをハードコードせずにルートへリンクでき、ルート
// 変更を各テンプレートではなく 1 箇所で行えます。
type Path string

// String returns the path as a string.
//
// [Ja] String はパスを文字列として返します。
func (p Path) String() string {
	return string(p)
}

// SafeURL returns the path as a templ.SafeURL for use in href / action attributes.
//
// [Ja] SafeURL はパスを href / action 属性で使うための templ.SafeURL として返します。
func (p Path) SafeURL() templ.SafeURL {
	return templ.SafeURL(p)
}

// WithReturnTo returns the path carrying returnTo in the return_to query
// parameter. RequireAuth builds the first such URL when it turns an anonymous
// request away, and each later hop rebuilds it — the password step on to the TOTP
// challenge, the links between the TOTP and recovery-code challenges, and the
// restart back to sign-in when a challenge is gone — so the destination the
// visitor was originally headed for survives to the step that issues the session.
// An empty returnTo leaves the path untouched, which is how a flow that carries
// no destination stays on the bare path. The caller passes a value that
// middleware.SanitizeReturnTo has already accepted; this method builds the URL
// and does not re-check it. The receiver must be a path without a query string,
// as the route helpers in this file return: the parameter is appended after a
// literal "?", so a receiver that already carries a query would produce two.
// AfterSignInPath is the one helper here that does not qualify, since it hands
// back the caller's destination unchanged and that may carry a query.
//
// [Ja] WithReturnTo は returnTo を return_to クエリパラメータに載せたパスを返します。
// 最初のこの URL は RequireAuth が匿名リクエストを追い返すときに組み立て、以降の各ホップ
// (パスワードのステップから TOTP チャレンジへ、TOTP とリカバリーコードのチャレンジを
// 行き来するリンク、チャレンジが失われたときのサインインへのやり直し) が組み立て直します。
// これにより、訪問者が本来向かっていた遷移先はセッションを発行するステップまで残ります。
// returnTo が空のときはパスをそのまま返し、遷移先を持たないフローは素のパスのままになり
// ます。呼び出し側は middleware.SanitizeReturnTo が受け付け済みの値を渡します。本メソッド
// は URL を組み立てるだけで、値の再検証は行いません。レシーバーは、本ファイルのルート
// ヘルパーが返すとおりクエリを持たないパスであることを前提とします。パラメータはリテラルの
// "?" の後ろに連結するため、既にクエリを持つレシーバーでは "?" が 2 つ並んでしまいます。
// 本ファイルで唯一これに当てはまらないのが AfterSignInPath で、呼び出し側の遷移先を
// そのまま返すためクエリを含みうります。
func (p Path) WithReturnTo(returnTo string) Path {
	if returnTo == "" {
		return p
	}

	return Path(string(p) + "?" + url.Values{ReturnToParam: {returnTo}}.Encode())
}

// RootPath returns the path to the top page.
//
// [Ja] RootPath はトップページのパスを返します。
func RootPath() Path {
	return Path("/")
}

// AfterSignInPath returns where to send a visitor once their session is issued:
// the destination the sign-in flow carried in returnTo, or the home page when the
// flow carried none. The three routes that issue a session (password, TOTP, and
// recovery code) share it so they land the visitor in the same place. Home rather
// than the top page, because the top page turns a signed-in visitor away to home
// anyway; sending them there directly saves that extra redirect hop. returnTo is
// a value middleware.SanitizeReturnTo has already accepted.
//
// [Ja] AfterSignInPath はセッション発行後に訪問者を送る先を返します。サインインフローが
// returnTo で運んできた遷移先、運んでこなかったときはホームです。セッションを発行する
// 3 つのルート (パスワード・TOTP・リカバリーコード) がこれを共有し、訪問者を同じ場所へ
// 着地させます。トップページではなくホームなのは、トップページがサインイン済みの訪問者を
// 結局ホームへ送るためです。直接ホームへ送ればその 1 段分のリダイレクトを省けます。
// returnTo は middleware.SanitizeReturnTo が受け付け済みの値です。
func AfterSignInPath(returnTo string) Path {
	if returnTo == "" {
		return HomePath()
	}

	return Path(returnTo)
}

// SignUpPath returns the path to the sign-up form.
//
// [Ja] SignUpPath はサインアップフォームのパスを返します。
func SignUpPath() Path {
	return Path("/sign_up")
}

// SignInPath returns the path to the sign-in form.
//
// [Ja] SignInPath はサインインフォームのパスを返します。
func SignInPath() Path {
	return Path("/sign_in")
}

// SignInTwoFactorNewPath returns the path to the sign-in TOTP challenge form,
// where a 2FA-enabled account enters an authenticator code to finish signing in
// after the password step.
//
// [Ja] SignInTwoFactorNewPath はサインイン時の TOTP チャレンジフォーム (2FA 有効な
// アカウントがパスワードのステップの後に認証アプリのコードを入力してサインインを完了する)
// のパスを返します。
func SignInTwoFactorNewPath() Path {
	return Path("/sign_in/two_factor/new")
}

// SignInTwoFactorPath returns the path to the sign-in two-factor challenge
// resource. Submitting the code targets it with POST /sign_in/two_factor.
//
// [Ja] SignInTwoFactorPath はサインイン 2 段階認証チャレンジのリソースのパスを返します。
// コードの送信は POST /sign_in/two_factor でこれを対象とします。
func SignInTwoFactorPath() Path {
	return Path("/sign_in/two_factor")
}

// SignInTwoFactorRecoveryNewPath returns the path to the sign-in recovery-code
// challenge form, the fallback where a 2FA-enabled account enters a saved recovery
// code to finish signing in when the authenticator app is unavailable.
//
// [Ja] SignInTwoFactorRecoveryNewPath はサインイン時のリカバリーコードチャレンジフォーム
// (認証アプリを使えないとき、2FA 有効なアカウントが保存済みのリカバリーコードを入力して
// サインインを完了する) のパスを返します。
func SignInTwoFactorRecoveryNewPath() Path {
	return Path("/sign_in/two_factor/recovery/new")
}

// SignInTwoFactorRecoveryPath returns the path to the sign-in recovery-code
// challenge resource. Submitting the code targets it with POST
// /sign_in/two_factor/recovery.
//
// [Ja] SignInTwoFactorRecoveryPath はサインイン時のリカバリーコードチャレンジのリソースの
// パスを返します。コードの送信は POST /sign_in/two_factor/recovery でこれを対象とします。
func SignInTwoFactorRecoveryPath() Path {
	return Path("/sign_in/two_factor/recovery")
}

// HomePath returns the path to the signed-in home page.
//
// [Ja] HomePath はサインイン済みユーザーのホームページのパスを返します。
func HomePath() Path {
	return Path("/home")
}

// UserSessionPath returns the path to the user session resource. Signing out
// targets it with DELETE /user_session (forms reach it via the _method override).
//
// [Ja] UserSessionPath はユーザーセッションリソースのパスを返します。サインアウトは
// DELETE /user_session でこれを対象とします (フォームは _method オーバーライドで到達します)。
func UserSessionPath() Path {
	return Path("/user_session")
}

// SettingsPath returns the path to the settings hub.
//
// [Ja] SettingsPath は設定ハブのパスを返します。
func SettingsPath() Path {
	return Path("/settings")
}

// SettingsEmailEditPath returns the path to the email-change form.
//
// [Ja] SettingsEmailEditPath はメールアドレス変更フォームのパスを返します。
func SettingsEmailEditPath() Path {
	return Path("/settings/email/edit")
}

// SettingsEmailPath returns the path to the email resource under settings. The
// change request targets it with PATCH /settings/email (the form reaches it via
// the _method override).
//
// [Ja] SettingsEmailPath は設定配下の email リソースのパスを返します。変更申請は
// PATCH /settings/email でこれを対象とします (フォームは _method オーバーライドで到達します)。
func SettingsEmailPath() Path {
	return Path("/settings/email")
}

// SettingsEmailConfirmationNewPath returns the path to the email-change
// confirmation-code entry form.
//
// [Ja] SettingsEmailConfirmationNewPath はメールアドレス変更の確認コード入力フォームの
// パスを返します。
func SettingsEmailConfirmationNewPath() Path {
	return Path("/settings/email/confirmation/new")
}

// SettingsEmailConfirmationPath returns the path to the email-change confirmation
// resource. Submitting the code targets it with POST /settings/email/confirmation.
//
// [Ja] SettingsEmailConfirmationPath はメールアドレス変更の確認リソースのパスを返します。
// コードの送信は POST /settings/email/confirmation でこれを対象とします。
func SettingsEmailConfirmationPath() Path {
	return Path("/settings/email/confirmation")
}

// SettingsTwoFactorAuthNewPath returns the path to the two-factor authentication
// setup form (which issues the enrollment secret and shows the QR code).
//
// [Ja] SettingsTwoFactorAuthNewPath は 2 段階認証の設定フォーム (登録用 secret を発行し
// QR コードを表示する) のパスを返します。
func SettingsTwoFactorAuthNewPath() Path {
	return Path("/settings/two_factor_auth/new")
}

// SettingsTwoFactorAuthPath returns the path to the two-factor authentication
// resource under settings. Enabling it targets this with POST
// /settings/two_factor_auth.
//
// [Ja] SettingsTwoFactorAuthPath は設定配下の 2 段階認証リソースのパスを返します。
// 有効化は POST /settings/two_factor_auth でこれを対象とします。
func SettingsTwoFactorAuthPath() Path {
	return Path("/settings/two_factor_auth")
}

// SettingsWithdrawalNewPath returns the path to the account-withdrawal
// confirmation form.
//
// [Ja] SettingsWithdrawalNewPath は退会確認フォームのパスを返します。
func SettingsWithdrawalNewPath() Path {
	return Path("/settings/withdrawal/new")
}

// SettingsWithdrawalPath returns the path to the withdrawal resource under
// settings. Executing the withdrawal targets it with DELETE /settings/withdrawal
// (the form reaches it via the _method override).
//
// [Ja] SettingsWithdrawalPath は設定配下の退会リソースのパスを返します。退会の実行は
// DELETE /settings/withdrawal でこれを対象とします (フォームは _method オーバーライドで
// 到達します)。
func SettingsWithdrawalPath() Path {
	return Path("/settings/withdrawal")
}

// CommunityNewPath returns the path to the community-creation form.
//
// [Ja] CommunityNewPath はコミュニティ作成フォームのパスを返します。
func CommunityNewPath() Path {
	return Path("/communities/new")
}

// CommunityListPath returns the path to the community collection. Creating a
// community targets it with POST /communities.
//
// [Ja] CommunityListPath はコミュニティのコレクションのパスを返します。コミュニティの
// 作成は POST /communities でこれを対象とします。
func CommunityListPath() Path {
	return Path("/communities")
}

// CommunityPath returns the path to a community's own page. Pages belonging to a
// community live under the short /c/ prefix rather than /communities, so the URL
// stays short as boards and posts nest beneath it and no identifier can collide
// with a route in the collection namespace. identifier is a value the
// community-creation validator has accepted (ASCII letters, digits, and hyphens),
// so it needs no escaping here.
//
// [Ja] CommunityPath はコミュニティ自身の画面のパスを返します。コミュニティに属する画面は
// /communities ではなく短縮した /c/ 接頭辞の下に置き、掲示板や投稿がその下に入れ子に
// なっても URL を短く保ち、識別子がコレクション名前空間のルートと衝突しないようにします。
// identifier はコミュニティ作成のバリデーターが受け付けた値 (ASCII 英数字とハイフン) の
// ため、ここでのエスケープは不要です。
func CommunityPath(identifier string) Path {
	return Path("/c/" + identifier)
}
