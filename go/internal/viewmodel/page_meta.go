// Package viewmodel provides presentation-layer data structures that convert
// domain models into shapes the templates render.
//
// [Ja] viewmodel パッケージは、ドメインモデルをテンプレートが描画する形へ変換する
// プレゼンテーション層のデータ構造を提供します。
package viewmodel

import (
	"context"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/i18n"
)

// PageMeta holds the per-page metadata the shared layout renders: the contents
// of the HTML <head>, and which of the layout's shared parts the page carries.
//
// [Ja] PageMeta は共通レイアウトが描画するページ単位のメタ情報 (HTML の <head> の
// 内容と、そのページが備えるレイアウトの共通部品) を保持します。
type PageMeta struct {
	// Title is the name of the page itself: the part of the <title> element that
	// tells one page of this site from another. A handler sets it and nothing else
	// of the title, since what the rendered <title> holds is decided by
	// DocumentTitle.
	//
	// [Ja] Title はページ自身の名前であり、<title> 要素のうち、このサイトのページ同士を
	// 見分けさせる部分です。ハンドラーが設定するのはこれだけで、描画される <title> が何を
	// 持つかは DocumentTitle が決めます。
	Title string

	// SiteName is the name of the site the page belongs to, which DocumentTitle
	// puts after the name of the page. An instance serves exactly one community
	// (ADR 0006), so the site a visitor is on is that community: the name is what
	// the people who gather here call the place, not "Groobb", which names only the
	// software the place runs on.
	//
	// It is empty on an instance whose community has not been created yet, and the
	// page then carries its own name alone.
	//
	// [Ja] SiteName はページが属するサイトの名前であり、DocumentTitle がページの名前の
	// 後ろに置きます。1 インスタンスはちょうど 1 つのコミュニティを運営する (ADR 0006)
	// ため、訪問者がいるサイトはそのコミュニティです。名前はここに集まる人がこの場所を何と
	// 呼ぶかであって、その場所が動いているソフトウェアを指すだけの "Groobb" ではありません。
	//
	// コミュニティがまだ作られていないインスタンスでは空になり、その場合ページは自身の名前
	// だけを運びます。
	SiteName string

	// Description is the page description rendered into the meta description tag.
	//
	// [Ja] Description は meta description タグに描画されるページ説明です。
	Description string

	// AssetVersion is the cache-busting query value appended to CSS / JS URLs.
	//
	// [Ja] AssetVersion は CSS / JS の URL に付与するキャッシュ無効化用のクエリ値です。
	AssetVersion string

	// CanonicalURL is the absolute address the page declares as the one it is to
	// be known by, rendered as <link rel="canonical">. It gathers onto that one
	// address the signals of every other address the same page answers under — a
	// spelling that redirects here, or the same URL carrying a campaign parameter
	// — which are otherwise counted as so many separate pages.
	//
	// A page that asks not to be indexed has no signals to gather, so Head leaves
	// the link out there whatever this holds. On an instance whose public base URL
	// is not configured this is empty: a canonical link is expected to be
	// absolute, and there is then no host to build one from.
	//
	// [Ja] CanonicalURL は、そのページが自身を知られるべき 1 つのアドレスとして宣言する
	// 絶対アドレスであり、<link rel="canonical"> として描画されます。同じページが応答する
	// 他のどのアドレス (ここへリダイレクトされる綴りや、キャンペーンのパラメータを伴う
	// 同じ URL) のシグナルも、その 1 つのアドレスへ集めます。集めなければ、それらは別々の
	// ページとして数えられます。
	//
	// インデックスされないよう求めるページは集めるシグナルを持たないため、この値が何を
	// 保持していても Head はそこでリンクを描画しません。公開ベース URL が設定されていない
	// インスタンスでは、この値自体が空になります。canonical のリンクは絶対 URL であること
	// が期待され、その場合それを組み立てるホストが無いためです。
	CanonicalURL string

	// NoIndex, when true, renders a <meta name="robots" content="noindex"> so
	// search engines keep the page out of their index. Set it on per-user or
	// behind-auth pages (e.g. the home page) that should not be indexed; public
	// pages leave it false to inherit the implicit index, follow default.
	//
	// [Ja] NoIndex が true のとき <meta name="robots" content="noindex"> を描画し、
	// 検索エンジンにページをインデックスさせません。ユーザー固有 / 認証背後のページ
	// (例: ホームページ) で設定します。公開ページは false のままとし、暗黙の
	// index, follow の既定に従います。
	NoIndex bool

	// SignedIn, when true, renders the shared header carrying the navigation back
	// to home. The behind-RequireAuth handlers drawn in the Default layout set it:
	// only a signed-in visitor reaches their pages, and without the header the
	// visitor has nothing but the browser's back button to leave them by. A page
	// drawn in the Community layout leaves it alone, since there the sidebar is
	// what carries the navigation and the account controls. Which of the layout's
	// shared parts a page carries is decided by the handler and carried here, the
	// same way NoIndex is, so the layout renders what the page asked for instead
	// of inspecting the request for itself.
	//
	// [Ja] SignedIn が true のとき、ホームへ戻る導線を持つ共通ヘッダーを描画します。
	// Default レイアウトで描画される RequireAuth の背後のハンドラーが設定します。
	// それらのページにはサインイン済みの訪問者しか到達せず、ヘッダーが無いとブラウザの
	// 戻る操作以外にそこから出る手段がありません。Community レイアウトで描画されるページは
	// これを設定しません。そちらではサイドバーがナビゲーションとアカウント操作を運ぶため
	// です。ページがレイアウトのどの共通部品を備えるかは NoIndex と同じくハンドラーが
	// 決めてここで運び、レイアウトは自分でリクエストを調べるのではなくページが求めたものを
	// 描画します。
	SignedIn bool
}

