package post

import (
	"github.com/groobb/groobb/go/internal/templates/components"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// CreatePageDataは、拒否された返信が戻ってくるページのデータです。それが書かれた
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

	// ThreadLanguageはスレッドが主に書かれている言語で、タイトルがここに繰り返される
	// にあたってそのタイトルに宣言します。下の返信は自身の言語を持ちません。別の言語での
	// 返信も受け付けるためです。
	ThreadLanguage viewmodel.ThreadLanguage

	// Lockはスレッドがこれ以上の投稿を受け付けない理由を持ち、訪問者がまだ手を
	// 打てる何かによって送信が拒否されたときは空です。両者は別のページになります。
	// ロックは待っても直しても解けないため、ここに立つのは理由と書かれたテキストであり、
	// もう一度送る手立ては伴いません。
	Lock viewmodel.ThreadLock

	// PostLimitはスレッドが持てる投稿の数で、その上限を名指すロックの案内に
	// 差し込みます。
	PostLimit int

	// Replyは書かれたままの送信で、直して送り直せるように、あるいはそれを受け付けない
	// スレッドから写し取れるように、持ち帰られます。
	Reply components.PostFormData
}

// LockNoticeは、ロック中のスレッドについての案内を描くためのデータを返します。
// 送信への答えである旨を立てます。このページには送信を行うことでしか辿り着かないため
// です。
func (d CreatePageData) LockNotice() components.ThreadLockNoticeData {
	return components.ThreadLockNoticeData{Lock: d.Lock, PostLimit: d.PostLimit, Refusal: true}
}

// CreateHeadingIDはこのページの主見出しのidです。コミュニティレイアウトが
// aria-labelledbyで <main> ランドマークをこれに向けるため、領域のアクセシブルな名前と、
// 目で見る訪問者が読む見出しが同じ文字列になります。
const CreateHeadingID = "post-create-heading"
