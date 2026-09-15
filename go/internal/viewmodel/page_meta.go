// viewmodelパッケージは、ドメインモデルをテンプレートが描画する形へ変換する
// プレゼンテーション層のデータ構造を提供します。
package viewmodel

import (
	"context"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/i18n"
)

// PageMetaは共通レイアウトが描画するページ単位のメタ情報 (HTMLの <head> の
// 内容と、そのページが備えるレイアウトの共通部品) を保持します。
type PageMeta struct {
	// Titleはページ自身の名前であり、<title> 要素のうち、このサイトのページ同士を
	// 見分けさせる部分です。ハンドラーが設定するのはこれだけで、描画される <title> が何を
	// 持つかはDocumentTitleが決めます。
	Title string

	// SiteNameはページが属するサイトの名前であり、DocumentTitleがページの名前の
	// 後ろに置きます。1インスタンスはちょうど1つのコミュニティを運営する (ADR 0006)
	// ため、訪問者がいるサイトはそのコミュニティです。名前はここに集まる人がこの場所を何と
	// 呼ぶかであって、その場所が動いているソフトウェアを指すだけの "Groobb" ではありません。
	//
	// コミュニティがまだ作られていないインスタンスでは空になり、その場合ページは自身の名前
	// だけを運びます。
	SiteName string

	// Descriptionはmeta descriptionタグに描画されるページ説明です。
	Description string

	// AssetVersionはCSS / JSのURLに付与するキャッシュ無効化用のクエリ値です。
	AssetVersion string

	// CanonicalURLは、そのページが自身を知られるべき1つのアドレスとして宣言する
	// 絶対アドレスであり、<link rel="canonical"> として描画されます。同じページが応答する
	// 他のどのアドレス (ここへリダイレクトされる綴りや、キャンペーンのパラメータを伴う
	// 同じURL) のシグナルも、その1つのアドレスへ集めます。集めなければ、それらは別々の
	// ページとして数えられます。
	//
	// インデックスされないよう求めるページは集めるシグナルを持たないため、この値が何を
	// 保持していてもHeadはそこでリンクを描画しません。公開ベースURLが設定されていない
	// インスタンスでは、この値自体が空になります。canonicalのリンクは絶対URLであること
	// が期待され、その場合それを組み立てるホストが無いためです。
	CanonicalURL string

	// NoIndexがtrueのとき <meta name="robots" content="noindex"> を描画し、
	// 検索エンジンにページをインデックスさせません。ユーザー固有 / 認証背後のページ
	// (例: ホームページ) で設定します。公開ページはfalseのままとし、暗黙の
	// index, followの既定に従います。
	NoIndex bool

	// SignedInがtrueのとき、ホームへ戻る導線を持つ共通ヘッダーを描画します。
	// Defaultレイアウトで描画されるRequireAuthの背後のハンドラーが設定します。
	// それらのページにはサインイン済みの訪問者しか到達せず、ヘッダーが無いとブラウザの
	// 戻る操作以外にそこから出る手段がありません。Communityレイアウトで描画されるページは
	// これを設定しません。そちらではサイドバーがナビゲーションとアカウント操作を運ぶため
	// です。ページがレイアウトのどの共通部品を備えるかはNoIndexと同じくハンドラーが
	// 決めてここで運び、レイアウトは自分でリクエストを調べるのではなくページが求めたものを
	// 描画します。
	SignedIn bool
}

// DefaultPageMetaは全ページの基準となるメタ情報を返します。Titleと
// Descriptionはctxから解決したサイト全体のローカライズ済みの既定値で、呼び出し元
// が必要に応じてページ固有の文言で上書きします。
//
// サイトの名前は、コミュニティを解決するミドルウェアが置いたcontextから読みます。
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

// DocumentTitleは <title> 要素が運ぶもの、すなわちページの名前と、それに続く、
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
