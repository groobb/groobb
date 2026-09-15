package model

import "time"

// Categoryはコミュニティが提供する掲示板をまとめ、サイドバーに見出しを与えます。
// 掲示板のアドレスの一部ではなくナビゲーションの手がかりです。掲示板は自身のslugで
// 辿り着くため、掲示板をカテゴリー間で移してもその掲示板へのリンクはすべて保たれます。
//
// 1インスタンスはちょうど1つのコミュニティを運営する (ADR 0006) ため、カテゴリーは
// インスタンス全体に属し、コミュニティを指すものは持ちません。
type Category struct {
	ID CategoryID

	// Slugは /c/{slug} が解決する小文字ASCIIの識別子です。DBの一意性は
	// 大文字小文字を無視するため、正規値と大小だけが異なる値を並べて保存できません。
	Slug string

	Name string

	// Positionはコミュニティがカテゴリーを並べたい順序で、昇順です。名前順でも
	// 作成順でもその意図は表せません。
	Position int

	CreatedAt time.Time
	UpdatedAt time.Time
}
