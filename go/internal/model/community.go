package model

import "time"

// Community is the container that boards and their posts belong to (ADR 0002).
// Name is the human-facing display name, and Identifier is the URL identifier
// that addresses the community in the short path /c/{identifier}.
//
// Identifier is a plain string rather than a dedicated value type, following how
// users.atname is modeled: both are user-chosen handles whose format is enforced
// by a validator at the entry point, not by the type system. Its uniqueness is
// case-insensitive because the column is citext.
//
// [Ja] Community は掲示板とその投稿が属するコンテナ (ADR 0002)。Name は対人向けの表示名、
// Identifier は短縮パス /c/{identifier} でコミュニティを指す URL 識別子です。
//
// Identifier は専用の値型ではなく素の string とし、users.atname のモデル化に倣います。
// どちらもユーザーが選ぶハンドルであり、その形式は型システムではなく入口のバリデーターが
// 担保します。カラムが citext のため、一意性は大文字小文字を区別しません。
type Community struct {
	ID         CommunityID
	Name       string
	Identifier string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
