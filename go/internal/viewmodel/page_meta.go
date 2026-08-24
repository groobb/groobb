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
	// Title is the page title rendered into the <title> tag.
	//
	// [Ja] Title は <title> タグに描画されるページタイトルです。
	Title string

	// Description is the page description rendered into the meta description tag.
	//
	// [Ja] Description は meta description タグに描画されるページ説明です。
	Description string

	// AssetVersion is the cache-busting query value appended to CSS / JS URLs.
	//
	// [Ja] AssetVersion は CSS / JS の URL に付与するキャッシュ無効化用のクエリ値です。
	AssetVersion string

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
	// to home. The handlers registered behind RequireAuth set it: only a signed-in
	// visitor reaches their pages, and without the header the visitor has nothing
	// but the browser's back button to leave them by. Which of the layout's shared
	// parts a page carries is decided by the handler and carried here, the same
	// way NoIndex is, so the layout renders what the page asked for instead of
	// inspecting the request for itself.
	//
	// [Ja] SignedIn が true のとき、ホームへ戻る導線を持つ共通ヘッダーを描画します。
	// RequireAuth の背後に登録されたハンドラーが設定します。それらのページにはサイン
	// イン済みの訪問者しか到達せず、ヘッダーが無いとブラウザの戻る操作以外にそこから
	// 出る手段がありません。ページがレイアウトのどの共通部品を備えるかは NoIndex と
	// 同じくハンドラーが決めてここで運び、レイアウトは自分でリクエストを調べるのでは
	// なくページが求めたものを描画します。
	SignedIn bool
}

// DefaultPageMeta returns the metadata used as a baseline for every page. The
// Title and Description default to the site-wide localized values resolved from
// ctx; callers override them with page-specific text as needed.
//
// [Ja] DefaultPageMeta は全ページの基準となるメタ情報を返します。Title と
// Description は ctx から解決したサイト全体のローカライズ済みの既定値で、呼び出し元
// が必要に応じてページ固有の文言で上書きします。
func DefaultPageMeta(ctx context.Context, cfg *config.Config) PageMeta {
	return PageMeta{
		Title:        i18n.T(ctx, "default_title"),
		Description:  i18n.T(ctx, "default_description"),
		AssetVersion: cfg.GetAssetVersion(),
	}
}
