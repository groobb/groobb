package components

import (
	"github.com/groobb/groobb/go/internal/templates"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// ThreadLockNoticeDataは、ロック中のスレッドについての案内を描くためのデータです。
// スレッドがこれ以上の投稿を受け付けない理由と、そのうちの1つが名指す上限を持ちます。
type ThreadLockNoticeData struct {
	Lock viewmodel.ThreadLock

	// PostLimitはスレッドが持てる投稿の数で、その上限を述べる文に差し込みます。
	// 訪問者が読む数値をアプリケーションが適用する数値と同じにするためです。
	PostLimit int

	// BoardThreadsNewPathは、このスレッドが立った掲示板でスレッドを立てる場所で
	// あり、このスレッドが会話をこれ以上持てなくなった後、会話が続く場所です。名指す
	// 掲示板を持たない呼び出し側はこれを空のままにし、案内がそれを差し出すかどうかは
	// NextThreadPathが決めます。
	BoardThreadsNewPath templates.Path

	// Refusalは、この案内が、読まれているスレッドの状態を述べるものではなく、
	// たった今拒否された送信について説明するものであることを表します。2つは同じ事実を
	// 別の場面で伝えるものです。一方は訪問者が読むために開いたページの一部であり、
	// もう一方は訪問者が行ったことへの答えです。アラートとして読み上げるのが後者だけ
	// なのはそのためです。
	Refusal bool
}

// StatedReasonsは、案内が文を書く理由を返します。
//
// 管理者のロックは、他の理由を隣に並べず、それだけを述べます。これはこのスレッドについての
// 判断であり、それを持つスレッドは他に何が成り立っていても閉じています。併せて到達していた
// 上限は、実のところ1つのことを告げられている訪問者にとって、2つ目の独立した答えとして
// 読まれてしまいます。それ以外の理由はいずれも述べます。それらは互いに並び立ち、どれもが
// 他の理由の述べないことを述べるためです。
func (d ThreadLockNoticeData) StatedReasons() []viewmodel.ThreadLockReason {
	if d.Lock.LockedByModerator() {
		return []viewmodel.ThreadLockReason{viewmodel.ThreadLockReasonLockedByModerator}
	}

	return d.Lock.Reasons
}

// NextThreadPathは、まだ書くことのある訪問者をこの案内がどこへ送るかを返し、
// どこへも送らないときは "" を返します。
//
// パスを差し出すのは上限だけが成立している間に限ります。新しいスレッドが応えられる理由は
// それだけだからです。別の理由で止まっているスレッドは判断によって止められており、次の
// スレッドを立てることはそれを迂回することになります。行き先を名指すのは呼び出し側です。
// 会話がどこで続くかは、ロックではなく掲示板の性質であるためです。
func (d ThreadLockNoticeData) NextThreadPath() templates.Path {
	if !d.Lock.LockedOnly(viewmodel.ThreadLockReasonPostLimitReached) {
		return ""
	}

	return d.BoardThreadsNewPath
}
