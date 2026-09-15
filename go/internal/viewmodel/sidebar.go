package viewmodel

import (
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/usecase"
)

// Sidebarはコミュニティレイアウトのサイドバーが描画するもの、すなわちこの
// コミュニティの名前・それが提供する掲示板・それを見ているアカウントの操作です。
//
// 特定のページのtemplパッケージではなくPageMetaの隣に置くのは、コミュニティの
// シェルを持つどのページもこれを運ぶためです。各ハンドラーが変換を繰り返さず、同じ
// 組み立て方をします。
type Sidebar struct {
	// CommunityNameはこのインスタンスが運営するコミュニティの名前で、
	// インスタンスがまだ立ち上げられていないときは "" です。どちらの場合もサイドバーは
	// 板のナビゲーションを描画します。
	CommunityName string

	// Boardsはコミュニティの掲示板を、コミュニティが並べた順で保持します。
	// それらをまとめるカテゴリーで区切らず、フラットに並べます (ADR 0011)。
	Boards []SidebarBoard

	// SignedInはサイドバーに2つのアカウントのブロックのどちらを描画するかを
	// 伝えます。コミュニティを見ているアカウントの操作か、アカウントを持つための導線か
	// です。コミュニティの公開ページはサインアウト状態でも読めるため、アカウントを
	// 持たない訪問者もその位置に辿り着きます。
	SignedIn bool

	// Atnameはサインイン済みユーザーのハンドルで、アカウント操作の上に表示します。
	Atname string

	// CSRFTokenはdouble-submit cookie検証のためサインアウトフォームにhidden
	// フィールドとして埋め込みます。
	CSRFToken string

	// CanAccessAdminはサイドバーに、管理画面への導線を描画するかどうかを伝えます。
	// 導線を描くのはそれを開いてよいアカウントに対してだけです。コミュニティのページが、
	// 拒否で応じる入口を差し出すことのないようにするためです。
	CanAccessAdmin bool

	// ReturnToはサイドバーを今描画しているページで、サインインのリンクがこれを
	// 運びます。ここからサインインした訪問者を、読んでいたものへ戻すためです。値は
	// middleware.SanitizeReturnToが受け付け済みのものです。サインイン済みの訪問者には
	// これを載せるサインインのリンクを描画しないため、空になります。
	ReturnTo string
}

// SidebarBoardはサイドバーの掲示板リンク1つです。掲示板を指すのは /b/{slug} で
// あるため、idではなくslugを運びます。
type SidebarBoard struct {
	Slug string
	Name string
}

// NewSidebarはコミュニティのナビゲーションを、与えられた閲覧者にとってサイド
// バーが描画する形へ変換します。userがnilのときは匿名の訪問者であり、アカウント
// 操作はアカウントを持つための導線に置き換わります。
//
// 各閲覧者は自身のブロックが描画するものだけを運びます。サインイン済みの訪問者は
// サインアウトフォームのCSRFトークンと管理画面が開かれているかどうかを、匿名の訪問者は
// 戻ってくる先のページをです。
// returnToはmiddleware.SanitizeReturnToが受け付け済みの値であり、どの呼び出し側も、
// 自身のルートがサインアウト状態で到達できるかを問わずに描画中のページを渡します。
func NewSidebar(nav *usecase.GetCommunityNavigationOutput, user *model.User, csrfToken, returnTo string) Sidebar {
	sidebar := Sidebar{Boards: make([]SidebarBoard, len(nav.Boards))}

	if nav.Community != nil {
		sidebar.CommunityName = nav.Community.Name
	}

	for i, board := range nav.Boards {
		sidebar.Boards[i] = SidebarBoard{Slug: board.Slug, Name: board.Name}
	}

	if user != nil {
		sidebar.SignedIn = true
		sidebar.Atname = user.Atname
		sidebar.CSRFToken = csrfToken
		sidebar.CanAccessAdmin = nav.CanAccessAdmin
	} else {
		sidebar.ReturnTo = returnTo
	}

	return sidebar
}
