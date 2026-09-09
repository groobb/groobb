package model

import "time"

// RoleName is the name a role is addressed by, unique across the instance. It
// is a type of its own rather than a string so that a name cannot be handed to
// something expecting a different one: an atname and a role name are both
// strings a caller can hold side by side, and swapping them is a compile error
// here instead of a lookup that finds nobody.
//
// [Ja] RoleName はロールを指す名前で、インスタンス内で一意です。string ではなく独自の型に
// するのは、ある名前が別の名前を期待する場所へ渡らないようにするためです。atname とロール名
// はどちらも呼び出し側が並べて持ちうる文字列であり、両者の取り違えは、誰も見つからない
// ルックアップではなくここでのコンパイルエラーになります。
type RoleName string

// RoleNameAdmin names the one role the instance ships with, created by a
// migration and carrying ScopeCommunityAdmin. The role is reached by this name
// rather than by an id, because an id is assigned by whichever database the
// migration ran against and so names a different row from one instance to the
// next.
//
// [Ja] RoleNameAdmin は、インスタンスに同梱される唯一のロールを名指します。マイグレーション
// が作り、ScopeCommunityAdmin を持ちます。このロールへは id ではなく名前で辿り着きます。
// id はマイグレーションを適用したデータベースごとに採番されるものであり、インスタンスが
// 違えば別の行を名指すためです。
const RoleNameAdmin RoleName = "admin"

// Role is a named set of scopes a community grants to the people holding it.
// Scopes sit on the role rather than on each assignment, so changing what a
// role may do is one update instead of one per holder.
//
// [Ja] Role は、コミュニティがそれを持つ人々に与えるスコープの、名前の付いた集合です。
// スコープは割当ごとではなくロール側にあるため、ロールでできることを変えるのは、保持者ごと
// ではなく 1 回の更新で済みます。
type Role struct {
	ID   RoleID
	Name RoleName

	// Scopes is what the role grants, held in the database as the JSON array
	// roles.scopes. That column takes any array of strings, so a name outside
	// the vocabulary can appear here; see Scope for what such a name amounts
	// to.
	//
	// [Ja] Scopes はロールが与えるものであり、データベースでは roles.scopes の JSON 配列
	// として保持されます。この列は文字列の配列なら何でも受け取るため、語彙の外にある名前も
	// ここに現れえます。そうした名前が何であるかは Scope を参照してください。
	Scopes []Scope

	CreatedAt time.Time
	UpdatedAt time.Time
}
