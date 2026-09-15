package model

// ThreadLockReasonは、スレッドがそれ以上の投稿を受け付けなくなる条件を名指します。
// 閲覧が止まることはありません。ロックされたスレッドも、読まれ、引用され、リンクされます。
//
// 理由はスレッドに属し、全員に対して同時に成立します。サインインしているか、ここへ
// 書いてよいか、自分の投稿と投稿の間隔を空け終えたかは、1つの送信を決めるものです。
// ある人の投稿を拒みながら次の人の投稿は受け付けるスレッドは、ロックされていません。
type ThreadLockReason string

// ThreadLockReasonPostLimitReachedは、スレッドが持てる投稿をすべて持っている
// ことを表します (ADR 0009)。誰かが操作した結果ではなく、書き込まれることによって
// 到達する理由であり、これが名指す上限があるからこそ、レス番号はページ分割されない
// 1つのURLのもとで永久アドレスでいられます。
const ThreadLockReasonPostLimitReached ThreadLockReason = "post_limit_reached"

// ThreadLockReasonLockedByModeratorは、管理者がスレッドを閉じたことを表します。
// 上限と違い、書き込まれることで到達した状態ではなく判断であるため、これを持つ
// スレッドには次のスレッドへの導線を差し出しません。新しいスレッドを立てることは、
// その判断を迂回することになるためです。
const ThreadLockReasonLockedByModerator ThreadLockReason = "locked_by_moderator"

// ThreadLockReasonsは、スレッドがロックされうる理由をすべて返します。値域を
// 書き下す唯一の場所であり、ロック中のスレッドが述べることをこれと突き合わせられる
// ようにするためにあります。ロック中のスレッドの末尾は返信フォームもアカウントへの
// 導線も持たないため、自身の文言を伴わずに導入された理由は、訪問者にそこで何も立って
// いない状態と、何も述べられていない状態を残します。
//
// 呼び出しごとに新しいスライスを返すため、ある呼び出し側が他から見える集合を書き換えて
// しまうことはありません。
func ThreadLockReasons() []ThreadLockReason {
	return []ThreadLockReason{ThreadLockReasonLockedByModerator, ThreadLockReasonPostLimitReached}
}

// LockReasonsは、tが現在持つロック理由を返し、ロックされていなければ空のスライスを
// 返します。tを変更しません。投稿の可否を判断する呼び出し元は、書き込みトランザクション内で
// 読み直したモデルに対して、このメソッドを呼び出す必要があります。
//
// 管理者によるロックを先に置くのは、それが訪問者に対して答えとなる理由であるためです。
// こちらはこのスレッドについての判断であり、上限はスレッドが自ら到達した状態です。
//
// 答えが一覧なのは、理由が互いを置き換えるのではなく並び立つためです。単一の値として
// 持つと、最後に成立した理由が、成立し続けている他の理由を隠します。そしてその値を
// 消すことは、別の理由がなお閉じているスレッドを開けてしまいます。
//
// 上限到達をスレッドが既に持つ件数から導き、保存しないことで、歩調を合わせ続ける列も、
// 定期的に走らせるものも要らなくなります。スレッドは最後の投稿を保存するそのコミットで
// ロック中になり、ロールバックした送信は、満杯を名乗るスレッドを後に残しません。上限を
// 越えた件数もロック中と読みます。理由が述べているのは空きが無いことであって、件数が
// ちょうど上限に載ったことではないためです。
func (t *Thread) LockReasons() []ThreadLockReason {
	var reasons []ThreadLockReason

	if t.LockedAt != nil {
		reasons = append(reasons, ThreadLockReasonLockedByModerator)
	}

	if t.PostsCount >= ThreadPostLimit {
		reasons = append(reasons, ThreadLockReasonPostLimitReached)
	}

	return reasons
}
