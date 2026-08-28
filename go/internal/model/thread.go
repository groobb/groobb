package model

import "time"

// ThreadPostLimit is the number of posts a thread can hold (ADR 0009). The cap
// is what lets the post list go unpaginated: a reply number stays a permanent
// address only while the whole thread answers under one URL. A thread that has
// reached it takes no further post and says so.
//
// [Ja] ThreadPostLimit はスレッドが持てる投稿の数です (ADR 0009)。投稿一覧をページ
// 分割せずに済ませているのはこの上限があるためで、レス番号が永久アドレスであり続けるのは、
// スレッド全体が 1 つの URL で応答する限りです。上限に達したスレッドはそれ以上の投稿を
// 受け付けず、その旨を表示します。
const ThreadPostLimit = 1000

// Thread is a conversation inside a board: a container for posts that holds no
// body of its own. It is addressed by id rather than a slug because its title
// can be edited, and an address derived from the title would break the links
// already shared.
//
// [Ja] Thread は掲示板の中の 1 つの会話で、投稿の入れ物であり自身は本文を持ちません。
// slug ではなく id で指すのは、タイトルが編集されうるためで、タイトルから導いた
// アドレスでは既に共有されたリンクが壊れます。
type Thread struct {
	ID ThreadID

	// BoardID is the board this thread was posted in. A thread can be moved to
	// another board, which is why /t/{id} names a thread without naming its
	// board.
	//
	// [Ja] BoardID はこのスレッドが立った掲示板です。スレッドは別の掲示板へ移されうる
	// ため、/t/{id} はスレッドをその掲示板を言わずに名指しします。
	BoardID BoardID

	// UserID is the account that started the thread. It remains set while the
	// account is logically withdrawn and becomes nil only after the account row
	// is physically deleted. Resolve the referenced user to distinguish an active
	// author from a logically withdrawn one. The conversation stays either way,
	// so everyone else's replies remain intact.
	//
	// [Ja] UserID はスレッドを立てたアカウントです。アカウントが論理退会している間も値を
	// 保ち、その行が物理削除された後にだけ nil になります。有効な作者と論理退会済みの作者は、
	// 参照先のユーザーを解決して区別します。いずれの場合も会話は残るため、他の全員の返信は
	// 維持されます。
	UserID *UserID

	Title string

	// PostsCount, LastPostID and LastPostedAt are denormalized from the thread's
	// posts. A row of a board's thread list needs all three, and deriving them
	// per row would mean one aggregate per thread on every page view. They are
	// written in the transaction that writes the post, so holding them costs no
	// extra write transaction.
	//
	// LastPostedAt is never absent because a thread is created together with its
	// first post; LastPostID is nil only once that post has been deleted.
	//
	// [Ja] PostsCount・LastPostID・LastPostedAt はスレッドの投稿からの非正規化です。
	// 掲示板のスレッド一覧は 1 行を描くのにこの 3 つをいずれも必要とし、行ごとに導くと
	// ページを開くたびにスレッド 1 件につき 1 回の集計が走ります。これらは投稿を書き込む
	// トランザクションの中で書かれるため、保持しても書き込みトランザクションは増えません。
	//
	// LastPostedAt が欠けることが無いのは、スレッドが最初の投稿と同時に作られるためです。
	// LastPostID が nil になるのは、その投稿が削除された場合だけです。
	PostsCount   int
	LastPostID   *PostID
	LastPostedAt time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
}
