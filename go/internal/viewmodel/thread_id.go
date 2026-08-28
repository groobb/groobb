package viewmodel

import "github.com/groobb/groobb/go/internal/model"

// ThreadID is a thread's identifier as the Presentation layer holds it. It
// exists so that a path helper naming a thread takes a thread's id and nothing
// else, while templates stay free of a dependency on the domain: a template
// that could reach model would also be able to take a domain entity straight
// from a UseCase, skipping the conversion this package exists to perform.
//
// It is defined as its own type over model.ThreadID rather than as an alias, so
// that the conversion is written where a handler builds the page's data and
// cannot happen by accident anywhere else.
//
// [Ja] ThreadID は、Presentation 層が保持する形のスレッドの識別子です。スレッドを
// 名指すパスヘルパーがスレッドの id だけを受け取れるようにしつつ、テンプレートを
// ドメインへの依存から遠ざけるために存在します。model へ届くテンプレートは、UseCase から
// ドメインのエンティティをそのまま受け取ることもできてしまい、それは本パッケージが行う
// 変換を飛ばす道になります。
//
// エイリアスではなく model.ThreadID を基にした独自の型として定義するのは、変換が、
// ハンドラーがページのデータを組み立てる場所に書かれ、他のどこでも偶然には起きない
// ようにするためです。
type ThreadID model.ThreadID

// String returns the decimal form of the ThreadID, which is how a thread is
// spelled in the path /t/{id}.
//
// [Ja] String は ThreadID を 10 進表記で返します。パス /t/{id} でスレッドが綴られる
// 形がこれです。
func (id ThreadID) String() string { return model.ThreadID(id).String() }
