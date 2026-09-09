package viewmodel

import "github.com/groobb/groobb/go/internal/model"

// UserID is a user's identifier as the Presentation layer holds it. It exists
// for the reason ThreadID does: a page that names a user names them by id, while
// templates stay free of a dependency on the domain.
//
// It is defined as its own type over model.UserID rather than as an alias, so
// that the conversion is written where a handler builds the page's data and
// cannot happen by accident anywhere else.
//
// [Ja] UserID は、Presentation 層が保持する形の利用者の識別子です。存在する理由は
// ThreadID と同じで、利用者を名指すページはその id で名指す一方、テンプレートはドメインへの
// 依存から遠ざけるためです。
//
// エイリアスではなく model.UserID を基にした独自の型として定義するのは、変換が、
// ハンドラーがページのデータを組み立てる場所に書かれ、他のどこでも偶然には起きない
// ようにするためです。
type UserID model.UserID

// String returns the decimal form of the UserID, which is how a user is spelled
// in the element ids and addresses a page builds for one.
//
// [Ja] String は UserID を 10 進表記で返します。ページが利用者ごとに組み立てる要素 id や
// アドレスで、その利用者が綴られる形がこれです。
func (id UserID) String() string { return model.UserID(id).String() }
