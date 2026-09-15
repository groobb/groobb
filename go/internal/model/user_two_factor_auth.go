package model

import "time"

// UserTwoFactorAuthはユーザーのTOTPによる2段階認証設定で、専用テーブル
// (ユーザーあたり1行) に置くことで2FAの資格情報を身元から分離します
// (UserPasswordがパスワード資格情報を分離するのと同じ)。行はユーザーが登録中の間は
// 存在し (Secret発行済み、Enabledはfalse)、TOTPコードの確認後に有効になります
// (Enabledがtrue、EnabledAtが設定される)。SecretはTOTPの共有シークレット、
// RecoveryCodesは1回使い切りのバックアップコードで、いずれも平文で保存します
// (マイグレーションを参照)。使用したリカバリーコードはスライスから削除します。EnabledAtは
// 設定が有効化されるまでnilです。UserIDは所有ユーザーで、テーブル内で一意です。
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
