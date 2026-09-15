// admin_userパッケージは管理画面の利用者一覧のテンプレートを保持します。
// コミュニティのアカウント、それぞれが持つもの、そしてそこからロールを渡す手立てです。
package admin_user

import (
	"strconv"
	"time"

	"github.com/groobb/groobb/go/internal/viewmodel"
)

// IndexPageDataは利用者一覧の1ページのデータです。そこに並ぶアカウント、一覧が
// 何で絞り込まれているか、全体で何件が一致したか、そしてそのページが他のページのどこに
// 位置するかを持ちます。
//
// AtnamePrefixは検索フィールドへ書き戻します。絞り込まれて返ってきた一覧が、何で
// 絞り込まれたのかを述べるためです。ページ送りのリンクもこれを運び、ページを移っても
// 検索が最初からやり直しにならないようにします。
// TotalCountは、一致するものが無い一覧と、一致するものはあるものの終端を越えたページを
// 区別します。空状態が一覧全体を誤って説明しないようにするためです。
type IndexPageData struct {
	AtnamePrefix string
	TotalCount   int
	Users        []IndexUser
	Pagination   IndexPagination

	// CSRFTokenは、どの行のロールのフォームも書き戻す値です。一覧は行に対して
	// 操作するために読まれ、その操作は行ごとに1つのフォームであるためです。
	CSRFToken string

	// AdminRoleNameは保存されている形の組み込みロールで、フォームが何を付与し
	// 何を剥奪するのかを名指すために運びます。テンプレートに書き込むのではなく渡すのは、
	// ページが、名前の出どころであるドメインを知らずにロールを名指すためです。
	AdminRoleName string
}

// IndexUserは一覧の1行、すなわち1つのアカウントと、コミュニティがそれに与えた
// ものです。ロールは保存されている形ではなくページが見せる形で名指します。組み込みの
// ロールは訳された名前で読まれるためです。
type IndexUser struct {
	// IDは行の要素idの元になります。あるアカウントに作用するボタンが、作用する
	// 相手の名前で呼ばれるようにするためです。2つのアカウントが1つのセルに同じ文字列を
	// 載せることはあっても、同じidを持つことはありません。
	ID viewmodel.UserID

	Atname    string
	CreatedAt time.Time
	RoleNames []string

	// HoldsAdminは、行が2つのロールのボタンのどちらを差し出すかを決めます。
	// アカウントは、外す対象となる管理者ロールを持っているか、まだ与えられていないかの
	// いずれかであるためです。
	HoldsAdmin bool

	// IsSelfは、一覧を読んでいるアカウント自身の行を示します。確認の言い回しを
	// 変える必要があるのはこの剥奪だけです。自分からロールを外すことは、読んでいる一覧
	// そのものを取り去ることであり、押し戻すためのボタンも残らないためです。
	IsSelf bool
}

// IndexPaginationは、読まれているページが一覧の持つページのどこに位置するかです。
// ページ送りのリンクが知る必要のあるものはこれだけで、どのページへ進むか、そもそも進む
// 先があるかを決めます。
//
// TotalPagesは、何も一致しなかった一覧では0になります。そこで読まれているページには
// どの番号も振られていません。最後のページより後ろの番号はそうではないため、番号が終端の
// 外に位置したまま、訪問者が戻る道を見つけられます。
type IndexPagination struct {
	Page       int
	TotalPages int
}

// IsNumberedは、読まれているページが一覧の振る番号のうちの1つかどうかを返します。
// 最後のページより後ろのページはそうではなく、何も一致しなかった一覧のどのページもそう
// ではありません。そのページが他のページのどこに位置するかを述べるのは、答えのある場合
// だけにするためです。
func (p IndexPagination) IsNumbered() bool { return p.TotalPages > 0 && p.Page <= p.TotalPages }

// HasPreviousは、一覧が戻る先のページを持っているかどうかを返します。何も一致
// しなかった一覧はそれを1つも持たないため、そのページは、持っていないアカウントへ戻る
// 道を差し出しません。
func (p IndexPagination) HasPrevious() bool { return p.TotalPages > 0 && p.Page > 1 }

// HasNextはこのページより後のページが存在するかどうかを返します。
func (p IndexPagination) HasNext() bool { return p.Page < p.TotalPages }

// PreviousPageは後ろ向きのリンクが進む先のページです。終端を越えたページからは、
// 読まれている番号の1つ下ではなく、一覧が振る最後のページになります。一覧がもっと
// 長かった頃のアドレスは終端のはるか先に位置しうるため、1ページずつ戻れば、このページと
// 同じ理由で空であるページを次々に渡ることになるからです。
func (p IndexPagination) PreviousPage() int { return min(p.Page-1, p.TotalPages) }

// NextPageは前向きのリンクが進む先のページです。
func (p IndexPagination) NextPage() int { return p.Page + 1 }

// PageValueは、行のフォームが書き戻す形の「読まれているページ」です。付与や剥奪が、
// 一覧の最初のページではなく、押された一覧へ戻るためです。絞り込みはその隣で運ばれ、
// 2つ合わせて一覧が読まれたアドレスになります。
func (p IndexPagination) PageValue() string { return strconv.Itoa(p.Page) }

// atnameCellIDは、行がどのアカウントについてのものかを名指すセルのidを返します。
// 行のロールのボタンはこれによって名付けられるため、単独で読まれたボタンが、どの行でも
// 同じ文字列を繰り返すのではなく、作用する相手のアカウントを述べます。
func atnameCellID(id viewmodel.UserID) string {
	return "admin-user-atname-" + id.String()
}

// roleButtonIDは行のロールのボタンのidを返します。ボタンは自身の
// aria-labelledbyでこれを最初に並べるため、そのアクセシブルネームは、何をするのかで
// 始まり、誰に対してするのかで終わります。
func roleButtonID(id viewmodel.UserID) string {
	return "admin-user-role-button-" + id.String()
}
