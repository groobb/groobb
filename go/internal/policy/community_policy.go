package policy

import "github.com/groobb/groobb/go/internal/model"

// CommunityPolicy answers whether the scopes someone holds admit an operation on
// this community. It is built from the scopes of every role the actor holds, and
// each of its methods names one operation rather than one scope, so that a call
// site says what it is about to do while this file alone decides what that takes.
//
// [Ja] CommunityPolicy は、ある人が持つスコープが、このコミュニティに対する操作を
// 許すかどうかを答えます。操作者が持つすべてのロールのスコープから構築し、各メソッドは
// スコープではなく操作を 1 つ名指します。これにより、呼び出し側はこれから何をするかを
// 述べ、それに何が要るかはこのファイルだけが決めます。
type CommunityPolicy struct {
	scopes map[model.Scope]struct{}
}

// NewCommunityPolicy builds the policy for someone holding the given scopes,
// expanding model.ScopeCommunityAdmin into the whole vocabulary as it does.
// Expanding here rather than in each check is what lets a later plan add a scope
// without revisiting the stored roles: the community's administrator holds the
// new one from the moment model.Scopes lists it.
//
// The scopes arrive as they were stored, so a name the vocabulary does not
// define may be among them. Such a name is admitted to nothing, because every
// check below asks for a name this application defines.
//
// [Ja] NewCommunityPolicy は、渡したスコープを持つ人のポリシーを構築します。その際に
// model.ScopeCommunityAdmin を語彙の全体へ展開します。判定ごとではなくここで展開する
// ことが、後続の計画がスコープを足すときに保存済みのロールへ立ち返らずに済む理由です。
// コミュニティの管理者は、model.Scopes がその新しいスコープを並べた時点でそれを持ちます。
//
// スコープは保存されていたまま届くため、語彙が定義していない名前が混じりえます。以下の
// 判定はどれもこのアプリケーションが定義する名前を尋ねるため、そうした名前は何も許しません。
func NewCommunityPolicy(scopes []model.Scope) *CommunityPolicy {
	held := make(map[model.Scope]struct{}, len(scopes))
	for _, scope := range scopes {
		if scope == model.ScopeCommunityAdmin {
			for _, expanded := range model.Scopes() {
				held[expanded] = struct{}{}
			}
			continue
		}
		held[scope] = struct{}{}
	}

	return &CommunityPolicy{scopes: held}
}

// CanAccessAdmin reports whether the actor may open the admin screens at all. It
// is what the admin hub and the sidebar's link to it ask, so it holds for anyone
// admitted to any one of the screens rather than to all of them, and it widens as
// a later plan adds a screen with a scope of its own.
//
// [Ja] CanAccessAdmin は、操作者がそもそも管理画面を開いてよいかどうかを返します。
// 管理ハブと、サイドバーからそこへの導線が尋ねるのがこれであるため、すべての画面では
// なくどれか 1 つの画面を許された人に対して真になります。後続の計画が固有のスコープを
// 持つ画面を足すと、この判定はそのぶん広がります。
func (p *CommunityPolicy) CanAccessAdmin() bool {
	return p.has(model.ScopeUserRead) || p.has(model.ScopeUserRoleWrite)
}

// CanListUsers reports whether the actor may read the list of the community's
// people in the admin screens.
//
// [Ja] CanListUsers は、操作者が管理画面でコミュニティの利用者の一覧を読んでよいか
// どうかを返します。
func (p *CommunityPolicy) CanListUsers() bool {
	return p.has(model.ScopeUserRead)
}

// CanGrantUserRole reports whether the actor may give someone a role.
//
// [Ja] CanGrantUserRole は、操作者が誰かにロールを与えてよいかどうかを返します。
func (p *CommunityPolicy) CanGrantUserRole() bool {
	return p.has(model.ScopeUserRoleWrite)
}

// CanRevokeUserRole reports whether the actor may take a role away from someone.
// Whether the community may be left without an administrator is a separate
// question, decided against the rows inside the transaction that writes rather
// than against the actor's scopes.
//
// [Ja] CanRevokeUserRole は、操作者が誰かからロールを取り上げてよいかどうかを返します。
// コミュニティが管理者のいない状態になってよいかどうかは別の問いで、操作者のスコープでは
// なく、書き込むトランザクションの中で行に対して判断します。
func (p *CommunityPolicy) CanRevokeUserRole() bool {
	return p.has(model.ScopeUserRoleWrite)
}

// has reports whether the expanded set holds the given scope.
//
// [Ja] has は、展開後の集合が指定したスコープを持つかどうかを返します。
func (p *CommunityPolicy) has(scope model.Scope) bool {
	_, ok := p.scopes[scope]
	return ok
}
