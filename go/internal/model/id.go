// Package model holds Groobb's domain entities and the value types they are
// built from, such as the typed entity IDs defined here.
//
// [Ja] model パッケージは Groobb のドメインエンティティと、それを構成する値型
// (ここで定義する型付きエンティティ ID など) を保持します。
package model

import "github.com/google/uuid"

// UserID is the typed identifier for a user.
//
// It wraps uuid.UUID rather than a bare string so that IDs of different
// entities cannot be assigned to one another by mistake: the compiler rejects
// passing a UserID where another entity's ID is expected, and an invalid value
// such as UserID("not-a-uuid") cannot be constructed.
//
// [Ja] UserID はユーザーの型付き識別子です。
//
// 素の string ではなく uuid.UUID をラップするのは、異なるエンティティの ID を
// 取り違えて代入できないようにするためです。別エンティティの ID が期待される箇所に
// UserID を渡すとコンパイラが拒否し、UserID("not-a-uuid") のような不正値も構築でき
// ません。
type UserID uuid.UUID

// String returns the canonical UUID string form of the UserID.
//
// [Ja] String は UserID を正準の UUID 文字列形式で返します。
func (id UserID) String() string { return uuid.UUID(id).String() }

// UserIDsToUUIDs converts a slice of UserID to a slice of uuid.UUID, for passing
// IDs to sqlc-generated queries that take PostgreSQL uuid arrays.
//
// [Ja] UserIDsToUUIDs は UserID スライスを uuid.UUID スライスに変換します。
// PostgreSQL の uuid 配列を受け取る sqlc 生成クエリに ID を渡すために使います。
func UserIDsToUUIDs(ids []UserID) []uuid.UUID {
	us := make([]uuid.UUID, len(ids))
	for i, id := range ids {
		us[i] = uuid.UUID(id)
	}
	return us
}

// UUIDsToUserIDs converts a slice of uuid.UUID to a slice of UserID, for turning
// query results back into typed IDs.
//
// [Ja] UUIDsToUserIDs は uuid.UUID スライスを UserID スライスに変換します。
// クエリ結果を型付き ID に戻すために使います。
func UUIDsToUserIDs(us []uuid.UUID) []UserID {
	ids := make([]UserID, len(us))
	for i, u := range us {
		ids[i] = UserID(u)
	}
	return ids
}

// UserSessionID is the typed identifier for a user session. Like UserID it wraps
// uuid.UUID so session IDs cannot be mixed up with other entities' IDs.
//
// [Ja] UserSessionID はユーザーセッションの型付き識別子です。UserID と同様に
// uuid.UUID をラップし、セッション ID を他エンティティの ID と取り違えられない
// ようにします。
type UserSessionID uuid.UUID

// String returns the canonical UUID string form of the UserSessionID.
//
// [Ja] String は UserSessionID を正準の UUID 文字列形式で返します。
func (id UserSessionID) String() string { return uuid.UUID(id).String() }
