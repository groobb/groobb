package model

import "time"

// CommunityMember is a user's membership in a community. It is created for the
// community's creator when the community is founded and, once joining exists,
// for everyone else who joins.
//
// A membership carries no handle of its own (ADR 0003): the globally unique
// atname lives on the user, so a membership records only the belonging itself.
// The roles the member holds in that community are modeled separately as
// CommunityMemberRole assignments.
//
// [Ja] CommunityMember はユーザーのコミュニティへの所属です。コミュニティ作成時に
// 作成者の分が作られ、参加機能ができれば参加した他のユーザーの分も作られます。
//
// メンバーシップは自身のハンドルを持ちません (ADR 0003)。グローバルに一意な atname は
// ユーザーが持つため、メンバーシップは所属そのものだけを記録します。そのコミュニティで
// メンバーが持つロールは CommunityMemberRole の割当として別にモデル化します。
type CommunityMember struct {
	ID          CommunityMemberID
	CommunityID CommunityID
	UserID      UserID
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
