// Package model holds Groobb's domain entities and the value types they are
// built from, such as the typed entity IDs defined here.
//
// [Ja] model パッケージは Groobb のドメインエンティティと、それを構成する値型
// (ここで定義する型付きエンティティ ID など) を保持します。
package model

import "strconv"

// UserID is the typed identifier for a user.
//
// It wraps int64, the type of the INTEGER PRIMARY KEY the database assigns,
// rather than being that type, so that IDs of different entities cannot be
// assigned to one another by mistake: the compiler rejects passing a UserID
// where another entity's ID is expected.
//
// [Ja] UserID はユーザーの型付き識別子です。
//
// データベースが採番する INTEGER PRIMARY KEY の型である int64 を、その型のまま
// 使わずラップするのは、異なるエンティティの ID を取り違えて代入できないように
// するためです。別エンティティの ID が期待される箇所に UserID を渡すとコンパイラが
// 拒否します。
type UserID int64

// String returns the decimal form of the UserID.
//
// [Ja] String は UserID を 10 進表記で返します。
func (id UserID) String() string { return strconv.FormatInt(int64(id), 10) }

// UserSessionID is the typed identifier for a user session. Like UserID it wraps
// int64 so session IDs cannot be mixed up with other entities' IDs.
//
// [Ja] UserSessionID はユーザーセッションの型付き識別子です。UserID と同様に
// int64 をラップし、セッション ID を他エンティティの ID と取り違えられない
// ようにします。
type UserSessionID int64

// String returns the decimal form of the UserSessionID.
//
// [Ja] String は UserSessionID を 10 進表記で返します。
func (id UserSessionID) String() string { return strconv.FormatInt(int64(id), 10) }

// EmailConfirmationID is the typed identifier for an email confirmation. Like
// UserID it wraps int64 so confirmation IDs cannot be mixed up with other
// entities' IDs.
//
// [Ja] EmailConfirmationID はメール確認の型付き識別子です。UserID と同様に
// int64 をラップし、確認 ID を他エンティティの ID と取り違えられないように
// します。
type EmailConfirmationID int64

// String returns the decimal form of the EmailConfirmationID.
//
// [Ja] String は EmailConfirmationID を 10 進表記で返します。
func (id EmailConfirmationID) String() string { return strconv.FormatInt(int64(id), 10) }

// UserPasswordID is the typed identifier for a user's password credential. Like
// UserID it wraps int64 so password IDs cannot be mixed up with other entities'
// IDs.
//
// [Ja] UserPasswordID はユーザーのパスワード資格情報の型付き識別子です。UserID と
// 同様に int64 をラップし、パスワード ID を他エンティティの ID と取り違えられ
// ないようにします。
type UserPasswordID int64

// String returns the decimal form of the UserPasswordID.
//
// [Ja] String は UserPasswordID を 10 進表記で返します。
func (id UserPasswordID) String() string { return strconv.FormatInt(int64(id), 10) }

// PasswordResetTokenID is the typed identifier for a password reset token. Like
// UserID it wraps int64 so reset-token IDs cannot be mixed up with other
// entities' IDs.
//
// [Ja] PasswordResetTokenID はパスワードリセットトークンの型付き識別子です。UserID と
// 同様に int64 をラップし、リセットトークン ID を他エンティティの ID と取り違え
// られないようにします。
type PasswordResetTokenID int64

// String returns the decimal form of the PasswordResetTokenID.
//
// [Ja] String は PasswordResetTokenID を 10 進表記で返します。
func (id PasswordResetTokenID) String() string { return strconv.FormatInt(int64(id), 10) }

// UserTwoFactorAuthID is the typed identifier for a user's two-factor
// authentication setting. Like UserID it wraps int64 so 2FA IDs cannot be mixed
// up with other entities' IDs.
//
// [Ja] UserTwoFactorAuthID はユーザーの 2 段階認証設定の型付き識別子です。UserID と
// 同様に int64 をラップし、2FA ID を他エンティティの ID と取り違えられないように
// します。
type UserTwoFactorAuthID int64

