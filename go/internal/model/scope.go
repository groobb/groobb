package model

// Scopeは、ロールがその保持者に何を許すかを1つ名指します。表記は他のKorylusの
// サービスが使う語彙にならったresource:actionです。この名前がroles.scopesに保存される
// 値そのものであるため、ロールが何を与えるかは、このファイルが書き下すのと同じ言葉として
// 読み戻されます。
//
// 以下の集合がアプリケーションの知る語彙のすべてです。ロールはここに無い名前を持つことも
// できます (roles.scopesは文字列のJSON配列なら何でも受け取ります)。そうした名前は何も
// 与えません。それに対する判定がどこにも書かれていないためです。これにより、バイナリより
// 先にマイグレートされたデータベースは行を保てます。古いビルドで動くインスタンスは、
// 新しいビルドが定義するスコープを、ロールの読み取りを拒む代わりに無視します。
type Scope string

const (
	// ScopeCommunityAdminは1つの操作ではなく語彙の全体を表します。Scopesが返す
	// スコープをすべて名指し、それを持つロールが保存された後にその一覧へ足されたスコープも
	// 名指します。組み込みのadminロールが持つのはこれであるため、語彙にスコープを足しても
	// 保存済みのロールに手を入れる必要はありません。
	ScopeCommunityAdmin Scope = "community:admin"

	// ScopeUserReadは、管理画面でコミュニティの利用者の一覧を読むことを許します。
	ScopeUserRead Scope = "user:read"

	// ScopeUserRoleWriteは、誰かにロールを付与すること、およびそれを剥奪することを
	// 許します。
	ScopeUserRoleWrite Scope = "user_role:write"

	// ScopeThreadLockWriteは、スレッドをロックすること、およびそのロックを解除する
	// ことを許します。左側で名指すリソースはスレッドではなくロックです。これにより、印を
	// 付けることと外すことが、ロールの付与と剥奪と同じく1つの境界になります。後で2つに
	// 分けるときは、外す側をthread_lock:deleteとして切り出します。
	ScopeThreadLockWrite Scope = "thread_lock:write"

	// ScopeThreadUnpublicationWriteは、スレッドをコミュニティから見えなくすることを
	// 許します。actionがdeleteではなくwriteであるのは、スレッドの行がそのまま残り
	// 続けるためです。deleteは、左側が名指すものを恒久的に取り去る操作のために
	// 取ってあります。
	ScopeThreadUnpublicationWrite Scope = "thread_unpublication:write"

	// ScopePostUnpublicationWriteは、投稿を1つコミュニティから見えなくすることを
	// 許します。ScopeThreadUnpublicationWriteと別に置いているのは、ロールに2つのうち
	// 狭いほうだけを許せるようにするためです。
	ScopePostUnpublicationWrite Scope = "post_unpublication:write"

	// ScopeUserSuspensionWriteは、利用者を停止すること、およびその停止を解除する
	// ことを許します。停止はアカウントの身元に触れずに活動だけを止めるため、コミュニティ
	// から去る途中の一段ではなく、それ自体が1つの印です。
	ScopeUserSuspensionWrite Scope = "user_suspension:write"

	// ScopeModerationLogReadは、管理画面でモデレーション操作の記録を読むことを
	// 許します。
	ScopeModerationLogRead Scope = "moderation_log:read"
)

// Scopesは、アプリケーションが定義するスコープをすべて返します。これが
// ScopeCommunityAdminの展開先です。展開をこの1つの一覧から導くことが、スコープの追加を
// 1箇所の編集に保ちます。2度書き下せば2つの一覧は離れていき、片方にしか無いスコープは、
// コミュニティの管理者が許されていないと判明する操作になります。
//
// 呼び出しごとに新しいスライスを返すため、ある呼び出し側が他から見える集合を書き換えて
// しまうことはありません。
func Scopes() []Scope {
	return []Scope{
		ScopeCommunityAdmin,
		ScopeUserRead,
		ScopeUserRoleWrite,
		ScopeThreadLockWrite,
		ScopeThreadUnpublicationWrite,
		ScopePostUnpublicationWrite,
		ScopeUserSuspensionWrite,
		ScopeModerationLogRead,
	}
}
