package viewmodel

import "github.com/groobb/groobb/go/internal/model"

// ThreadLockReason is a reason a thread takes no further post, as the
// Presentation layer holds it. A page draws one notice per reason, so what it
// needs of a reason is to tell it apart from the others.
//
// It is defined over model.ThreadLockReason rather than as an alias, for the
// reason ThreadID is: the conversion is then written where a handler builds the
// page's data, and a template cannot take the domain's own value straight from
// a UseCase.
//
// [Ja] ThreadLockReason は、スレッドがそれ以上の投稿を受け付けなくなる理由を、
// Presentation 層が保持する形で表します。ページは理由 1 つにつき 1 つの案内を描くため、
// ページが理由に求めるのは、それを他の理由と区別できることです。
//
// エイリアスではなく model.ThreadLockReason を基にした型として定義するのは ThreadID と
// 同じ理由です。変換はハンドラーがページのデータを組み立てる場所に書かれることになり、
// テンプレートが UseCase からドメインの値をそのまま受け取ることはできなくなります。
type ThreadLockReason model.ThreadLockReason

// ThreadLockReasonPostLimitReached is the reason a thread reaches by holding
// every post it can hold (ADR 0009).
//
// [Ja] ThreadLockReasonPostLimitReached は、スレッドが持てる投稿をすべて持つことに
// よって到達する理由です (ADR 0009)。
const ThreadLockReasonPostLimitReached = ThreadLockReason(model.ThreadLockReasonPostLimitReached)

// ThreadLock is whether a thread takes a further post, together with the reasons
// it does not. It carries the reasons rather than a single flag because they
// hold alongside one another: a page saying only that a thread is locked would
// leave the visitor to guess which of the conditions is the one they have met,
// and one of them is answered by starting the next thread while another is not.
//
// [Ja] ThreadLock は、スレッドがこれ以上の投稿を受け付けるかどうかと、受け付けない理由を
// 併せて持ちます。単一のフラグではなく理由を運ぶのは、理由が互いに並び立つためです。
// ロック中であることだけを述べるページは、どの条件に当たったのかの推測を訪問者に委ねる
// ことになります。そして理由のうちには次のスレッドを立てることで応えられるものと、
// そうでないものがあります。
type ThreadLock struct {
	Reasons []ThreadLockReason
}

// NewThreadLock converts the reasons a thread carries into the form a page draws
// them from.
//
// [Ja] NewThreadLock は、スレッドが持つ理由を、ページがそれを描く形へ変換します。
func NewThreadLock(reasons []model.ThreadLockReason) ThreadLock {
	converted := make([]ThreadLockReason, len(reasons))
	for i, reason := range reasons {
		converted[i] = ThreadLockReason(reason)
	}
	return ThreadLock{Reasons: converted}
}

// Locked reports whether the thread takes no further post, which is what decides
// whether a page draws a reply form at all.
//
// [Ja] Locked は、スレッドがこれ以上の投稿を受け付けないかどうかを返します。ページが
// そもそも返信フォームを描くかどうかを決めるものがこれです。
func (l ThreadLock) Locked() bool {
	return len(l.Reasons) > 0
}

// LockedOnly reports whether reason is the only thing standing in the way of a
// further post. A page acts on this rather than on the presence of a reason,
// because what it offers next depends on nothing else holding: the cap alone is
// answered by starting a new thread, while the same cap alongside another reason
// is not.
//
// [Ja] LockedOnly は、これ以上の投稿を阻んでいるものが reason だけであるかどうかを
// 返します。ページが理由の有無ではなくこれに基づいて動くのは、次に差し出すものが、
// 他に何も成立していないことに懸かっているためです。上限だけであれば新しいスレッドを
// 立てることで応えられますが、同じ上限に別の理由が並んでいるときはそうではありません。
func (l ThreadLock) LockedOnly(reason ThreadLockReason) bool {
	return len(l.Reasons) == 1 && l.Reasons[0] == reason
}
