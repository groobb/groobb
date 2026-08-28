package viewmodel

import (
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/usecase"
)

// Sidebar is what the community layout's sidebar renders: this community's
// name, the boards it offers, and the controls for the account viewing them.
//
// It lives here beside PageMeta rather than in the templ package of one page
// because every page of the community shell carries it, so each handler builds
// it the same way instead of repeating the conversion.
//
// [Ja] Sidebar はコミュニティレイアウトのサイドバーが描画するもの、すなわちこの
// コミュニティの名前・それが提供する掲示板・それを見ているアカウントの操作です。
//
// 特定のページの templ パッケージではなく PageMeta の隣に置くのは、コミュニティの
// シェルを持つどのページもこれを運ぶためです。各ハンドラーが変換を繰り返さず、同じ
// 組み立て方をします。
type Sidebar struct {
	// CommunityName is the name of the community this instance hosts, or "" when
	// the instance has not been set up yet. The sidebar renders the board
	// navigation either way.
	//
	// [Ja] CommunityName はこのインスタンスが運営するコミュニティの名前で、
	// インスタンスがまだ立ち上げられていないときは "" です。どちらの場合もサイドバーは
	// 板のナビゲーションを描画します。
	CommunityName string

	// Boards are the community's boards in the order it placed them, listed flat
	// rather than divided by the categories that group them (ADR 0011).
	//
	// [Ja] Boards はコミュニティの掲示板を、コミュニティが並べた順で保持します。
	// それらをまとめるカテゴリーで区切らず、フラットに並べます (ADR 0011)。
	Boards []SidebarBoard

	// SignedIn tells the sidebar which of the two account blocks to render: the
	// controls for the account viewing the community, or the way into one. The
	// public pages of the community are readable while signed out, so a visitor
	// without an account reaches that position too.
	//
	// [Ja] SignedIn はサイドバーに 2 つのアカウントのブロックのどちらを描画するかを
	// 伝えます。コミュニティを見ているアカウントの操作か、アカウントを持つための導線か
	// です。コミュニティの公開ページはサインアウト状態でも読めるため、アカウントを
	// 持たない訪問者もその位置に辿り着きます。
	SignedIn bool

	// Atname is the signed-in user's handle, shown above the account controls.
	//
	// [Ja] Atname はサインイン済みユーザーのハンドルで、アカウント操作の上に表示します。
	Atname string

	// CSRFToken is embedded as a hidden field in the sign-out form for the
	// double-submit-cookie check.
	//
	// [Ja] CSRFToken は double-submit cookie 検証のためサインアウトフォームに hidden
	// フィールドとして埋め込みます。
	CSRFToken string

	// ReturnTo is the page the sidebar is being rendered on, carried by the
	// sign-in link so that signing in from here returns the visitor to what they
	// were reading. It is a value middleware.SanitizeReturnTo has already
	// accepted, and it is empty for a signed-in visitor, who is shown no sign-in
	// link to put it on.
	//
	// [Ja] ReturnTo はサイドバーを今描画しているページで、サインインのリンクがこれを
	// 運びます。ここからサインインした訪問者を、読んでいたものへ戻すためです。値は
	// middleware.SanitizeReturnTo が受け付け済みのものです。サインイン済みの訪問者には
	// これを載せるサインインのリンクを描画しないため、空になります。
	ReturnTo string
}

// SidebarBoard is one board link of the sidebar. It carries the slug rather
// than the id because /b/{slug} is what addresses a board.
//
// [Ja] SidebarBoard はサイドバーの掲示板リンク 1 つです。掲示板を指すのは /b/{slug} で
// あるため、id ではなく slug を運びます。
type SidebarBoard struct {
	Slug string
	Name string
}

// NewSidebar converts the community navigation into the shape the sidebar
// renders, for the given viewer. A nil user is an anonymous visitor, whose
// account controls are replaced by the way into an account.
//
// Each viewer carries only what their own block renders: the sign-out form's
// CSRF token for a signed-in visitor, and the page to come back to for an
// anonymous one. returnTo is a value middleware.SanitizeReturnTo has already
// accepted, and every caller passes the page it is rendering without having to
// ask whether its own route can be reached while signed out.
//
// [Ja] NewSidebar はコミュニティのナビゲーションを、与えられた閲覧者にとってサイド
// バーが描画する形へ変換します。user が nil のときは匿名の訪問者であり、アカウント
// 操作はアカウントを持つための導線に置き換わります。
//
// 各閲覧者は自身のブロックが描画するものだけを運びます。サインイン済みの訪問者は
// サインアウトフォームの CSRF トークンを、匿名の訪問者は戻ってくる先のページをです。
// returnTo は middleware.SanitizeReturnTo が受け付け済みの値であり、どの呼び出し側も、
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
	} else {
		sidebar.ReturnTo = returnTo
	}

	return sidebar
}
