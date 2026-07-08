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

// PageMeta holds the per-page metadata rendered into the HTML <head>.
//
// [Ja] PageMeta は HTML の <head> に描画されるページ単位のメタ情報を保持します。
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
