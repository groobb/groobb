package viewmodel

import "github.com/groobb/groobb/go/internal/model"

// ThreadIDは、Presentation層が保持する形のスレッドの識別子です。スレッドを
// 名指すパスヘルパーがスレッドのidだけを受け取れるようにしつつ、テンプレートを
// ドメインへの依存から遠ざけるために存在します。modelへ届くテンプレートは、UseCaseから
// ドメインのエンティティをそのまま受け取ることもできてしまい、それは本パッケージが行う
// 変換を飛ばす道になります。
//
// エイリアスではなくmodel.ThreadIDを基にした独自の型として定義するのは、変換が、
// ハンドラーがページのデータを組み立てる場所に書かれ、他のどこでも偶然には起きない
// ようにするためです。
type ThreadID model.ThreadID

// StringはThreadIDを10進表記で返します。パス /t/{id} でスレッドが綴られる
// 形がこれです。
func (id ThreadID) String() string { return model.ThreadID(id).String() }
