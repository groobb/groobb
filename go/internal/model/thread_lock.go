package model

// ThreadLockReason names a condition under which a thread takes no further post.
// Reading is never stopped by one: a locked thread is still read, quoted and
// linked to.
//
// A reason belongs to the thread and holds for everyone at once. Whether someone
// is signed in, may write here, or has waited out the interval between their own
// posts decides a single submission, and a thread that refuses one person while
// taking the next person's post is not locked.
//
// [Ja] ThreadLockReason は、スレッドがそれ以上の投稿を受け付けなくなる条件を名指します。
// 閲覧が止まることはありません。ロックされたスレッドも、読まれ、引用され、リンクされます。
//
// 理由はスレッドに属し、全員に対して同時に成立します。サインインしているか、ここへ
// 書いてよいか、自分の投稿と投稿の間隔を空け終えたかは、1 つの送信を決めるものです。
// ある人の投稿を拒みながら次の人の投稿は受け付けるスレッドは、ロックされていません。
type ThreadLockReason string

// ThreadLockReasonPostLimitReached says the thread holds every post it can hold
// (ADR 0009). It is the reason a thread reaches by being written to, with nobody
// acting on it, and the cap it names is what lets a reply number be a permanent
// address under a single unpaginated URL.
//
// [Ja] ThreadLockReasonPostLimitReached は、スレッドが持てる投稿をすべて持っている
// ことを表します (ADR 0009)。誰かが操作した結果ではなく、書き込まれることによって
// 到達する理由であり、これが名指す上限があるからこそ、レス番号はページ分割されない
// 1 つの URL のもとで永久アドレスでいられます。
const ThreadLockReasonPostLimitReached ThreadLockReason = "post_limit_reached"

// ThreadLockReasons returns every reason a thread can be locked for. It is the
// one place the set is written out, and it exists so that what a locked thread
// says can be checked against it: the end of a locked thread carries neither the
// reply form nor the way into an account, so a reason introduced without wording
// of its own would leave a visitor with nothing standing there and nothing said.
//
// A fresh slice is returned per call so a caller cannot edit the set out from
// under the others.
//
// [Ja] ThreadLockReasons は、スレッドがロックされうる理由をすべて返します。値域を
// 書き下す唯一の場所であり、ロック中のスレッドが述べることをこれと突き合わせられる
// ようにするためにあります。ロック中のスレッドの末尾は返信フォームもアカウントへの
// 導線も持たないため、自身の文言を伴わずに導入された理由は、訪問者にそこで何も立って
// いない状態と、何も述べられていない状態を残します。
//
// 呼び出しごとに新しいスライスを返すため、ある呼び出し側が他から見える集合を書き換えて
// しまうことはありません。
func ThreadLockReasons() []ThreadLockReason {
	return []ThreadLockReason{ThreadLockReasonPostLimitReached}
}

// LockReasons returns the lock reasons derived from t's current PostsCount,
// or an empty slice if it is unlocked. It does not modify t. Callers deciding
// whether to accept a post must call this method on a model re-read inside
// their write transaction.
//
// The answer is a list because reasons hold alongside one another rather than
// replace one another. Kept as a single value, the reason that arrived last
// would hide the ones still holding, and clearing it would reopen a thread that
// another reason still closes.
//
// Reaching the cap is derived from the count the thread already carries rather
// than stored, which leaves no column to keep in step and nothing to run on a
// schedule. The thread turns locked in the very commit that saves its last post,
// and a submission that rolls back leaves behind no thread claiming to be full.
// A count that has passed the cap reads as locked as well, since what the reason
// says is that there is no room left, not that the number landed exactly on it.
//
// [Ja] LockReasons は、tの現在のPostsCountからロック理由を返し、ロックされていなければ
// 空のスライスを返します。tを変更しません。投稿の可否を判断する呼び出し元は、
// 書き込みトランザクション内で読み直したモデルに対して、このメソッドを呼び出す必要が
// あります。
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

	if t.PostsCount >= ThreadPostLimit {
		reasons = append(reasons, ThreadLockReasonPostLimitReached)
	}

	return reasons
}
