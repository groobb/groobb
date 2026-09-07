package model

import "time"

// PostInterval is the time one person waits between posts. It is measured from
// their last saved post wherever in the instance it was written, because the
// interval belongs to the person: writing in another board, thread or session
// does not start a fresh one.
//
// What it spaces out is successful posts. A refused submission saves nothing,
// so it neither starts an interval nor is stopped by one; how many requests an
// instance answers is a separate question from how fast one account can fill a
// thread.
//
// [Ja] PostInterval は 1 人が投稿と投稿の間に置く時間です。インスタンスのどこに書かれた
// ものであれ、その人の最後に保存された投稿から測ります。間隔はその人に属するもので、
// 別の掲示板・スレッド・セッションへ書いても新しく始まることはないためです。
//
// これが間隔を空けるのは成功した投稿です。拒否された送信は何も保存しないため、間隔を
// 始めることも、間隔に止められることもありません。インスタンスがいくつの要求に応答するかは、
// 1 つのアカウントがどれだけ速くスレッドを埋められるかとは別の問いです。
const PostInterval = 10 * time.Second

// PostIntervalWait returns how much of PostInterval is still to run at now for
// someone whose last post was saved at lastPostedAt, and 0 once it has run out.
// The interval is over at exactly PostInterval, so the post that arrives then is
// taken.
//
// The remainder is rounded up to a whole second because it is what the person is
// told to wait and what Retry-After carries, and that header admits only whole
// seconds. Rounded down, a wait of 1.2s would be given as 1s and would invite a
// retry that is refused again.
//
// [Ja] PostIntervalWait は、最後の投稿が lastPostedAt に保存された人について、now の
// 時点で PostInterval のうちどれだけが残っているかを返し、尽きていれば 0 を返します。
// 間隔はちょうど PostInterval で終わるため、その時点で届いた投稿は受け付けられます。
//
// 残りを 1 秒単位に切り上げるのは、それが利用者に伝える待ち時間であり Retry-After が運ぶ
// 値でもあるためで、このヘッダーは整数秒しか受け付けません。切り捨てると 1.2 秒の待ちが
// 1 秒として伝わり、再び拒否される再送を招きます。
func PostIntervalWait(lastPostedAt, now time.Time) time.Duration {
	remaining := PostInterval - now.Sub(lastPostedAt)
	if remaining <= 0 {
		return 0
	}

	return (remaining + time.Second - 1).Truncate(time.Second)
}

// Post is what a person wrote in a thread.
//
// [Ja] Post は人がスレッドに書いたものです。
type Post struct {
	ID PostID

	ThreadID ThreadID

	// UserID is the account that wrote the post. It remains set while the account
	// is logically withdrawn and becomes nil only after the account row is
	// physically deleted. Resolve the referenced user to distinguish an active
	// author from a logically withdrawn one. The post stays either way, so the
	// replies that quote it keep their context.
	//
	// [Ja] UserID は投稿を書いたアカウントです。アカウントが論理退会している間も値を保ち、
	// その行が物理削除された後にだけ nil になります。有効な作者と論理退会済みの作者は、
	// 参照先のユーザーを解決して区別します。いずれの場合も投稿は残るため、それを引用した
	// 返信は文脈を保てます。
	UserID *UserID

	// Number is the reply number within the thread and is the post's permanent
	// address: a >>N written in another body, the #p{number} anchor, and a URL
	// shared elsewhere all resolve through it.
	//
	// [Ja] Number はスレッド内のレス番号であり、投稿の永久アドレスです。他の本文に
	// 書かれた >>N、アンカーの #p{number}、外部で共有された URL が、いずれもこれで
	// 解決します。
	Number int

	// Body is the text exactly as it was entered, with no markup applied.
	// Linking >>N and URLs happens on the way out, so notation can be added later
	// by changing the rendering alone.
	//
	// [Ja] Body は入力されたテキストそのままで、記法は適用されていません。>>N と URL の
	// リンク化は取り出す側で行うため、記法は描画側の変更だけで後から足せます。
	Body string

	CreatedAt time.Time
	UpdatedAt time.Time
}
