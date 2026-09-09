// Package admin_user holds the templates of the admin user listing: the
// community's accounts, what each of them holds, and the way roles are handed
// out from there.
//
// [Ja] admin_user パッケージは管理画面の利用者一覧のテンプレートを保持します。
// コミュニティのアカウント、それぞれが持つもの、そしてそこからロールを渡す手立てです。
package admin_user

import (
	"strconv"
	"time"

	"github.com/groobb/groobb/go/internal/viewmodel"
)

// IndexPageData is the data of one page of the user listing: the accounts on it,
// what the listing is narrowed to, how many accounts matched in all, and where
// the page sits among the others.
//
// AtnamePrefix is echoed back into the search field so a listing that came back
// narrowed says what it was narrowed by, and it is carried by the paging links
// so moving between pages keeps the search rather than starting over.
// TotalCount distinguishes a listing with no matches from a page past the end
// of a listing that does have matches, so the empty state never describes the
// whole listing incorrectly.
//
// [Ja] IndexPageData は利用者一覧の 1 ページのデータです。そこに並ぶアカウント、一覧が
// 何で絞り込まれているか、全体で何件が一致したか、そしてそのページが他のページのどこに
// 位置するかを持ちます。
//
// AtnamePrefix は検索フィールドへ書き戻します。絞り込まれて返ってきた一覧が、何で
// 絞り込まれたのかを述べるためです。ページ送りのリンクもこれを運び、ページを移っても
// 検索が最初からやり直しにならないようにします。
// TotalCount は、一致するものが無い一覧と、一致するものはあるものの終端を越えたページを
// 区別します。空状態が一覧全体を誤って説明しないようにするためです。
type IndexPageData struct {
	AtnamePrefix string
	TotalCount   int
	Users        []IndexUser
	Pagination   IndexPagination

	// CSRFToken is what the role forms of every row echo back. The listing is
	// read to act on it, and each row's action is a form of its own.
	//
	// [Ja] CSRFToken は、どの行のロールのフォームも書き戻す値です。一覧は行に対して
	// 操作するために読まれ、その操作は行ごとに 1 つのフォームであるためです。
	CSRFToken string

	// AdminRoleName is the built-in role as it is stored, which the forms carry to
	// name what they grant and revoke. It is passed in rather than written into
	// the template, so the page names a role without knowing the domain the name
	// comes from.
	//
	// [Ja] AdminRoleName は保存されている形の組み込みロールで、フォームが何を付与し
	// 何を剥奪するのかを名指すために運びます。テンプレートに書き込むのではなく渡すのは、
	// ページが、名前の出どころであるドメインを知らずにロールを名指すためです。
	AdminRoleName string
}

// IndexUser is one row of the listing: an account, and what the community has
// given it. The roles are named as the page shows them rather than as they are
// stored, since the built-in role is read under its translated name.
//
// [Ja] IndexUser は一覧の 1 行、すなわち 1 つのアカウントと、コミュニティがそれに与えた
// ものです。ロールは保存されている形ではなくページが見せる形で名指します。組み込みの
// ロールは訳された名前で読まれるためです。
type IndexUser struct {
	// ID is what the row's element ids are built from, so that the button acting
	// on an account is named after the account it acts on. Two accounts can carry
	// the same words in a cell, but never the same id.
	//
	// [Ja] ID は行の要素 id の元になります。あるアカウントに作用するボタンが、作用する
	// 相手の名前で呼ばれるようにするためです。2 つのアカウントが 1 つのセルに同じ文字列を
	// 載せることはあっても、同じ id を持つことはありません。
	ID viewmodel.UserID

	Atname    string
	CreatedAt time.Time
	RoleNames []string

	// HoldsAdmin decides which of the two role buttons the row offers, since an
	// account either has the administrator role to be taken from it or has not
	// been given it yet.
	//
	// [Ja] HoldsAdmin は、行が 2 つのロールのボタンのどちらを差し出すかを決めます。
	// アカウントは、外す対象となる管理者ロールを持っているか、まだ与えられていないかの
	// いずれかであるためです。
	HoldsAdmin bool

	// IsSelf marks the row of the account reading the listing, which is the one
	// revocation the confirmation has to phrase differently: taking the role from
	// oneself removes the listing being read, and no button remains to press it
	// back with.
	//
	// [Ja] IsSelf は、一覧を読んでいるアカウント自身の行を示します。確認の言い回しを
	// 変える必要があるのはこの剥奪だけです。自分からロールを外すことは、読んでいる一覧
	// そのものを取り去ることであり、押し戻すためのボタンも残らないためです。
	IsSelf bool
}

