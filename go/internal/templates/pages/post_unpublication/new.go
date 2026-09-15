// post_unpublicationパッケージは、投稿の非公開を確認するページ
// (GET /t/{id}/posts/{number}/unpublication/new) を持ちます。投稿1件がそこから視界の外へ
// 移される唯一の画面です。
package post_unpublication

import (
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// NewPageDataは、非公開の確認ページが描かれる元です。操作の対象となる投稿、それとともに
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

	// Numberは、スレッドの中で投稿を指すレス番号です (ADR 0009)。このページで投稿を
	// 名指すのも、フォームの送信先のアドレスを組み立てるのもこれであるため、画面は自身が
	// 示している投稿に対して働きかけます。
	Number int

	// Authorは投稿を書いたアカウントのatnameで、先頭の@を含みません。作者が退会して
	// いるときは""です。スレッドでそうであるのと同じく、どちらの場合も投稿は示します。退会が
	// 外すのは書かれたものから名前であって、書かれたものではありません。
	Author string

	Body       string
	Reason     string
	CSRFToken  string
	FormErrors *model.ValidationError
}
