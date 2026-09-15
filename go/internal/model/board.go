package model

import "time"

// Boardはスレッドが立つ場所であり、コミュニティの会話を分ける単位であると同時に、
// 人が辿り着く行き先です。
type Board struct {
	ID BoardID

	// CategoryIDはこの掲示板を並べるカテゴリーで、どのカテゴリーにも属さない
	// 掲示板ではnilです。どのカテゴリーにも属さないことは埋めるべき欠落ではなく正常な
	// 状態であり (ADR 0011)、カテゴリーを削除すると、それが並べていた掲示板はその状態へ
	// 戻ります。
	CategoryID *CategoryID

	// Slugは /b/{slug} が解決する小文字ASCIIの識別子です。カテゴリー内ではなく
	// インスタンス全体で一意なのは、アドレスが掲示板を、それが今どのカテゴリーに
	// 属するかを言わずに名指しするためです。
	Slug string

	Name        string
	Description string

	// Positionはコミュニティが掲示板を並べたい順序で、昇順です。サイドバーは
	// コミュニティのすべての掲示板を、カテゴリーのページはそのカテゴリーが並べる掲示板を
	// これで並べるため、片方が独自の順序を持たずとも1つの列が双方をまかないます。
	Position int

	CreatedAt time.Time
	UpdatedAt time.Time
}
