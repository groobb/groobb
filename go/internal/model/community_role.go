package model

import "time"

// CommunityRole is a named role that a community defines for its members. A
// community creates one when it is founded (the administrator role given to its
// creator) and, once role management exists, its administrators can define more.
//
// A role carries only a name: permission scopes are not modeled yet because
// Groobb has no authorization gate that would consume them. Name is unique
// within a community, so two communities may each have a role of the same name.
//
// [Ja] CommunityRole はコミュニティがメンバー向けに定義する名前付きロールです。
// コミュニティは作成時に 1 つ (作成者へ与える管理者ロール) を作り、ロール管理機能が
// 実装されれば管理者がさらに定義できるようになります。
//
// ロールは名前だけを持ちます。権限スコープをまだモデル化しないのは、Groobb にそれを
// 消費する認可ゲートが無いためです。Name はコミュニティ内で一意なので、異なる
// コミュニティが同じ名前のロールを持つことはできます。
type CommunityRole struct {
	ID          CommunityRoleID
	CommunityID CommunityID
	Name        string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
