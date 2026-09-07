package post

import (
	"github.com/groobb/groobb/go/internal/templates/components"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// CreatePageData is the data for the page a refused reply comes back on: the
// thread it was written in, why it was not saved, and the reply itself.
//
// The page exists so that a submission the application would not take is not
// lost with it. A reply is written at the end of a thread, and the thread is
// long: sending the visitor back to it with an error would have them find the
// end again and write the reply a second time.
//
// The thread is named and linked rather than only identified, both because a
// visitor has to be able to tell which conversation this was, and because a
// submission whose answer never arrived is checked by opening the thread and
// looking for the post.
//
// [Ja] CreatePageData は、拒否された返信が戻ってくるページのデータです。それが書かれた
// スレッド、保存されなかった理由、そして返信そのものを持ちます。
//
// このページは、アプリケーションが受け付けなかった送信を、それと一緒に失わせないために
// あります。返信はスレッドの末尾で書かれ、スレッドは長いものです。エラーと共にスレッドへ
// 戻すことは、訪問者に末尾をもう一度探させ、返信をもう一度書かせることになります。
//
// スレッドを識別するだけでなく名指してリンクするのは、どの会話であったかを訪問者が
// 判別できる必要があるためであり、答えの届かなかった送信は、スレッドを開いてそこに投稿が
// あるかを見ることで確かめられるためでもあります。
type CreatePageData struct {
	ThreadID    viewmodel.ThreadID
	ThreadTitle string

	// ThreadLanguage is the language the thread is mainly written in, declared on
	// its title where the title is repeated here. The reply below carries no
	// language of its own, since a reply in another language is accepted.
	//
	// [Ja] ThreadLanguage はスレッドが主に書かれている言語で、タイトルがここに繰り返される
	// にあたってそのタイトルに宣言します。下の返信は自身の言語を持ちません。別の言語での
	// 返信も受け付けるためです。
	ThreadLanguage viewmodel.ThreadLanguage

	// Lock holds the reasons the thread takes no further post, and is empty when
	// the submission was refused over something the visitor can still act on.
	// The two lead to different pages: a lock is not waited out or corrected, so
	// what stands here is the reason and the text, without a way to send it
	// again.
	//
	// [Ja] Lock はスレッドがこれ以上の投稿を受け付けない理由を持ち、訪問者がまだ手を
	// 打てる何かによって送信が拒否されたときは空です。両者は別のページになります。
	// ロックは待っても直しても解けないため、ここに立つのは理由と書かれたテキストであり、
	// もう一度送る手立ては伴いません。
	Lock viewmodel.ThreadLock

	// PostLimit is the number of posts a thread can hold, interpolated into the
	// lock notice that names that cap.
	//
	// [Ja] PostLimit はスレッドが持てる投稿の数で、その上限を名指すロックの案内に
	// 差し込みます。
	PostLimit int

	// Reply is the submission as it was written, carried back so it can be
	// corrected and sent again, or copied out of a thread that will not take it.
	//
	// [Ja] Reply は書かれたままの送信で、直して送り直せるように、あるいはそれを受け付けない
	// スレッドから写し取れるように、持ち帰られます。
	Reply components.PostFormData
}

// LockNotice returns what the notice about the locked thread is drawn from. It
// is marked as answering a submission, since this page is only ever reached by
// making one.
//
// [Ja] LockNotice は、ロック中のスレッドについての案内を描くためのデータを返します。
// 送信への答えである旨を立てます。このページには送信を行うことでしか辿り着かないため
// です。
func (d CreatePageData) LockNotice() components.ThreadLockNoticeData {
	return components.ThreadLockNoticeData{Lock: d.Lock, PostLimit: d.PostLimit, Refusal: true}
}

// CreateHeadingID is the id of this page's main heading. The community layout
// points the <main> landmark at it with aria-labelledby, so the region's
// accessible name and the heading a sighted visitor reads are the same text.
//
// [Ja] CreateHeadingID はこのページの主見出しの id です。コミュニティレイアウトが
// aria-labelledby で <main> ランドマークをこれに向けるため、領域のアクセシブルな名前と、
// 目で見る訪問者が読む見出しが同じ文字列になります。
const CreateHeadingID = "post-create-heading"
