package model

import "time"

// UserRoleは、1人のユーザーに対する1つのロールの割当です。1人が複数のロールを
// 持て、1つのロールを複数人が持てるため、ある人が何を許されるかは、その人が持つすべての
// ロールのスコープを合わせたものになります。
//
// 割当は自身のスコープを持ちません。同じロールを持つ2人は同じことを許され、ある人の分の
// ロールだけを広げる方法はありません。
type UserRole struct {
	ID     UserRoleID
	UserID UserID
	RoleID RoleID

	CreatedAt time.Time
	UpdatedAt time.Time
}
