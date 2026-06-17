package model

import "time"

// UserPassword is a user's native password credential, kept in its own table
// rather than on the user so that identity and authentication methods stay
// separate: a user authenticating only through SSO has no UserPassword, while a
// native user has exactly one. PasswordDigest is a bcrypt hash of the chosen
// password; the plaintext is never stored. UserID is the owning user, and a
// user has at most one password (enforced by a UNIQUE constraint on user_id).
//
// [Ja] UserPassword はユーザーの native パスワード資格情報で、ユーザー本体ではなく
// 専用テーブルに置くことで身元と認証手段を分離します。SSO のみで認証するユーザーは
// UserPassword を持たず、native ユーザーはちょうど 1 つ持ちます。PasswordDigest は
// 選んだパスワードの bcrypt ハッシュで、平文は保存しません。UserID は所有ユーザーで、
// ユーザーは高々 1 つのパスワードを持ちます (user_id の UNIQUE 制約で強制)。
type UserPassword struct {
	ID             UserPasswordID
	UserID         UserID
	PasswordDigest string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
