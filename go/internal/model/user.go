package model

import "time"

// User is the global, canonical identity that authentication hangs off of. It
// holds only identity-level attributes: email (also the contact address), and
// the account-level locale and time zone used when rendering messages outside
// of any space context (e.g. password-reset emails rendered asynchronously).
//
// Presentation and role attributes (atname, display name, role) deliberately
// live on space_members in a later plan, and the password digest is kept in a
// separate credentials table, so they are absent here.
//
// [Ja] User は認証がぶら下がるグローバルで正準な身元。身元レベルの属性のみを持つ。
// すなわち email (連絡先も兼ねる) と、スペース文脈の外でメッセージを描画するときに
// 使うアカウントレベルの locale / time zone (例: 非同期に描画するパスワード
// リセットメール) である。
//
// 表示・権限の属性 (atname / 表示名 / ロール) は意図的に後続計画で space_members に
// 置き、パスワードダイジェストは別の資格情報テーブルに保持するため、ここには無い。
type User struct {
	ID        UserID
	Email     string
	Locale    string
	TimeZone  string
	CreatedAt time.Time
	UpdatedAt time.Time
}
