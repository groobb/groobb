// thread_lockパッケージは、スレッドのロックを確認するページ
// (GET /t/{id}/lock/new) を持ちます。コミュニティのスレッドがそこから閉じられる唯一の
// 画面です。
package thread_lock

import (
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// NewPageDataは、ロックの確認ページが描かれる元です。操作の対象となるスレッド、
// それとともに運ばれる注記、そして送信が運ぶトークンを持ちます。
//
// スレッドをタイトルだけでなくidでも名指すのは、このページが、これから閉じられるのがどの
// スレッドかを述べると同時に、そのスレッド自身のアドレスへ送信しなければならないためです。
// 言語が伴うのは、スレッド自身のページがそれを運ぶのと同じ理由です。タイトルはスレッドの
// 言語で書かれており、その周りのページはそうではありません。
//
// Reasonは、退会フォームのパスワードと違ってエコーバックします。これは資格情報ではなく、
// 長さを理由に拒否された注記は、管理者が書き直すのではなく短くできるべきものであるためです。
type NewPageData struct {
	ThreadID   viewmodel.ThreadID
	Title      string
	Language   viewmodel.ThreadLanguage
	Reason     string
	CSRFToken  string
	FormErrors *model.ValidationError
}
