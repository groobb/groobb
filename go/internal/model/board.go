package model

import "time"

// Board is where threads are posted: the unit a community's conversations are
// divided into, and the destination a person navigates to.
//
// [Ja] Board はスレッドが立つ場所であり、コミュニティの会話を分ける単位であると同時に、
// 人が辿り着く行き先です。
type Board struct {
	ID BoardID

	// CategoryID is the category that lists this board, and nil for a board that
	// sits in none. Belonging to no category is a normal state rather than a gap
	// waiting to be filled (ADR 0011), and deleting a category returns the boards
	// it listed to that state.
	//
	// [Ja] CategoryID はこの掲示板を並べるカテゴリーで、どのカテゴリーにも属さない
	// 掲示板では nil です。どのカテゴリーにも属さないことは埋めるべき欠落ではなく正常な
	// 状態であり (ADR 0011)、カテゴリーを削除すると、それが並べていた掲示板はその状態へ
	// 戻ります。
	CategoryID *CategoryID

	// Slug is the lowercase ASCII identifier /b/{slug} resolves. It is unique
	// across the instance rather than within a category, because the address
	// names a board without naming the category it currently sits in.
	//
	// [Ja] Slug は /b/{slug} が解決する小文字 ASCII の識別子です。カテゴリー内ではなく
	// インスタンス全体で一意なのは、アドレスが掲示板を、それが今どのカテゴリーに
	// 属するかを言わずに名指しするためです。
	Slug string

	Name        string
	Description string

	// Position is the order the community intends its boards to appear in,
	// ascending. The sidebar orders every board of the community by it and a
	// category's page the ones that category lists, so the single column serves
	// both without one of them needing an order of its own.
	//
	// [Ja] Position はコミュニティが掲示板を並べたい順序で、昇順です。サイドバーは
	// コミュニティのすべての掲示板を、カテゴリーのページはそのカテゴリーが並べる掲示板を
	// これで並べるため、片方が独自の順序を持たずとも 1 つの列が双方をまかないます。
	Position int

	CreatedAt time.Time
	UpdatedAt time.Time
}
