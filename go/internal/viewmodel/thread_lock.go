package viewmodel

import "github.com/groobb/groobb/go/internal/model"

// ThreadLockReasonは、スレッドがそれ以上の投稿を受け付けなくなる理由を、
// Presentation層が保持する形で表します。ページは理由1つにつき1つの案内を描くため、
// ページが理由に求めるのは、それを他の理由と区別できることです。
//
// エイリアスではなくmodel.ThreadLockReasonを基にした型として定義するのはThreadIDと
// 同じ理由です。変換はハンドラーがページのデータを組み立てる場所に書かれることになり、
// テンプレートがUseCaseからドメインの値をそのまま受け取ることはできなくなります。
type ThreadLockReason model.ThreadLockReason

// ThreadLockReasonPostLimitReachedは、スレッドが持てる投稿をすべて持つことに
// よって到達する理由です (ADR 0009)。
const ThreadLockReasonPostLimitReached = ThreadLockReason(model.ThreadLockReasonPostLimitReached)

// ThreadLockReasonLockedByModeratorは、管理者がスレッドを閉じたことによって
// スレッドが持つ理由です。
const ThreadLockReasonLockedByModerator = ThreadLockReason(model.ThreadLockReasonLockedByModerator)

// ThreadLockは、スレッドがこれ以上の投稿を受け付けるかどうかと、受け付けない理由を
// 併せて持ちます。単一のフラグではなく理由を運ぶのは、理由が互いに並び立つためです。
// ロック中であることだけを述べるページは、どの条件に当たったのかの推測を訪問者に委ねる
// ことになります。そして理由のうちには次のスレッドを立てることで応えられるものと、
// そうでないものがあります。
type ThreadLock struct {
	Reasons []ThreadLockReason
}

// NewThreadLockは、スレッドが持つ理由を、ページがそれを描く形へ変換します。
func NewThreadLock(reasons []model.ThreadLockReason) ThreadLock {
	converted := make([]ThreadLockReason, len(reasons))
	for i, reason := range reasons {
		converted[i] = ThreadLockReason(reason)
	}
	return ThreadLock{Reasons: converted}
}

// Lockedは、スレッドがこれ以上の投稿を受け付けないかどうかを返します。ページが
// そもそも返信フォームを描くかどうかを決めるものがこれです。
func (l ThreadLock) Locked() bool {
	return len(l.Reasons) > 0
}

// LockedByModeratorは、スレッドがこれ以上の投稿を受け付けない理由のうちに管理者の
// 判断があるかどうかを返します。ページがこれに基づいて動くのは、描くものが、その判断が
// 下されていることに懸かっていて、それが唯一成立しているものであることには懸かっていない
// 場合です。案内はその隣に並ぶ条件ではなくその判断を述べ、スレッド自身のページはそれを
// 外す手立てを差し出します。
func (l ThreadLock) LockedByModerator() bool {
	for _, reason := range l.Reasons {
		if reason == ThreadLockReasonLockedByModerator {
			return true
		}
	}
	return false
}

// LockedOnlyは、これ以上の投稿を阻んでいるものがreasonだけであるかどうかを
// 返します。ページが理由の有無ではなくこれに基づいて動くのは、次に差し出すものが、
// 他に何も成立していないことに懸かっているためです。上限だけであれば新しいスレッドを
// 立てることで応えられますが、同じ上限に別の理由が並んでいるときはそうではありません。
func (l ThreadLock) LockedOnly(reason ThreadLockReason) bool {
	return len(l.Reasons) == 1 && l.Reasons[0] == reason
}
