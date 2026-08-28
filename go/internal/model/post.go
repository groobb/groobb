package model

import "time"

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
