package model

import "time"

// RoleNameはロールを指す名前で、インスタンス内で一意です。stringではなく独自の型に
// するのは、ある名前が別の名前を期待する場所へ渡らないようにするためです。atnameとロール名
// はどちらも呼び出し側が並べて持ちうる文字列であり、両者の取り違えは、誰も見つからない
// ルックアップではなくここでのコンパイルエラーになります。
type RoleName string

// RoleNameAdminは、インスタンスに同梱される唯一のロールを名指します。マイグレーション
// が作り、ScopeCommunityAdminを持ちます。このロールへはidではなく名前で辿り着きます。
// idはマイグレーションを適用したデータベースごとに採番されるものであり、インスタンスが
// 違えば別の行を名指すためです。
const RoleNameAdmin RoleName = "admin"

// Roleは、コミュニティがそれを持つ人々に与えるスコープの、名前の付いた集合です。
// スコープは割当ごとではなくロール側にあるため、ロールでできることを変えるのは、保持者ごと
// ではなく1回の更新で済みます。
type Role struct {
	ID   RoleID
	Name RoleName

	// Scopesはロールが与えるものであり、データベースではroles.scopesのJSON配列
	// として保持されます。この列は文字列の配列なら何でも受け取るため、語彙の外にある名前も
	// ここに現れえます。そうした名前が何であるかはScopeを参照してください。
	Scopes []Scope

	CreatedAt time.Time
	UpdatedAt time.Time
}
