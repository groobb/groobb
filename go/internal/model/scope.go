package model

// Scope names one thing a role lets its holders do, written as resource:action
// after the vocabulary the other Korylus services use. The names are the values
// stored in roles.scopes, so what a role grants is read back as the same words
// this file writes down.
//
// The set below is the whole vocabulary the application knows. A role may carry
// a name that is not in it -- roles.scopes takes any JSON array of strings --
// and such a name grants nothing, because no check is written against it. That
// is what lets a database migrated ahead of the binary keep its rows: an
// instance running an older build ignores the scope a newer one defines instead
// of refusing to read the role.
//
// [Ja] Scope は、ロールがその保持者に何を許すかを 1 つ名指します。表記は他の Korylus の
// サービスが使う語彙にならった resource:action です。この名前が roles.scopes に保存される
// 値そのものであるため、ロールが何を与えるかは、このファイルが書き下すのと同じ言葉として
// 読み戻されます。
//
// 以下の集合がアプリケーションの知る語彙のすべてです。ロールはここに無い名前を持つことも
// できます (roles.scopes は文字列の JSON 配列なら何でも受け取ります)。そうした名前は何も
// 与えません。それに対する判定がどこにも書かれていないためです。これにより、バイナリより
// 先にマイグレートされたデータベースは行を保てます。古いビルドで動くインスタンスは、
// 新しいビルドが定義するスコープを、ロールの読み取りを拒む代わりに無視します。
type Scope string

const (
	// ScopeCommunityAdmin stands for the whole vocabulary rather than for one
	// operation: it names every scope Scopes returns, including the ones added
	// to that list after a role carrying it was stored. It is what the built-in
	// admin role holds, so a scope added to the vocabulary needs no change to
	// the stored roles.
	//
	// [Ja] ScopeCommunityAdmin は 1 つの操作ではなく語彙の全体を表します。Scopes が返す
	// スコープをすべて名指し、それを持つロールが保存された後にその一覧へ足されたスコープも
	// 名指します。組み込みの admin ロールが持つのはこれであるため、語彙にスコープを足しても
	// 保存済みのロールに手を入れる必要はありません。
	ScopeCommunityAdmin Scope = "community:admin"

	// ScopeUserRead admits reading the list of the community's people in the
	// admin screens.
	//
	// [Ja] ScopeUserRead は、管理画面でコミュニティの利用者の一覧を読むことを許します。
	ScopeUserRead Scope = "user:read"

	// ScopeUserRoleWrite admits granting a role to someone and revoking it.
	//
	// [Ja] ScopeUserRoleWrite は、誰かにロールを付与すること、およびそれを剥奪することを
	// 許します。
	ScopeUserRoleWrite Scope = "user_role:write"
)

// Scopes returns every scope the application defines, which is what
// ScopeCommunityAdmin expands to. Deriving the expansion from this one list is
// what keeps adding a scope a single edit: written down a second time, the two
// lists would drift apart, and a scope present in only one of them would be an
// operation the community's administrator turns out not to be admitted to.
//
// A fresh slice is returned per call so a caller cannot edit the set out from
// under the others.
//
// [Ja] Scopes は、アプリケーションが定義するスコープをすべて返します。これが
// ScopeCommunityAdmin の展開先です。展開をこの 1 つの一覧から導くことが、スコープの追加を
// 1 箇所の編集に保ちます。2 度書き下せば 2 つの一覧は離れていき、片方にしか無いスコープは、
// コミュニティの管理者が許されていないと判明する操作になります。
//
// 呼び出しごとに新しいスライスを返すため、ある呼び出し側が他から見える集合を書き換えて
// しまうことはありません。
func Scopes() []Scope {
	return []Scope{ScopeCommunityAdmin, ScopeUserRead, ScopeUserRoleWrite}
}
