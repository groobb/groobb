// Package viewmodel provides presentation-layer data structures that convert
// domain models into shapes the templates render.
//
// [Ja] viewmodel パッケージは、ドメインモデルをテンプレートが描画する形へ変換する
// プレゼンテーション層のデータ構造を提供します。
package viewmodel

import "github.com/groobb/groobb/go/internal/config"

// PageMeta holds the per-page metadata rendered into the HTML <head>.
// [Ja] PageMeta は HTML の <head> に描画されるページ単位のメタ情報を保持します。
type PageMeta struct {
	// Title is the page title rendered into the <title> tag.
	// [Ja] Title は <title> タグに描画されるページタイトルです。
	Title string

	// Description is the page description rendered into the meta description tag.
	// [Ja] Description は meta description タグに描画されるページ説明です。
	Description string

	// AssetVersion is the cache-busting query value appended to CSS / JS URLs.
	// [Ja] AssetVersion は CSS / JS の URL に付与するキャッシュ無効化用のクエリ値です。
	AssetVersion string
}

// DefaultPageMeta returns the metadata used as a baseline for every page.
//
// The Title and Description are hard-coded placeholders until i18n lands in
// phase 4 and the welcome page in phase 5; from then on they are sourced from
// the locale files.
//
// [Ja] DefaultPageMeta は全ページの基準となるメタ情報を返します。
//
// Title と Description は、i18n がフェーズ 4 で、welcome ページがフェーズ 5 で
// 入るまでのハードコードされたプレースホルダーです。それ以降はロケールファイル
// から取得します。
func DefaultPageMeta(cfg *config.Config) PageMeta {
	return PageMeta{
		Title:        "Groobb",
		Description:  "Groobb is a bulletin board service.",
		AssetVersion: cfg.GetAssetVersion(),
	}
}
