package model

import "time"

// PasswordResetTokenExpirationDurationはパスワードリセットリンクが発行後に
// 有効であり続ける期間です。1時間はユーザーがメールを読んでリンクをたどるのに十分で、
// かつ漏えいしたリンクが使える期間を抑えられる短さです。値は姉妹Korylusプロジェクトに
// 揃えています。
const PasswordResetTokenExpirationDuration = time.Hour

// PasswordResetTokenは、ユーザーがパスワードのリセットを申請したときに発行される
// 1つの使い捨てトークンです。平文トークンはリセットリンクに入れてユーザーへメールし、
// 永続化はしません。ハッシュだけをTokenDigestに持つため、DBが漏えいしても使える
// トークンは露出しません。UserIDはトークンがパスワードをリセットする対象のユーザー、
// ExpiresAtはリンクが有効な期間を区切り、UsedAtはトークンが使われるまでnilです
// (使用時に打刻し、リンクの再利用を防ぎます)。
type PasswordResetToken struct {
	ID          PasswordResetTokenID
	UserID      UserID
	TokenDigest string
	ExpiresAt   time.Time
	UsedAt      *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// IsUsedはトークンが既に消費済みかを返します。UsedAtはトークンがパスワード更新を
// 完了した時点で打刻されるため、非nilのUsedAtは再利用してはならない使い捨てリンクを
// 表します。
func (t *PasswordResetToken) IsUsed() bool {
	return t.UsedAt != nil
}

// IsExpiredはトークンの有効期間が過ぎたかを返します。有効期限は現在時刻と照合する
// ため、PasswordResetTokenExpirationDurationより前に発行されたトークンはもうリセットを
// 許可しません。
func (t *PasswordResetToken) IsExpired() bool {
	return time.Now().After(t.ExpiresAt)
}
