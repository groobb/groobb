package model

import "time"

// UserPasswordはユーザーのnativeパスワード資格情報で、ユーザー本体ではなく
// 専用テーブルに置くことで身元と認証手段を分離します。SSOのみで認証するユーザーは
// UserPasswordを持たず、nativeユーザーはちょうど1つ持ちます。PasswordDigestは
// 選んだパスワードのbcryptハッシュで、平文は保存しません。UserIDは所有ユーザーで、
// ユーザーは高々1つのパスワードを持ちます (user_idのUNIQUE制約で強制)。
type UserPassword struct {
	ID             UserPasswordID
	UserID         UserID
	PasswordDigest string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
