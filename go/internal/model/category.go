package model

import "time"

// Category groups the boards a community offers and gives the sidebar its
// headings. It is a navigation aid rather than part of a board's address: a
// board is reached by its own slug, so moving one between categories leaves
// every link to that board intact.
//
// An instance hosts exactly one community (ADR 0006), so a category belongs to
// the instance as a whole and carries nothing pointing at a community.
//
// [Ja] Category はコミュニティが提供する掲示板をまとめ、サイドバーに見出しを与えます。
// 掲示板のアドレスの一部ではなくナビゲーションの手がかりです。掲示板は自身の slug で
// 辿り着くため、掲示板をカテゴリー間で移してもその掲示板へのリンクはすべて保たれます。
//
// 1 インスタンスはちょうど 1 つのコミュニティを運営する (ADR 0006) ため、カテゴリーは
// インスタンス全体に属し、コミュニティを指すものは持ちません。
type Category struct {
	ID CategoryID

	// Slug is the lowercase ASCII identifier /c/{slug} resolves. Database
	// uniqueness ignores letter case so a case variant cannot be stored beside
	// its canonical value.
	//
	// [Ja] Slug は /c/{slug} が解決する小文字 ASCII の識別子です。DB の一意性は
	// 大文字小文字を無視するため、正規値と大小だけが異なる値を並べて保存できません。
	Slug string

	Name string

	// Position is the order the community intends its categories to appear in,
	// ascending. Neither name nor creation order can express that intent.
	//
	// [Ja] Position はコミュニティがカテゴリーを並べたい順序で、昇順です。名前順でも
	// 作成順でもその意図は表せません。
	Position int

	CreatedAt time.Time
	UpdatedAt time.Time
}
