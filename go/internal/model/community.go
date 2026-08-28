package model

import "time"

// Community is the community this instance hosts: the name the people who
// gather here call the place, above the categories and boards their
// conversations are divided into.
//
// An instance serves exactly one community (ADR 0006), so there is never more
// than one of these and nothing points at it — a category belongs to the
// instance as a whole. The row is created when the instance is set up, which
// means a freshly migrated database has none, and a reader has to be able to
// answer for its absence.
//
// [Ja] Community はこのインスタンスが運営するコミュニティです。ここに集まる人が
// この場所を何と呼ぶかであり、その会話を分けるカテゴリーや掲示板の上に立ちます。
//
// 1 インスタンスはちょうど 1 つのコミュニティを運営する (ADR 0006) ため、これが 2 つ
// 存在することはなく、これを指すものもありません。カテゴリーはインスタンス全体に
// 属します。行はインスタンスの立ち上げが作るため、マイグレーション直後のデータベースには
// 存在せず、読み取る側はそれが無い場合にも答えられる必要があります。
type Community struct {
	ID   CommunityID
	Name string

	CreatedAt time.Time
	UpdatedAt time.Time
}
