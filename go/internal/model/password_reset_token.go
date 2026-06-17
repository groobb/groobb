package model

import "time"

// PasswordResetTokenExpirationDuration is how long a password reset link stays
// valid after it is issued. One hour is long enough for a user to read the mail
// and follow the link, but short enough to limit the window in which a leaked
// link could be used; the value matches the sister Korylus projects.
//
// [Ja] PasswordResetTokenExpirationDuration はパスワードリセットリンクが発行後に
// 有効であり続ける期間です。1 時間はユーザーがメールを読んでリンクをたどるのに十分で、
// かつ漏えいしたリンクが使える期間を抑えられる短さです。値は姉妹 Korylus プロジェクトに
// 揃えています。
const PasswordResetTokenExpirationDuration = time.Hour

// PasswordResetToken is one one-time token issued when a user asks to reset
// their password. The plaintext token is mailed to the user inside a reset link
// and never persisted; only its hash is kept in TokenDigest, so a database leak
// does not expose usable tokens. UserID is the user the token resets the password
// for, ExpiresAt bounds how long the link works, and UsedAt is nil until the
// token is spent (stamped on use so a link cannot be replayed).
//
// [Ja] PasswordResetToken は、ユーザーがパスワードのリセットを申請したときに発行される
// 1 つの使い捨てトークンです。平文トークンはリセットリンクに入れてユーザーへメールし、
// 永続化はしません。ハッシュだけを TokenDigest に持つため、DB が漏えいしても使える
// トークンは露出しません。UserID はトークンがパスワードをリセットする対象のユーザー、
// ExpiresAt はリンクが有効な期間を区切り、UsedAt はトークンが使われるまで nil です
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

// IsUsed reports whether the token has already been spent. UsedAt is stamped the
// moment the token completes a password update, so a non-nil UsedAt marks a
// one-time link that must not be replayed.
//
// [Ja] IsUsed はトークンが既に消費済みかを返します。UsedAt はトークンがパスワード更新を
// 完了した時点で打刻されるため、非 nil の UsedAt は再利用してはならない使い捨てリンクを
// 表します。
func (t *PasswordResetToken) IsUsed() bool {
	return t.UsedAt != nil
}

// IsExpired reports whether the token's validity window has passed. Expiry is
// checked against the current time, so a token issued more than
// PasswordResetTokenExpirationDuration ago no longer permits a reset.
//
// [Ja] IsExpired はトークンの有効期間が過ぎたかを返します。有効期限は現在時刻と照合する
// ため、PasswordResetTokenExpirationDuration より前に発行されたトークンはもうリセットを
// 許可しません。
func (t *PasswordResetToken) IsExpired() bool {
	return time.Now().After(t.ExpiresAt)
}
