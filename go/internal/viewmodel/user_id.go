package viewmodel

import "github.com/groobb/groobb/go/internal/model"

// UserIDは、Presentation層が保持する形の利用者の識別子です。存在する理由は
// ThreadIDと同じで、利用者を名指すページはそのidで名指す一方、テンプレートはドメインへの
// 依存から遠ざけるためです。
//
// エイリアスではなくmodel.UserIDを基にした独自の型として定義するのは、変換が、
// ハンドラーがページのデータを組み立てる場所に書かれ、他のどこでも偶然には起きない
// ようにするためです。
type UserID model.UserID

// StringはUserIDを10進表記で返します。ページが利用者ごとに組み立てる要素idや
// アドレスで、その利用者が綴られる形がこれです。
func (id UserID) String() string { return model.UserID(id).String() }
