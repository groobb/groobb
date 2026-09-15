package model

import (
	"strconv"
	"time"
)

// PostIntervalは1人が投稿と投稿の間に置く時間です。インスタンスのどこに書かれた
// ものであれ、その人の最後に保存された投稿から測ります。間隔はその人に属するもので、
// 別の掲示板・スレッド・セッションへ書いても新しく始まることはないためです。
//
// これが間隔を空けるのは成功した投稿です。拒否された送信は何も保存しないため、間隔を
// 始めることも、間隔に止められることもありません。インスタンスがいくつの要求に応答するかは、
// 1つのアカウントがどれだけ速くスレッドを埋められるかとは別の問いです。
const PostInterval = 10 * time.Second

// PostIntervalWaitは、最後の投稿がlastPostedAtに保存された人について、nowの
// 時点でPostIntervalのうちどれだけが残っているかを返し、尽きていれば0を返します。
// 間隔はちょうどPostIntervalで終わるため、その時点で届いた投稿は受け付けられます。
//
// 残りを1秒単位に切り上げるのは、それが利用者に伝える待ち時間でありRetry-Afterが運ぶ
// 値でもあるためで、このヘッダーは整数秒しか受け付けません。切り捨てると1.2秒の待ちが
// 1秒として伝わり、再び拒否される再送を招きます。
func PostIntervalWait(lastPostedAt, now time.Time) time.Duration {
	remaining := PostInterval - now.Sub(lastPostedAt)
	if remaining <= 0 {
		return 0
	}

	return (remaining + time.Second - 1).Truncate(time.Second)
}

// Postは人がスレッドに書いたものです。
type Post struct {
	ID PostID

	ThreadID ThreadID

	// UserIDは投稿を書いたアカウントです。アカウントが論理退会している間も値を保ち、
	// その行が物理削除された後にだけnilになります。有効な作者と論理退会済みの作者は、
	// 参照先のユーザーを解決して区別します。いずれの場合も投稿は残るため、それを引用した
	// 返信は文脈を保てます。
	UserID *UserID

	// Numberはスレッド内のレス番号であり、投稿の永久アドレスです。他の本文に
	// 書かれた >>N、アンカーの #p{number}、外部で共有されたURLが、いずれもこれで
	// 解決します。
	Number int

	// Bodyは入力されたテキストそのままで、記法は適用されていません。>>NとURLの
	// リンク化は取り出す側で行うため、記法は描画側の変更だけで後から足せます。
	Body string

	// UnpublishedAtは管理者が投稿を非公開にした時刻で、公開されている間はnilです。
	// 投稿はスレッドに残り、レス番号も保つため、それを引用した返信は変わらず解決し、
	// 後続の番号がずれることもありません。
	UnpublishedAt *time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
}

// ParsePostNumberは、アドレスがレス番号を綴る10進表記からそれを読み取り、そもそも
// rawがそれを表しているかどうかを併せて返します。投稿はスレッドの中でこの番号によって
// 名指されるため (ADR 0009)、投稿1件を名指すルートはここでそれを解決し、同じ投稿を名指す
// どのルートも、何がその投稿のアドレスであるかについて一致します。
//
// 正の整数でないものはルックアップせずに拒否します。スレッドは投稿に1から番号を振るため、
// そのような番号を持つ投稿は無く、ルックアップしてもクエリを1回発行した末に不在と答える
// だけだからです。strconvが読み取る数の周りに認める綴りは、スレッドのidと同じくここでも
// 受け付けます。番号から描かれるものは、辿られたパスではなく読み取った値から組み立てる
// ためです。
func ParsePostNumber(raw string) (int, bool) {
	number, err := strconv.Atoi(raw)
	if err != nil || number <= 0 {
		return 0, false
	}

	return number, true
}
