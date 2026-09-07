package components

import (
	"github.com/groobb/groobb/go/internal/templates"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// ThreadLockNoticeData is what the notice about a locked thread is drawn from:
// the reasons the thread takes no further post, and the cap one of them names.
//
// [Ja] ThreadLockNoticeData は、ロック中のスレッドについての案内を描くためのデータです。
// スレッドがこれ以上の投稿を受け付けない理由と、そのうちの 1 つが名指す上限を持ちます。
type ThreadLockNoticeData struct {
	Lock viewmodel.ThreadLock

	// PostLimit is the number of posts a thread can hold, interpolated into the
	// sentence about that cap so the number a visitor reads is the one the
	// application enforces.
	//
	// [Ja] PostLimit はスレッドが持てる投稿の数で、その上限を述べる文に差し込みます。
	// 訪問者が読む数値をアプリケーションが適用する数値と同じにするためです。
	PostLimit int

	// BoardThreadsNewPath is where a thread is started in the board this one was
	// posted in, which is where the conversation carries on once this thread can
	// hold no more of it. It is left empty by a caller that has no board to name,
	// and NextThreadPath decides whether the notice offers it at all.
	//
	// [Ja] BoardThreadsNewPath は、このスレッドが立った掲示板でスレッドを立てる場所で
	// あり、このスレッドが会話をこれ以上持てなくなった後、会話が続く場所です。名指す
	// 掲示板を持たない呼び出し側はこれを空のままにし、案内がそれを差し出すかどうかは
	// NextThreadPath が決めます。
	BoardThreadsNewPath templates.Path

	// Refusal says the notice is explaining a submission that was just refused,
	// rather than describing a thread being read. The two are the same fact told
	// at different moments: one is part of the page a visitor opened to read,
	// and the other is the answer to something they did, which is why only the
	// latter is announced as an alert.
	//
	// [Ja] Refusal は、この案内が、読まれているスレッドの状態を述べるものではなく、
	// たった今拒否された送信について説明するものであることを表します。2 つは同じ事実を
	// 別の場面で伝えるものです。一方は訪問者が読むために開いたページの一部であり、
	// もう一方は訪問者が行ったことへの答えです。アラートとして読み上げるのが後者だけ
	// なのはそのためです。
	Refusal bool
}

// NextThreadPath returns where the notice sends a visitor who still has
// something to write, and "" when it sends them nowhere.
//
// The path is offered only while the cap is the only thing holding, because
// that is the one reason a new thread answers: a thread stopped for another
// reason is stopped by a decision, and starting the next one would be walking
// around it. It is the caller that names the destination, since where the
// conversation carries on is a property of the board rather than of the lock.
//
// [Ja] NextThreadPath は、まだ書くことのある訪問者をこの案内がどこへ送るかを返し、
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
