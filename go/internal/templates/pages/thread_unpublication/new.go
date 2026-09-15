// thread_unpublicationパッケージは、スレッドの非公開を確認するページ
// (GET /t/{id}/unpublication/new) を持ちます。コミュニティのスレッドがそこから視界の外へ
// 移される唯一の画面です。
package thread_unpublication

import (
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// NewPageDataは、非公開の確認ページが描かれる元です。操作の対象となるスレッド、
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
