package model

import "time"

// UserTwoFactorAuth is a user's TOTP-based two-factor authentication setting,
// kept in its own table (one row per user) so the 2FA credential material stays
// separate from identity, the same way UserPassword isolates the password
// credential. A row exists while the user is enrolling (Secret issued, Enabled
// false) and becomes active once they confirm a TOTP code (Enabled true,
// EnabledAt set). Secret is the TOTP shared secret and RecoveryCodes are the
// one-time backup codes; both are stored in plaintext (see the migration), and a
// used recovery code is removed from the slice. EnabledAt is nil until the
// setting is enabled. UserID is the owning user, unique across the table.
//
// [Ja] UserTwoFactorAuth はユーザーの TOTP による 2 段階認証設定で、専用テーブル
// (ユーザーあたり 1 行) に置くことで 2FA の資格情報を身元から分離します
// (UserPassword がパスワード資格情報を分離するのと同じ)。行はユーザーが登録中の間は
// 存在し (Secret 発行済み、Enabled は false)、TOTP コードの確認後に有効になります
// (Enabled が true、EnabledAt が設定される)。Secret は TOTP の共有シークレット、
// RecoveryCodes は 1 回使い切りのバックアップコードで、いずれも平文で保存します
// (マイグレーションを参照)。使用したリカバリーコードはスライスから削除します。EnabledAt は
// 設定が有効化されるまで nil です。UserID は所有ユーザーで、テーブル内で一意です。
type UserTwoFactorAuth struct {
	ID            UserTwoFactorAuthID
	UserID        UserID
	Secret        string
	Enabled       bool
	EnabledAt     *time.Time
	RecoveryCodes []string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