// String returns the decimal form of the UserTwoFactorAuthID.
//
// [Ja] String は UserTwoFactorAuthID を 10 進表記で返します。
func (id UserTwoFactorAuthID) String() string { return strconv.FormatInt(int64(id), 10) }

// CommunityID is the typed identifier for a community. Like UserID it wraps
// int64 so community IDs cannot be mixed up with other entities' IDs, even
// though an instance hosts exactly one community (ADR 0006) and its row is
// always id 1.
//
// [Ja] CommunityID はコミュニティの型付き識別子です。1 インスタンスがちょうど 1 つの
// コミュニティを運営し (ADR 0006)、その行が常に id 1 であっても、UserID と同様に
// int64 をラップし、コミュニティ ID を他エンティティの ID と取り違えられないように
// します。
type CommunityID int64

// String returns the decimal form of the CommunityID.
//
// [Ja] String は CommunityID を 10 進表記で返します。
func (id CommunityID) String() string { return strconv.FormatInt(int64(id), 10) }

// CategoryID is the typed identifier for a category. Like UserID it wraps int64
// so category IDs cannot be mixed up with other entities' IDs.
//
// [Ja] CategoryID はカテゴリーの型付き識別子です。UserID と同様に int64 をラップし、
// カテゴリー ID を他エンティティの ID と取り違えられないようにします。
type CategoryID int64

// String returns the decimal form of the CategoryID.
//
// [Ja] String は CategoryID を 10 進表記で返します。
func (id CategoryID) String() string { return strconv.FormatInt(int64(id), 10) }

// BoardID is the typed identifier for a board. Like UserID it wraps int64 so
// board IDs cannot be mixed up with other entities' IDs.
//
// [Ja] BoardID は掲示板の型付き識別子です。UserID と同様に int64 をラップし、
// 掲示板 ID を他エンティティの ID と取り違えられないようにします。
type BoardID int64

// String returns the decimal form of the BoardID.
//
// [Ja] String は BoardID を 10 進表記で返します。
func (id BoardID) String() string { return strconv.FormatInt(int64(id), 10) }

// ThreadID is the typed identifier for a thread. Like UserID it wraps int64 so
// thread IDs cannot be mixed up with other entities' IDs.
//
// [Ja] ThreadID はスレッドの型付き識別子です。UserID と同様に int64 をラップし、
// スレッド ID を他エンティティの ID と取り違えられないようにします。
type ThreadID int64

// String returns the decimal form of the ThreadID.
//
// [Ja] String は ThreadID を 10 進表記で返します。
func (id ThreadID) String() string { return strconv.FormatInt(int64(id), 10) }

// PostID is the typed identifier for a post. Like UserID it wraps int64 so post
// IDs cannot be mixed up with other entities' IDs.
//
// [Ja] PostID は投稿の型付き識別子です。UserID と同様に int64 をラップし、投稿 ID を
// 他エンティティの ID と取り違えられないようにします。
type PostID int64

// String returns the decimal form of the PostID.
//
// [Ja] String は PostID を 10 進表記で返します。
func (id PostID) String() string { return strconv.FormatInt(int64(id), 10) }

// PostReferenceID is the typed identifier for a post reference. Like UserID it
// wraps int64 so reference IDs cannot be mixed up with other entities' IDs.
//
// [Ja] PostReferenceID はレス参照の型付き識別子です。UserID と同様に int64 をラップし、
// 参照 ID を他エンティティの ID と取り違えられないようにします。
type PostReferenceID int64

// String returns the decimal form of the PostReferenceID.
//
// [Ja] String は PostReferenceID を 10 進表記で返します。
func (id PostReferenceID) String() string { return strconv.FormatInt(int64(id), 10) }
