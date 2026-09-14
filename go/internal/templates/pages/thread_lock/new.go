// Package thread_lock holds the page a thread's lock is confirmed on
// (GET /t/{id}/lock/new), the one screen the community's threads are closed
// from.
//
// [Ja] thread_lockパッケージは、スレッドのロックを確認するページ
// (GET /t/{id}/lock/new) を持ちます。コミュニティのスレッドがそこから閉じられる唯一の
// 画面です。
package thread_lock

import (
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// NewPageData is what the lock confirmation page is drawn from: the thread the
// operation is about, the note that travels with it, and the token the
// submission carries.
//
// The thread is named by its id and its title rather than by the title alone,
// because the page has to both say which thread is about to be closed and
// submit to that thread's own address. The language comes along for the reason
// the thread's own page carries it: the title is written in the thread's
// language while the page around it is not.
//
// Reason is echoed back, unlike the password on the withdrawal form: it is not
// a credential, and a note refused for its length is one the administrator
// should be able to shorten rather than write again.
//
// [Ja] NewPageDataは、ロックの確認ページが描かれる元です。操作の対象となるスレッド、
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