// DefaultPageMeta returns the metadata used as a baseline for every page. The
// Title and Description default to the site-wide localized values resolved from
// ctx; callers override them with page-specific text as needed.
//
// The site name is read from the context, where the middleware that resolves the
// community put it, so that a handler sets the name of its own page and nothing
// else of the title.
//
// [Ja] DefaultPageMeta は全ページの基準となるメタ情報を返します。Title と
// Description は ctx から解決したサイト全体のローカライズ済みの既定値で、呼び出し元
// が必要に応じてページ固有の文言で上書きします。
//
// サイトの名前は、コミュニティを解決するミドルウェアが置いた context から読みます。
// ハンドラーが設定するのは自身のページの名前だけで、タイトルの他の部分に触れないように
// するためです。
func DefaultPageMeta(ctx context.Context, cfg *config.Config) PageMeta {
	return PageMeta{
		Title:        i18n.T(ctx, "default_title"),
		SiteName:     SiteNameFromContext(ctx),
		Description:  i18n.T(ctx, "default_description"),
		AssetVersion: cfg.GetAssetVersion(),
	}
}

// DocumentTitle returns what the <title> element carries: the name of the page,
// followed by the name of the site it belongs to. The name of the page comes
// first because a browser tab, a bookmark and a search result all cut the end
// off, and what is left has to be the part that tells the pages apart.
//
// Composing it here rather than in each handler is what keeps every page's title
// the same shape.
//
// A page on an instance whose community has not been created yet carries its own
// name alone: there is no name to end it with, and a separator with nothing after
// it reads as a title that failed to load.
//
// [Ja] DocumentTitle は <title> 要素が運ぶもの、すなわちページの名前と、それに続く、
// ページが属するサイトの名前を返します。ページの名前を先に置くのは、ブラウザのタブ・
// ブックマーク・検索結果のいずれもが末尾を切り詰めるためで、残るほうがページ同士を
// 見分けさせる部分である必要があります。
//
// 各ハンドラーではなくここで組み立てることが、どのページのタイトルも同じ形に保ちます。
//
// コミュニティがまだ作られていないインスタンスのページは、自身の名前だけを運びます。
// 末尾に置く名前が無く、後ろに何も続かない区切りは、読み込みに失敗したタイトルに見える
// ためです。
func (m PageMeta) DocumentTitle(ctx context.Context) string {
	if m.SiteName == "" {
		return m.Title
	}

	return i18n.T(ctx, "document_title", map[string]any{"Page": m.Title, "Site": m.SiteName})
}
