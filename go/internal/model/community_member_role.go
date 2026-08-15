package model

import "time"

// CommunityMemberRole assigns a role to a member of a community. A member may
// hold several roles and a role may be held by several members, so assignments
// are their own entity rather than a field on either side.
//
// CommunityID is redundant with the member and the role, which each belong to a
// community already, and is kept because the database uses it to enforce that
// both sides belong to the same community.
//
// [Ja] CommunityMemberRole はコミュニティのメンバーへロールを割り当てます。1 人の
// メンバーが複数のロールを持てて、1 つのロールを複数のメンバーが持てるため、割当は
// どちらか一方のフィールドではなく独立したエンティティとします。
//
// CommunityID はメンバーとロールがそれぞれ既にコミュニティに属している以上は冗長ですが、
// 双方が同じコミュニティに属することを DB が強制するために使われるため保持します。
type CommunityMemberRole struct {
	ID                CommunityMemberRoleID
	CommunityID       CommunityID
	CommunityMemberID CommunityMemberID
	CommunityRoleID   CommunityRoleID
	CreatedAt         time.Time
	UpdatedAt         time.Time
}
