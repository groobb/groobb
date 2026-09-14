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
	return p.has(model.ScopeUserRead) || p.has(model.ScopeUserRoleWrite) ||
		p.has(model.ScopeModerationLogRead)
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

// CanLockThread reports whether the actor may close a thread to new replies.
//
// [Ja] CanLockThread は、操作者がスレッドへの新しい返信を締め切ってよいかどうかを
// 返します。
func (p *CommunityPolicy) CanLockThread() bool {
	return p.has(model.ScopeThreadLockWrite)
}

// CanUnlockThread reports whether the actor may reopen a thread the moderators
// closed. Whether the thread then takes replies is a separate question: a thread
// that reached the cap on its posts stays locked for that reason after the
// moderators' lock is lifted.
//
// [Ja] CanUnlockThread は、操作者が管理者の締め切ったスレッドを開き直してよいかどうかを
// 返します。そのスレッドが実際に返信を受け付けるかどうかは別の問いです。投稿数の上限に
// 達したスレッドは、管理者のロックが外れた後もその理由でロックされたままです。
func (p *CommunityPolicy) CanUnlockThread() bool {
	return p.has(model.ScopeThreadLockWrite)
}

// CanUnpublishThread reports whether the actor may hide a thread from the
// community.
//
// [Ja] CanUnpublishThread は、操作者がスレッドをコミュニティから見えなくしてよいか
// どうかを返します。
func (p *CommunityPolicy) CanUnpublishThread() bool {
	return p.has(model.ScopeThreadUnpublicationWrite)
}

// CanUnpublishPost reports whether the actor may hide one post from the
// community.
//
// [Ja] CanUnpublishPost は、操作者が投稿を 1 つコミュニティから見えなくしてよいか
// どうかを返します。
func (p *CommunityPolicy) CanUnpublishPost() bool {
	return p.has(model.ScopePostUnpublicationWrite)
}

// CanModerateThread reports whether the actor may open the screens that act on
// a thread. It is what a confirmation page under /t/{id} asks before it reads
// the thread it is about, so it holds for anyone admitted to any one of those
// operations rather than to all of them, the way CanAccessAdmin holds for the
// admin screens. Which operation a page goes on to carry out is settled by the
// judgment for that operation, so a visitor admitted to one of them is not
// admitted to the rest by having reached the screen.
//
// [Ja] CanModerateThreadは、操作者がスレッドに対して働きかける画面を開いてよいかどうかを
// 返します。/t/{id} の下の確認ページが、対象のスレッドを読む前に尋ねるのがこれであるため、
// CanAccessAdminが管理画面に対してそうであるように、すべてではなくどれか1つの操作を許された
// 人に対して真になります。そのページが続いてどの操作を行うかはその操作の判定が決めるため、
// 1つを許された訪問者が、画面に辿り着いたことによって残りを許されることはありません。
func (p *CommunityPolicy) CanModerateThread() bool {
	return p.CanLockThread() || p.CanUnpublishThread() || p.CanUnpublishPost()
}

// CanSuspendUser reports whether the actor may stop what an account does in the
// community. Whether this particular account may be stopped -- the actor's own,
// or the last administrator's -- is a separate question, decided against the
// rows inside the transaction that writes rather than against the actor's
// scopes.
//
// [Ja] CanSuspendUser は、操作者がコミュニティにおけるアカウントの活動を止めてよいか
// どうかを返します。そのアカウントを止めてよいかどうか (操作者自身のものである場合や、
// 最後の管理者である場合) は別の問いで、操作者のスコープではなく、書き込むトランザク
// ションの中で行に対して判断します。
func (p *CommunityPolicy) CanSuspendUser() bool {
	return p.has(model.ScopeUserSuspensionWrite)
}

// CanUnsuspendUser reports whether the actor may let a stopped account act in
// the community again.
//
// [Ja] CanUnsuspendUser は、操作者が止められたアカウントをコミュニティで再び活動
// できるようにしてよいかどうかを返します。
func (p *CommunityPolicy) CanUnsuspendUser() bool {
	return p.has(model.ScopeUserSuspensionWrite)
}

// CanListModerationLogs reports whether the actor may read the record of
// moderation operations in the admin screens.
//
// [Ja] CanListModerationLogs は、操作者が管理画面でモデレーション操作の記録を読んで
// よいかどうかを返します。
func (p *CommunityPolicy) CanListModerationLogs() bool {
	return p.has(model.ScopeModerationLogRead)
}

// has reports whether the expanded set holds the given scope.
//
// [Ja] has は、展開後の集合が指定したスコープを持つかどうかを返します。
func (p *CommunityPolicy) has(scope model.Scope) bool {
	_, ok := p.scopes[scope]
	return ok
}
