// Package post_unpublication holds the page a post's unpublication is confirmed
// on (GET /t/{id}/posts/{number}/unpublication/new), the one screen a single
// post is taken out of view from.
//
// [Ja] post_unpublicationパッケージは、投稿の非公開を確認するページ
// (GET /t/{id}/posts/{number}/unpublication/new) を持ちます。投稿1件がそこから視界の外へ
// 移される唯一の画面です。
package post_unpublication

import (
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// NewPageData is what the unpublication confirmation page is drawn from: the
// post the operation is about, the note that travels with it, and the token the
// submission carries.
//
// The post is named the way the thread names it — by its reply number, with the
// author and the body under it — because that is what the administrator is
// deciding about. A page that named only the number would ask for a judgment
// about a post whose text it did not show.
//
// The body is carried as the text it was written as rather than as the
// viewmodel.PostBody the thread renders. Here it is evidence rather than
// conversation: a >>N in it points at a post of the thread, which is not the
// document this page is, and an address in it leads out of the community from a
// screen that exists to take something out of view.
//
// [Ja] NewPageDataは、非公開の確認ページが描かれる元です。操作の対象となる投稿、それとともに
// 運ばれる注記、そして送信が運ぶトークンを持ちます。
//
// 投稿をスレッドが名指すとおりに、すなわちレス番号と、その下の作者と本文で名指すのは、それが
// 管理者の判断の対象そのものであるためです。番号だけを名指すページは、本文を示さないまま、その
// 投稿についての判断を求めることになります。
//
// 本文は、スレッドが描画するviewmodel.PostBodyではなく、書かれたままのテキストとして運びます。
// ここでの本文は会話ではなく判断の材料です。本文の中の>>Nはスレッドの投稿を指しますが、この
// ページはその文書ではありません。本文の中のアドレスは、何かを視界から外すために在る画面から
// コミュニティの外へ連れ出します。
type NewPageData struct {
	ThreadID viewmodel.ThreadID

	// Number is the reply number that addresses the post inside its thread
	// (ADR 0009). It names the post on this page and builds the address the form
	// submits to, so the screen acts on the post it shows.
	//
	// [Ja] Numberは、スレッドの中で投稿を指すレス番号です (ADR 0009)。このページで投稿を
	// 名指すのも、フォームの送信先のアドレスを組み立てるのもこれであるため、画面は自身が
	// 示している投稿に対して働きかけます。
	Number int

	// Author is the atname of the account that wrote the post, without the
	// leading @, and "" when the author has withdrawn. The post is shown either
	// way, as it is on the thread: a withdrawal takes the name off what was
	// written, not the writing.
	//
	// [Ja] Authorは投稿を書いたアカウントのatnameで、先頭の@を含みません。作者が退会して
	// いるときは""です。スレッドでそうであるのと同じく、どちらの場合も投稿は示します。退会が
	// 外すのは書かれたものから名前であって、書かれたものではありません。
	Author string

	Body       string
	Reason     string
	CSRFToken  string
	FormErrors *model.ValidationError
}