// IndexPagination is where the page being read sits among the pages the listing
// has, which is all the paging links need to know: which page to step to, and
// whether there is one to step to at all.
//
// TotalPages is 0 for a listing nothing matched, where the page being read is
// numbered by nothing. A page past the last one is not, so the numbers can sit
// beyond the end and the visitor still finds the way back.
//
// [Ja] IndexPagination は、読まれているページが一覧の持つページのどこに位置するかです。
// ページ送りのリンクが知る必要のあるものはこれだけで、どのページへ進むか、そもそも進む
// 先があるかを決めます。
//
// TotalPages は、何も一致しなかった一覧では 0 になります。そこで読まれているページには
// どの番号も振られていません。最後のページより後ろの番号はそうではないため、番号が終端の
// 外に位置したまま、訪問者が戻る道を見つけられます。
type IndexPagination struct {
	Page       int
	TotalPages int
}

// IsNumbered reports whether the page being read is one of the pages the
// listing numbers. A page past the last one is not, and neither is any page of a
// listing nothing matched, so where the page sits among the others is said only
// where there is an answer.
//
// [Ja] IsNumbered は、読まれているページが一覧の振る番号のうちの 1 つかどうかを返します。
// 最後のページより後ろのページはそうではなく、何も一致しなかった一覧のどのページもそう
// ではありません。そのページが他のページのどこに位置するかを述べるのは、答えのある場合
// だけにするためです。
func (p IndexPagination) IsNumbered() bool { return p.TotalPages > 0 && p.Page <= p.TotalPages }

// HasPrevious reports whether the listing holds a page to step back to. A
// listing nothing matched holds none, so a page of it offers no way back to
// accounts it does not have.
//
// [Ja] HasPrevious は、一覧が戻る先のページを持っているかどうかを返します。何も一致
// しなかった一覧はそれを 1 つも持たないため、そのページは、持っていないアカウントへ戻る
// 道を差し出しません。
func (p IndexPagination) HasPrevious() bool { return p.TotalPages > 0 && p.Page > 1 }

// HasNext reports whether a page after this one exists.
//
// [Ja] HasNext はこのページより後のページが存在するかどうかを返します。
func (p IndexPagination) HasNext() bool { return p.Page < p.TotalPages }

// PreviousPage is the page the backward link steps to. From a page past the end
// it is the last page the listing numbers, rather than the number one below the
// one being read: an address kept from when the listing was longer can sit far
// beyond the end, and stepping back a page at a time would cross pages that are
// empty for the same reason this one is.
//
// [Ja] PreviousPage は後ろ向きのリンクが進む先のページです。終端を越えたページからは、
// 読まれている番号の 1 つ下ではなく、一覧が振る最後のページになります。一覧がもっと
// 長かった頃のアドレスは終端のはるか先に位置しうるため、1 ページずつ戻れば、このページと
// 同じ理由で空であるページを次々に渡ることになるからです。
func (p IndexPagination) PreviousPage() int { return min(p.Page-1, p.TotalPages) }

// NextPage is the page the forward link steps to.
//
// [Ja] NextPage は前向きのリンクが進む先のページです。
func (p IndexPagination) NextPage() int { return p.Page + 1 }

// PageValue is the page being read as the row forms carry it back, so a grant or
// a revoke returns to the listing it was pressed on rather than to its first
// page. The search is carried beside it, and the two together are the address
// the listing was read at.
//
// [Ja] PageValue は、行のフォームが書き戻す形の「読まれているページ」です。付与や剥奪が、
// 一覧の最初のページではなく、押された一覧へ戻るためです。絞り込みはその隣で運ばれ、
// 2 つ合わせて一覧が読まれたアドレスになります。
func (p IndexPagination) PageValue() string { return strconv.Itoa(p.Page) }

// atnameCellID returns the id of the cell naming the account a row is about. The
// role button of the row is named after it, so a button read on its own says
// which account it acts on rather than repeating the same words on every row.
//
// [Ja] atnameCellID は、行がどのアカウントについてのものかを名指すセルの id を返します。
// 行のロールのボタンはこれによって名付けられるため、単独で読まれたボタンが、どの行でも
// 同じ文字列を繰り返すのではなく、作用する相手のアカウントを述べます。
func atnameCellID(id viewmodel.UserID) string {
	return "admin-user-atname-" + id.String()
}

// roleButtonID returns the id of the row's role button. The button lists it
// first in its own aria-labelledby, so its accessible name opens with what it
// does and closes with who it does it to.
//
// [Ja] roleButtonID は行のロールのボタンの id を返します。ボタンは自身の
// aria-labelledby でこれを最初に並べるため、そのアクセシブルネームは、何をするのかで
// 始まり、誰に対してするのかで終わります。
func roleButtonID(id viewmodel.UserID) string {
	return "admin-user-role-button-" + id.String()
}
