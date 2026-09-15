package model

import "time"

// Communityはこのインスタンスが運営するコミュニティです。ここに集まる人が
// この場所を何と呼ぶかであり、その会話を分けるカテゴリーや掲示板の上に立ちます。
//
// 1インスタンスはちょうど1つのコミュニティを運営する (ADR 0006) ため、これが2つ
// 存在することはなく、これを指すものもありません。カテゴリーはインスタンス全体に
// 属します。行はインスタンスの立ち上げが作るため、マイグレーション直後のデータベースには
// 存在せず、読み取る側はそれが無い場合にも答えられる必要があります。
type Community struct {
	ID   CommunityID
	Name string

	CreatedAt time.Time
	UpdatedAt time.Time
}
