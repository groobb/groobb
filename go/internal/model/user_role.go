package model

import "time"

// UserRole holds one role for one user. A person may hold several roles and a
// role may be held by several people, so what someone is admitted to is the
// scopes of every role they hold taken together.
//
// The assignment carries no scopes of its own: two people holding the same role
// are admitted to the same things, and there is no way to widen one person's
// copy of a role.
//
// [Ja] UserRole は、1 人のユーザーに対する 1 つのロールの割当です。1 人が複数のロールを
// 持て、1 つのロールを複数人が持てるため、ある人が何を許されるかは、その人が持つすべての
// ロールのスコープを合わせたものになります。
//
// 割当は自身のスコープを持ちません。同じロールを持つ 2 人は同じことを許され、ある人の分の
// ロールだけを広げる方法はありません。
type UserRole struct {
	ID     UserRoleID
	UserID UserID
	RoleID RoleID

	CreatedAt time.Time
	UpdatedAt time.Time
}
