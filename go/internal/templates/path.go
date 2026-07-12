package templates

import (
	"github.com/a-h/templ"
)

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
