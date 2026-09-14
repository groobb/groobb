// Package thread_unpublication holds the page a thread's unpublication is
// confirmed on (GET /t/{id}/unpublication/new), the one screen the community's
// threads are taken out of view from.
//
// [Ja] thread_unpublicationパッケージは、スレッドの非公開を確認するページ
// (GET /t/{id}/unpublication/new) を持ちます。コミュニティのスレッドがそこから視界の外へ
// 移される唯一の画面です。
package thread_unpublication

import (
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// NewPageData is what the unpublication confirmation page is drawn from: the
// thread the operation is about, the note that travels with it, and the token
// the submission carries.
//
// It holds what the lock's confirmation page holds, and for the same reasons:
// the thread is named by its id as well as its title because the page both says
// which thread is about to go and submits to that thread's own address, the
// language comes along because the title is written in the thread's language
// while the page around it is not, and the note is echoed back because one
// refused for its length should be shortened rather than written again.
//
// [Ja] NewPageDataは、非公開の確認ページが描かれる元です。操作の対象となるスレッド、
// それとともに運ばれる注記、そして送信が運ぶトークンを持ちます。
//
// 持つものはロックの確認ページと同じであり、理由も同じです。スレッドをタイトルだけでなくidでも
// 名指すのは、このページが、これから去るのがどのスレッドかを述べると同時に、そのスレッド自身の
// アドレスへ送信するためです。言語が伴うのは、タイトルがスレッドの言語で書かれており、その
// 周りのページはそうではないためです。注記をエコーバックするのは、長さを理由に拒否された注記は
// 書き直すのではなく短くされるべきものであるためです。
type NewPageData struct {
	ThreadID   viewmodel.ThreadID
	Title      string
	Language   viewmodel.ThreadLanguage
	Reason     string
	CSRFToken  string
	FormErrors *model.ValidationError
}
