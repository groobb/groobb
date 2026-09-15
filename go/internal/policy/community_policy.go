package policy

import "github.com/groobb/groobb/go/internal/model"

// CommunityPolicyは、ある人が持つスコープが、このコミュニティに対する操作を
// 許すかどうかを答えます。操作者が持つすべてのロールのスコープから構築し、各メソッドは
// スコープではなく操作を1つ名指します。これにより、呼び出し側はこれから何をするかを
// 述べ、それに何が要るかはこのファイルだけが決めます。
type CommunityPolicy struct {
	scopes map[model.Scope]struct{}
}

// NewCommunityPolicyは、渡したスコープを持つ人のポリシーを構築します。その際に
// model.ScopeCommunityAdminを語彙の全体へ展開します。判定ごとではなくここで展開する
// ことが、後続の計画がスコープを足すときに保存済みのロールへ立ち返らずに済む理由です。
// コミュニティの管理者は、model.Scopesがその新しいスコープを並べた時点でそれを持ちます。
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

// CanAccessAdminは、操作者がそもそも管理画面を開いてよいかどうかを返します。
// 管理ハブと、サイドバーからそこへの導線が尋ねるのがこれであるため、すべての画面では
// なくどれか1つの画面を許された人に対して真になります。後続の計画が固有のスコープを
// 持つ画面を足すと、この判定はそのぶん広がります。
func (p *CommunityPolicy) CanAccessAdmin() bool {
	return p.has(model.ScopeUserRead) || p.has(model.ScopeUserRoleWrite) ||
		p.has(model.ScopeModerationLogRead)
}

// CanListUsersは、操作者が管理画面でコミュニティの利用者の一覧を読んでよいか
// どうかを返します。
func (p *CommunityPolicy) CanListUsers() bool {
	return p.has(model.ScopeUserRead)
}

// CanGrantUserRoleは、操作者が誰かにロールを与えてよいかどうかを返します。
func (p *CommunityPolicy) CanGrantUserRole() bool {
	return p.has(model.ScopeUserRoleWrite)
}

// CanRevokeUserRoleは、操作者が誰かからロールを取り上げてよいかどうかを返します。
// コミュニティが管理者のいない状態になってよいかどうかは別の問いで、操作者のスコープでは
// なく、書き込むトランザクションの中で行に対して判断します。
func (p *CommunityPolicy) CanRevokeUserRole() bool {
	return p.has(model.ScopeUserRoleWrite)
}

// CanLockThreadは、操作者がスレッドへの新しい返信を締め切ってよいかどうかを
// 返します。
func (p *CommunityPolicy) CanLockThread() bool {
	return p.has(model.ScopeThreadLockWrite)
}

// CanUnlockThreadは、操作者が管理者の締め切ったスレッドを開き直してよいかどうかを
// 返します。そのスレッドが実際に返信を受け付けるかどうかは別の問いです。投稿数の上限に
// 達したスレッドは、管理者のロックが外れた後もその理由でロックされたままです。
func (p *CommunityPolicy) CanUnlockThread() bool {
	return p.has(model.ScopeThreadLockWrite)
}

// CanUnpublishThreadは、操作者がスレッドをコミュニティから見えなくしてよいか
// どうかを返します。
func (p *CommunityPolicy) CanUnpublishThread() bool {
	return p.has(model.ScopeThreadUnpublicationWrite)
}

// CanUnpublishPostは、操作者が投稿を1つコミュニティから見えなくしてよいか
// どうかを返します。
func (p *CommunityPolicy) CanUnpublishPost() bool {
	return p.has(model.ScopePostUnpublicationWrite)
}

// CanModerateThreadは、操作者がスレッドに対して働きかける画面を開いてよいかどうかを
// 返します。/t/{id} の下の確認ページが、対象のスレッドを読む前に尋ねるのがこれであるため、
// CanAccessAdminが管理画面に対してそうであるように、すべてではなくどれか1つの操作を許された
// 人に対して真になります。そのページが続いてどの操作を行うかはその操作の判定が決めるため、
// 1つを許された訪問者が、画面に辿り着いたことによって残りを許されることはありません。
func (p *CommunityPolicy) CanModerateThread() bool {
	return p.CanLockThread() || p.CanUnpublishThread() || p.CanUnpublishPost()
}

// CanSuspendUserは、操作者がコミュニティにおけるアカウントの活動を止めてよいか
// どうかを返します。そのアカウントを止めてよいかどうか (操作者自身のものである場合や、
// 最後の管理者である場合) は別の問いで、操作者のスコープではなく、書き込むトランザク
// ションの中で行に対して判断します。
func (p *CommunityPolicy) CanSuspendUser() bool {
	return p.has(model.ScopeUserSuspensionWrite)
}

// CanUnsuspendUserは、操作者が止められたアカウントをコミュニティで再び活動
// できるようにしてよいかどうかを返します。
func (p *CommunityPolicy) CanUnsuspendUser() bool {
	return p.has(model.ScopeUserSuspensionWrite)
}

// CanListModerationLogsは、操作者が管理画面でモデレーション操作の記録を読んで
// よいかどうかを返します。
func (p *CommunityPolicy) CanListModerationLogs() bool {
	return p.has(model.ScopeModerationLogRead)
}

// hasは、展開後の集合が指定したスコープを持つかどうかを返します。
func (p *CommunityPolicy) has(scope model.Scope) bool {
	_, ok := p.scopes[scope]
	return ok
}
