// modelパッケージはGroobbのドメインエンティティと、それを構成する値型
// (ここで定義する型付きエンティティIDなど) を保持します。
package model

import "strconv"

// UserIDはユーザーの型付き識別子です。
//
// データベースが採番するINTEGER PRIMARY KEYの型であるint64を、その型のまま
// 使わずラップするのは、異なるエンティティのIDを取り違えて代入できないように
// するためです。別エンティティのIDが期待される箇所にUserIDを渡すとコンパイラが
// 拒否します。
type UserID int64

// StringはUserIDを10進表記で返します。
func (id UserID) String() string { return strconv.FormatInt(int64(id), 10) }

// ParseUserIDは、Stringが書く10進表記からUserIDを読み取り、そもそもrawが
// それを表しているかどうかを併せて返します。Stringの隣に置く理由はParseThreadIDが
// ThreadIDのStringの隣にある理由と同じで、何が利用者のアドレスであるかを1度で決め、
// 同じアカウントを名指すどのルートもそれに従うようにするためです。
//
// 正の整数でないものはルックアップせずに拒否します。そのようなidを持つアカウントは
// 無いためです。
func ParseUserID(raw string) (UserID, bool) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}

	return UserID(id), true
}

// UserSessionIDはユーザーセッションの型付き識別子です。UserIDと同様に
// int64をラップし、セッションIDを他エンティティのIDと取り違えられない
// ようにします。
type UserSessionID int64

// StringはUserSessionIDを10進表記で返します。
func (id UserSessionID) String() string { return strconv.FormatInt(int64(id), 10) }

// EmailConfirmationIDはメール確認の型付き識別子です。UserIDと同様に
// int64をラップし、確認IDを他エンティティのIDと取り違えられないように
// します。
type EmailConfirmationID int64

// StringはEmailConfirmationIDを10進表記で返します。
func (id EmailConfirmationID) String() string { return strconv.FormatInt(int64(id), 10) }

// UserPasswordIDはユーザーのパスワード資格情報の型付き識別子です。UserIDと
// 同様にint64をラップし、パスワードIDを他エンティティのIDと取り違えられ
// ないようにします。
type UserPasswordID int64

// StringはUserPasswordIDを10進表記で返します。
func (id UserPasswordID) String() string { return strconv.FormatInt(int64(id), 10) }

// PasswordResetTokenIDはパスワードリセットトークンの型付き識別子です。UserIDと
// 同様にint64をラップし、リセットトークンIDを他エンティティのIDと取り違え
// られないようにします。
type PasswordResetTokenID int64

// StringはPasswordResetTokenIDを10進表記で返します。
func (id PasswordResetTokenID) String() string { return strconv.FormatInt(int64(id), 10) }

// UserTwoFactorAuthIDはユーザーの2段階認証設定の型付き識別子です。UserIDと
// 同様にint64をラップし、2FA IDを他エンティティのIDと取り違えられないように
// します。
type UserTwoFactorAuthID int64

// StringはUserTwoFactorAuthIDを10進表記で返します。
func (id UserTwoFactorAuthID) String() string { return strconv.FormatInt(int64(id), 10) }

// CommunityIDはコミュニティの型付き識別子です。1インスタンスがちょうど1つの
// コミュニティを運営し (ADR 0006)、その行が常にid 1であっても、UserIDと同様に
// int64をラップし、コミュニティIDを他エンティティのIDと取り違えられないように
// します。
type CommunityID int64

// StringはCommunityIDを10進表記で返します。
func (id CommunityID) String() string { return strconv.FormatInt(int64(id), 10) }

// CategoryIDはカテゴリーの型付き識別子です。UserIDと同様にint64をラップし、
// カテゴリーIDを他エンティティのIDと取り違えられないようにします。
type CategoryID int64

// StringはCategoryIDを10進表記で返します。
func (id CategoryID) String() string { return strconv.FormatInt(int64(id), 10) }

// BoardIDは掲示板の型付き識別子です。UserIDと同様にint64をラップし、
// 掲示板IDを他エンティティのIDと取り違えられないようにします。
type BoardID int64

// StringはBoardIDを10進表記で返します。
func (id BoardID) String() string { return strconv.FormatInt(int64(id), 10) }

// ThreadIDはスレッドの型付き識別子です。UserIDと同様にint64をラップし、
// スレッドIDを他エンティティのIDと取り違えられないようにします。
type ThreadID int64

// StringはThreadIDを10進表記で返します。
func (id ThreadID) String() string { return strconv.FormatInt(int64(id), 10) }

// ParseThreadIDは、Stringが書く10進表記からThreadIDを読み取り、そもそもrawが
// それを表しているかどうかを併せて返します。2つを並べて置くのは、何がスレッドのアドレスで
// あるかを1度で決め、同じスレッドを指すどのルートもそれに従うようにするためです。
//
// 正の整数でないものはルックアップせずに拒否します。そのようなidを持つスレッドは無く、
// ルックアップしてもクエリを1回発行した末に不在と答えるだけだからです。strconvが読み取る
// 数の周りに認める綴り (先頭のゼロやプラス記号) はここでも受け付けるため、1つのアドレスで
// 応答する呼び出し側は、rawと解析したidのStringを突き合わせ、異なるときにリダイレクト
// します。
func ParseThreadID(raw string) (ThreadID, bool) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}

	return ThreadID(id), true
}

// PostIDは投稿の型付き識別子です。UserIDと同様にint64をラップし、投稿IDを
// 他エンティティのIDと取り違えられないようにします。
type PostID int64

// StringはPostIDを10進表記で返します。
func (id PostID) String() string { return strconv.FormatInt(int64(id), 10) }

// PostReferenceIDはレス参照の型付き識別子です。UserIDと同様にint64をラップし、
// 参照IDを他エンティティのIDと取り違えられないようにします。
type PostReferenceID int64

// StringはPostReferenceIDを10進表記で返します。
func (id PostReferenceID) String() string { return strconv.FormatInt(int64(id), 10) }

// RoleIDはロールの型付き識別子です。UserIDと同様にint64をラップし、ロールIDを
// 他エンティティのIDと取り違えられないようにします。
type RoleID int64

// StringはRoleIDを10進表記で返します。
func (id RoleID) String() string { return strconv.FormatInt(int64(id), 10) }

// UserRoleIDはロール割当の型付き識別子です。UserIDと同様にint64をラップし、
// 割当IDを他エンティティのIDと取り違えられないようにします。
type UserRoleID int64

// StringはUserRoleIDを10進表記で返します。
func (id UserRoleID) String() string { return strconv.FormatInt(int64(id), 10) }

// ModerationLogIDは操作履歴の1件の型付き識別子です。UserIDと同様にint64を
// ラップし、履歴IDを他エンティティのIDと取り違えられないようにします。
type ModerationLogID int64

// StringはModerationLogIDを10進表記で返します。
func (id ModerationLogID) String() string { return strconv.FormatInt(int64(id), 10) }
