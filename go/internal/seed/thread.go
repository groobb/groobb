package seed

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/sqlitetime"
)

// contentPlan says how much a run generates. The amounts sit together in one
// place because they are the one thing about the content that changes with who
// is asking: a developer wants a board heavy enough to be worth paging through,
// while a test wants the same shapes in a few rows.
//
// [Ja] contentPlan は実行がどれだけ生成するのかを述べます。件数を 1 箇所にまとめて
// いるのは、生成する中身のうち、求める側によって変わるのがここだけだからです。開発者は
// 捲る価値のある重さの掲示板を求め、テストは同じ形をわずかな行数で求めます。
type contentPlan struct {
	busyBoardThreads  int
	quietBoardThreads int
	minPostsPerThread int
	maxPostsPerThread int

	// span is how far back the oldest thread was last posted in, which the
	// threads are spread evenly over. It belongs to the plan because it says how
	// long the community has been going: the same three threads read as a board
	// that opened this week or as one nobody has written in for a fortnight,
	// depending only on this (ADR 0010).
	//
	// [Ja] span は、最も古いスレッドが最後に投稿された時点がどれだけ前かで、スレッドは
	// この幅に均等に散らばります。これが plan にあるのは、コミュニティがどれだけ続いて
	// きたのかを述べる値だからです。同じ 3 本のスレッドが、今週開いた掲示板に読めるか、
	// 2 週間誰も書いていない掲示板に読めるかは、ここだけで決まります (ADR 0010)。
	span time.Duration

	// fullThreadPosts is how many posts the thread that has reached the limit
	// holds, and zero means the community has no such thread at all. A thread
	// that has been written in a thousand times is something a community
	// accumulates, so the state it is looked at in has to be one a profile can
	// leave out.
	//
	// [Ja] fullThreadPosts は、上限に達したスレッドが持つ投稿の数で、0 はそうした
	// スレッドがコミュニティに 1 つも無いことを表します。千回書き込まれたスレッドは
	// コミュニティが蓄積するものであり、それを眺める状態は、プロファイルが省ける形で
	// なければなりません。
	fullThreadPosts int
}

// matureContentPlan is how much the mature profile generates. The busy board
// holds more threads than a page of a thread list will (M4), and the full thread
// holds what a thread holds at most, because both of those screens can only be
// looked at at those sizes.
//
// [Ja] matureContentPlan は mature プロファイルが生成する量です。賑わう掲示板は
// スレッド一覧の 1 ページ (M4) に収まらない数のスレッドを持ち、埋まったスレッドは
// スレッドが持てる最大の数の投稿を持ちます。どちらの画面も、その大きさでしか眺められない
// ためです。
var matureContentPlan = contentPlan{
	busyBoardThreads:  30,
	quietBoardThreads: 4,
	minPostsPerThread: 1,
	maxPostsPerThread: 14,
	span:              30 * 24 * time.Hour,
	fullThreadPosts:   model.ThreadPostLimit,
}

// coldStartContentPlan is how much a community holds in its first days: a few
// threads, a few posts in each, and nothing that took months to accumulate.
// The smallest thread is left at a single post, because a thread nobody has
// answered yet is the ordinary case on a board that has just opened, and both
// the reply count and the back references under a post read differently when
// there is only the one.
//
// [Ja] coldStartContentPlan は、立ち上げ直後のコミュニティが持つ量です。スレッドは数本、
// 各スレッドの投稿も数件で、何ヶ月もかけて積み上がるものは何も含みません。最小のスレッドを
// 投稿 1 件のままにしているのは、まだ誰も答えていないスレッドが、開いたばかりの掲示板では
// ありふれた状態だからです。レス数も、投稿の下に付く逆参照も、1 件しかないときには違う
// 読まれ方をします。
var coldStartContentPlan = contentPlan{
	quietBoardThreads: 3,
	minPostsPerThread: 1,
	maxPostsPerThread: 4,
	span:              3 * 24 * time.Hour,
	fullThreadPosts:   0,
}

// threadCount returns how many ordinary threads a board of this activity gets.
//
// [Ja] threadCount は、この賑わいの掲示板がいくつの通常のスレッドを得るのかを返します。
func (p contentPlan) threadCount(activity boardActivity) int {
	switch activity {
	case boardBusy:
		return p.busyBoardThreads
	case boardQuiet:
		return p.quietBoardThreads
	case boardEmpty:
		return 0
	default:
		return 0
	}
}

// contentSeed fixes the pseudo-random choices a generation makes. The same code
// and the same roster therefore produce the same rows in the same order, and a
// thread a developer opened yesterday is behind the same address today. A run
// that shuffled its ids would make every note taken while looking at a screen
// worthless the next morning.
//
// [Ja] contentSeed は生成が行う疑似乱数の選択を固定します。同じコードと同じ名簿からは
// 同じ行が同じ順序で生まれ、開発者が昨日開いたスレッドは今日も同じアドレスの先にあります。
// 実行のたびに id が入れ替われば、画面を見ながら取ったメモは翌朝には使えなくなります。
const contentSeed = 20260825

// postInterval is how far apart two posts of one thread are written. A run that
// stamped everything with the moment it ran would leave a thread whose posts all
// read the same, and a thread list ordered by a column every row shares.
//
// [Ja] postInterval は、1 つのスレッドの投稿どうしがどれだけ離れて書かれるかです。実行
// した瞬間の時刻をすべてに押した実行は、どの投稿も同じに読めるスレッドと、どの行も同じ値を
// 持つ列で並んだスレッド一覧を残します。
const postInterval = 11 * time.Minute

// scriptedThread is a thread written out post by post, and scriptedPost is one
// of those posts. A thread whose point is what the words do — which post answers
// which, a body that carries a URL, a body that looks like markup — cannot be
// assembled out of interchangeable sentences.
//
// [Ja] scriptedThread は投稿ごとに書き下したスレッド、scriptedPost はその投稿 1 件です。
// 見どころが言葉の働きそのものにあるスレッド (どの投稿がどれに答えるか、URL を含む本文、
// マークアップに見える本文) は、取り替えのきく文を並べても組み立てられません。
type scriptedThread struct {
	title string
	posts []scriptedPost
}

type scriptedPost struct {
	role seedRole
	body string
}

// referenceScript is the thread reply references are looked at in. It holds a
// post more than one later post answers (so the back references under it are a
// list), a post that answers two at once, the same number written twice (which
// is one reference, not two), and a number no post carries (which is text).
//
// The bodies a URL and something that looks like markup are written in are here
// too: both are decided while rendering one post's body, so the thread that
// shows what a body can hold is where they belong.
//
// [Ja] referenceScript はレス参照を眺めるためのスレッドです。後続の複数の投稿が答える
// 投稿 (その下に付く逆参照が一覧になります)、2 つの投稿にまとめて答える投稿、同じ番号を
// 2 度書いた本文 (参照は 2 つではなく 1 つです)、どの投稿も持たない番号 (テキストです) を
// 持ちます。
//
// URL を含む本文と、マークアップに見える本文もここに置いています。どちらも 1 つの投稿の
// 本文を描画するときに決まるものであり、本文が何を持てるのかを見せるスレッドがその
// 置き場所になります。
var referenceScript = scriptedThread{
	title: "レス参照の見え方を確かめるスレ",
	posts: []scriptedPost{
		{role: roleStarter, body: "1 つ目の投稿です。ここに返信が集まると、この投稿の下に逆参照が並びます。"},
		{role: roleReplier, body: ">>1 まずは 1 つ目の返信です。"},
		{role: roleStarter, body: ">>1 2 つ目の返信です。これで 1 つ目の投稿には逆参照が 2 つ付きます。"},
		{role: roleReplier, body: ">>2 >>3 1 つの投稿から 2 つの投稿を指すこともできます。"},
		{role: roleStarter, body: ">>1 >>1 同じ番号を 2 度書いても、参照として残るのは 1 つだけです。"},
		{role: roleReplier, body: ">>999 まだ誰も書いていない番号は、リンクにならずそのまま表示されます。"},
		{role: roleStarter, body: "https://example.com/help のように書いた URL はリンクになります。"},
		{role: roleReplier, body: "<b>タグに見える入力</b> や & のような記号も、書いたとおりに表示されます。"},
		{role: roleStarter, body: "改行を含む本文です。\n2 行目はここから始まります。\n\n空行をはさむこともできます。"},
	},
}

// withdrawnScript is the thread a post is read in without its author. The
// account that opens it withdraws at the end of the run, so the thread and two
// of its posts lose their author while the replies that answer them stay where
// they are: a withdrawal takes the name off what was written, not the writing.
//
// [Ja] withdrawnScript は、投稿を作者抜きで読むためのスレッドです。これを立てた
// アカウントは実行の最後に退会するため、スレッドとその投稿 2 件が作者を失う一方、それらに
// 答えた返信はその場に残ります。退会が外すのは書かれたものから名前であって、書かれたもの
// そのものではありません。
var withdrawnScript = scriptedThread{
	title: "退会した人の投稿が残っているスレ",
	posts: []scriptedPost{
		{role: roleWithdrawn, body: "このスレッドを立てたアカウントは、このあと退会します。"},
		{role: roleReplier, body: ">>1 立てた人がいなくなっても、スレッドと投稿はここに残ります。"},
		{role: roleWithdrawn, body: ">>2 返信の宛先も残るので、会話としてはそのまま読めるはずです。"},
		{role: roleStarter, body: ">>1 >>3 作者のいない投稿がどう見えるかは、この 2 つで確かめられます。"},
	},
}

// fullThreadTitle names the thread that has reached the post limit. It says the
// limit rather than the number, because the number comes from the plan and a
// test fills the thread with far fewer posts.
//
// [Ja] fullThreadTitle は投稿数の上限に達したスレッドの題名です。数ではなく上限と
// 述べているのは、数が contentPlan から来るもので、テストはずっと少ない投稿でこの
// スレッドを埋めるためです。
const fullThreadTitle = "上限まで埋まっていて書き込めないスレッド"

// threadTitles, openingBodies and replyBodies are what the ordinary threads are
// written from. They are Japanese because the screens are read in Japanese, and
// what a board of Japanese titles looks like is not something a board of English
// ones can be checked for: the two wrap and truncate at different widths.
//
// [Ja] threadTitles・openingBodies・replyBodies は、通常のスレッドを書き起こす材料です。
// 日本語なのは画面が日本語で読まれるためで、日本語の題名が並ぶ掲示板の見え方は、英語の
// 題名が並ぶ掲示板では確かめられません。両者は折り返す幅も切り詰まる幅も異なります。
var threadTitles = []string{
	"はじめまして、自己紹介をどうぞ",
	"今日あった小さないいことを書くスレ",
	"作業中に流している音を教えてほしい",
	"最近読んだ本の話をしませんか",
	"この掲示板の使い方を確かめるスレ",
	"深夜にやっているゲームの話",
	"おすすめのキーボードはありますか",
	"雨の日の過ごし方を共有する",
	"引っ越し先で困っていること",
	"お昼に何を食べたか報告するスレ",
	"うまくいかなかった料理の記録",
	"散歩コースを教え合いませんか",
	"連休の予定をここで立てる",
	"買ってよかったものを挙げていく",
	"寝る前にやめられない習慣",
	"来月までにやりたいこと",
}

var openingBodies = []string{
	"とりあえず立ててみました。思いついたことを気軽にどうぞ。",
	"前から気になっていたので、みなさんの話を聞かせてください。",
	"同じことを考えている人がいそうなので、スレッドにしておきます。",
	"うまく言葉にできていませんが、書きながら整理してみます。",
	"先週から続けていることについて、途中経過を書いておきます。",
	"結論は出ていません。似た経験のある方がいたら教えてください。",
}

var replyBodies = []string{
	"わかります。自分もちょうど同じことを考えていました。",
	"なるほど、その手がありましたか。今度試してみます。",
	"うちは逆のやり方でした。環境によって変わりそうですね。",
	"詳しく書いてもらえて助かりました。ありがとうございます。",
	"しばらく続けてみて、また結果を書きにきます。",
	"それは知りませんでした。調べてみたら確かにそうなっていますね。",
	"自分の場合はうまくいかなかったので、条件が違うのかもしれません。",
}

// plannedThread is a thread with its posts as they will be written, and
// plannedPost is one of those posts. A thread is composed in full before any of
// it is written so that the two steps stay separable: composing is where the
// wording and the randomness are, writing is where the ids and the times are.
//
// [Ja] plannedThread は書き込まれる形になったスレッドとその投稿、plannedPost はその
// 投稿 1 件です。スレッドを 1 つも書き込む前に丸ごと組み立てるのは、2 つの工程を分けて
// おくためです。文面と乱数は組み立てにあり、id と時刻は書き込みにあります。
type plannedThread struct {
	board *model.Board
	title string
	posts []plannedPost
}

type plannedPost struct {
	author *model.User
	body   string
}

// contentGenerator composes the community's conversations and writes them.
//
// [Ja] contentGenerator はコミュニティの会話を組み立て、書き込みます。
type contentGenerator struct {
	plan          contentPlan
	scripts       []scriptedThread
	rng           *rand.Rand
	users         *seededUsers
	threadRepo    *repository.ThreadRepository
	postRepo      *repository.PostRepository
	referenceRepo *repository.PostReferenceRepository
}

// generateThreads creates the threads, the posts they hold and the references
// between those posts.
//
// [Ja] generateThreads はスレッドと、それが持つ投稿、そして投稿どうしの参照を作成
// します。
func (r *Runner) generateThreads(ctx context.Context, tx *sql.Tx, st *state) error {
	g := &contentGenerator{
		plan:    r.profile.plan,
		scripts: r.profile.scripts,
		// The randomness picks which sentence a post is written from, so a
		// predictable sequence is what is wanted here rather than a defect: a
		// run has to be reproducible (see contentSeed). Nothing this generator
		// decides is a secret, so gosec's warning about a non-cryptographic
		// source does not apply.
		//
		// [Ja] ここでの乱数が選ぶのは、投稿がどの文から書かれるかです。そのため予測
		// できる系列であることは欠陥ではなく、ここで求めているものです。実行は再現できる
		// 必要があります (contentSeed を参照)。この生成器が決めるものに秘密は無いため、
		// 暗号論的でない乱数源に対する gosec の指摘はここでは当たりません。
		//nolint:gosec // G404
		rng:           rand.New(rand.NewPCG(contentSeed, contentSeed)),
		users:         st.users,
		threadRepo:    repository.NewThreadRepository(r.db).WithTx(tx),
		postRepo:      repository.NewPostRepository(r.db).WithTx(tx),
		referenceRepo: repository.NewPostReferenceRepository(r.db).WithTx(tx),
	}

	threads, err := g.composeThreads(st.boards)
	if err != nil {
		return err
	}

	bar := newProgress(r.out, "threads", len(threads))
	defer bar.finish()

	// The threads are written oldest first, so that the order they were created
	// in and the order they were last posted in agree. A thread list breaks a
	// tie on the timestamp with the id, and the two disagreeing would put a
	// thread above one posted in later.
	//
	// [Ja] スレッドは古いものから書き込みます。作成された順と最後に投稿された順が
	// 一致するようにするためです。スレッド一覧は時刻の同着を id で解くため、両者が
	// 食い違うと、後から投稿されたスレッドより上に並ぶスレッドが生まれます。
	now := time.Now()
	span := g.plan.span
	interval := threadInterval(span, len(threads))
	for i, thread := range threads {
		lastPostedAt := now.Add(-span + interval*time.Duration(i+1))
		if err := g.writeThread(ctx, tx, thread, lastPostedAt); err != nil {
			return err
		}
		bar.advance()
	}

	return nil
}

// threadInterval returns how far apart the last posts of two consecutive threads
// are placed, so that the threads fill the span however many of them there are.
//
// [Ja] threadInterval は、連続する 2 つのスレッドの最終投稿をどれだけ離して置くのかを
// 返します。スレッドの数がいくつであっても、それらが span を埋めるようにするためです。
func threadInterval(span time.Duration, threadCount int) time.Duration {
	if threadCount == 0 {
		return 0
	}

	return span / time.Duration(threadCount)
}

// composeThreads works out every thread a run writes: the ordinary ones each
// board gets, then the ones written to be opened. The written ones come last so
// that they are the most recently posted in, which is where a thread list puts
// them within reach.
//
// [Ja] composeThreads は実行が書き込むスレッドをすべて組み立てます。各掲示板が得る
// 通常のスレッドに続けて、開いて眺めるために書き下したスレッドを置きます。書き下した
// ものを最後にするのは、それらを最後に投稿されたスレッドにするためで、そうすることで
// スレッド一覧はそれらを手の届く位置に置きます。
func (g *contentGenerator) composeThreads(boards []seededBoard) ([]plannedThread, error) {
	starter, err := g.account(roleStarter)
	if err != nil {
		return nil, err
	}
	replier, err := g.account(roleReplier)
	if err != nil {
		return nil, err
	}
	speakers := [2]*model.User{starter, replier}

	var threads []plannedThread
	for _, board := range boards {
		for i := range g.plan.threadCount(board.activity) {
			threads = append(threads, g.composeOrdinaryThread(board.board, i, speakers))
		}
	}

	// Everything written out goes in the busy board, so the board is only looked
	// for once there is something to post in it. A community that has just opened
	// has neither.
	//
	// [Ja] 書き下したものはすべて賑わう掲示板に立つため、そこへ投稿するものがあるとき
	// にだけ掲示板を探します。開いたばかりのコミュニティは、そのどちらも持ちません。
	hasFullThread := g.plan.fullThreadPosts > 0
	if len(g.scripts) == 0 && !hasFullThread {
		return threads, nil
	}

	busy, err := busyBoard(boards)
	if err != nil {
		return nil, err
	}

	for _, script := range g.scripts {
		thread, err := g.composeScriptedThread(busy, script)
		if err != nil {
			return nil, err
		}
		threads = append(threads, thread)
	}

	if hasFullThread {
		threads = append(threads, g.composeFullThread(busy, speakers))
	}

	return threads, nil
}

// composeOrdinaryThread writes one of the threads a board is filled with. Which
// of the two speakers opens it alternates, so that a board list is not a column
// of one atname.
//
// [Ja] composeOrdinaryThread は、掲示板を埋めるスレッドの 1 つを組み立てます。2 人の
// うちどちらが立てるのかは交互に入れ替わります。スレッド一覧が 1 つの atname の列に
// ならないようにするためです。
func (g *contentGenerator) composeOrdinaryThread(board *model.Board, index int, speakers [2]*model.User) plannedThread {
	title := threadTitles[index%len(threadTitles)]
	if round := index / len(threadTitles); round > 0 {
		title = fmt.Sprintf("%s その%d", title, round+1)
	}

	count := g.plan.minPostsPerThread + g.rng.IntN(g.plan.maxPostsPerThread-g.plan.minPostsPerThread+1)
	posts := make([]plannedPost, 0, count)
	for i := range count {
		posts = append(posts, plannedPost{
			author: speakers[(index+i)%len(speakers)],
			body:   g.composeBody(i + 1),
		})
	}

	return plannedThread{board: board, title: title, posts: posts}
}

// composeFullThread writes the thread that has reached the post limit. Its
// first post says so, because the number of posts is the only other thing that
// says it and nobody counts to a thousand.
//
// [Ja] composeFullThread は投稿数の上限に達したスレッドを組み立てます。最初の投稿が
// そう述べているのは、それ以外にそう述べるものが投稿の数しか無く、1000 まで数える人は
// いないためです。
func (g *contentGenerator) composeFullThread(board *model.Board, speakers [2]*model.User) plannedThread {
	posts := make([]plannedPost, 0, g.plan.fullThreadPosts)
	for i := range g.plan.fullThreadPosts {
		body := g.composeBody(i + 1)
		if i == 0 {
			body = "上限まで書き込まれたスレッドです。ここには続きを書き込めません。"
		}

		posts = append(posts, plannedPost{author: speakers[i%len(speakers)], body: body})
	}

	return plannedThread{board: board, title: fullThreadTitle, posts: posts}
}

// composeScriptedThread turns a script into a thread, resolving the account each
// post is attributed to from the role the script names it by.
//
// [Ja] composeScriptedThread は台本をスレッドにします。各投稿が誰のものになるのかは、
// 台本がそれを名指しする役割から解決します。
func (g *contentGenerator) composeScriptedThread(board *model.Board, script scriptedThread) (plannedThread, error) {
	posts := make([]plannedPost, 0, len(script.posts))
	for _, scripted := range script.posts {
		author, err := g.account(scripted.role)
		if err != nil {
			return plannedThread{}, err
		}

		posts = append(posts, plannedPost{author: author, body: scripted.body})
	}

	return plannedThread{board: board, title: script.title, posts: posts}, nil
}

// composeBody writes the body of the post that will carry the given reply
// number. A reply quotes one of the posts above it often enough that a thread
// reads as an exchange, and not so often that every post is a quotation.
//
// [Ja] composeBody は、指定のレス番号を持つことになる投稿の本文を書きます。返信が
// 上の投稿を引用するのは、スレッドがやり取りとして読める程度に多く、どの投稿も引用で
// 始まってしまう程には多くない頻度です。
func (g *contentGenerator) composeBody(number int) string {
	if number == 1 {
		return openingBodies[g.rng.IntN(len(openingBodies))]
	}

	body := replyBodies[g.rng.IntN(len(replyBodies))]
	if g.rng.IntN(3) == 0 {
		return body
	}

	return fmt.Sprintf(">>%d %s", 1+g.rng.IntN(number-1), body)
}

// account returns the account created for the role. A role with no account is
// an error rather than a post with no author: an absent author is what a
// withdrawal leaves behind, and a generator must not arrive at that state by
// dropping something.
//
// [Ja] account はその役割で作成されたアカウントを返します。アカウントの無い役割を、
// 作者のいない投稿にせずエラーにするのは、作者が不在であることが退会の結果だからです。
// 生成器が何かを取りこぼしてその状態へ辿り着いてはなりません。
func (g *contentGenerator) account(role seedRole) (*model.User, error) {
	user := g.users.user(role)
	if user == nil {
		return nil, fmt.Errorf("no account was created for the role %s", role)
	}

	return user, nil
}

// busyBoard returns the board the threads written to be opened are posted in.
//
// [Ja] busyBoard は、開いて眺めるために書き下したスレッドが立つ掲示板を返します。
func busyBoard(boards []seededBoard) (*model.Board, error) {
	for _, board := range boards {
		if board.activity == boardBusy {
			return board.board, nil
		}
	}

	return nil, fmt.Errorf("no board was created to hold the threads that are written out")
}

// writeThread writes a composed thread: the thread, its posts in reply-number
// order, the references their bodies make, and the thread's denormalized view of
// the posts it ended up with.
//
// [Ja] writeThread は組み立て済みのスレッドを書き込みます。スレッド、レス番号順の投稿、
// それらの本文が作る参照、そして結果として持つことになった投稿についてスレッドが持つ
// 非正規化された姿です。
func (g *contentGenerator) writeThread(ctx context.Context, tx *sql.Tx, planned plannedThread, lastPostedAt time.Time) error {
	thread, err := g.threadRepo.Create(ctx, repository.CreateThreadInput{
		BoardID: planned.board.ID,
		UserID:  &planned.posts[0].author.ID,
		Title:   planned.title,
	})
	if err != nil {
		return fmt.Errorf("failed to create the thread %q: %w", planned.title, err)
	}

	posts := make(map[int]*model.Post, len(planned.posts))
	var firstPostedAt time.Time
	var lastPost *model.Post

	for i, plannedPost := range planned.posts {
		number := i + 1
		postedAt := lastPostedAt.Add(-postInterval * time.Duration(len(planned.posts)-number))

		post, err := g.postRepo.Create(ctx, repository.CreatePostInput{
			ThreadID: thread.ID,
			UserID:   &plannedPost.author.ID,
			Number:   number,
			Body:     plannedPost.body,
		})
		if err != nil {
			return fmt.Errorf("failed to create the post %d of the thread %q: %w", number, planned.title, err)
		}
		if err := backdate(ctx, tx, backdatePostStatement, int64(post.ID), postedAt, postedAt); err != nil {
			return fmt.Errorf("failed to backdate the post %d of the thread %q: %w", number, planned.title, err)
		}

		// The references are written before the post joins the ones a number can
		// resolve to, so that a body quoting its own number resolves to nothing:
		// a post answers the posts above it, never itself.
		//
		// [Ja] 参照は、この投稿が番号の解決先に加わる前に書き込みます。自分自身の番号を
		// 引用した本文が何も解決しないようにするためです。投稿が答える相手は上にある
		// 投稿であって、自分自身ではありません。
		if err := g.writeReferences(ctx, post, posts); err != nil {
			return err
		}

		posts[number] = post
		if number == 1 {
			firstPostedAt = postedAt
		}
		lastPost = post
	}

	if err := g.threadRepo.UpdateLastPost(ctx, thread.ID, repository.UpdateThreadLastPostInput{
		PostsCount:   len(planned.posts),
		LastPostID:   lastPost.ID,
		LastPostedAt: lastPostedAt,
	}); err != nil {
		return fmt.Errorf("failed to update the last post of the thread %q: %w", planned.title, err)
	}

	// A thread begins when its first post was written and last changed when its
	// latest one was, which is the same pair of moments the posts themselves
	// carry.
	//
	// [Ja] スレッドが始まるのは最初の投稿が書かれた時点、最後に変わったのは最新の投稿が
	// 書かれた時点です。これは投稿自身が持つのと同じ 2 つの時刻です。
	if err := backdate(ctx, tx, backdateThreadStatement, int64(thread.ID), firstPostedAt, lastPostedAt); err != nil {
		return fmt.Errorf("failed to backdate the thread %q: %w", planned.title, err)
	}

	return nil
}

// writeReferences records the posts the body refers to, out of the posts the
// thread carries so far. A number none of them has is left alone: a >>N pointing
// past what was written is text, and stays text.
//
// [Ja] writeReferences は、本文が参照する投稿を、そのスレッドがそこまでに持っている
// 投稿の中から記録します。どの投稿も持たない番号はそのままにします。書かれたものの先を
// 指す >>N はテキストであり、テキストのままです。
func (g *contentGenerator) writeReferences(ctx context.Context, post *model.Post, posts map[int]*model.Post) error {
	for _, number := range model.ReferencedPostNumbers(post.Body) {
		referenced, ok := posts[number]
		if !ok {
			continue
		}

		if _, err := g.referenceRepo.Create(ctx, repository.CreatePostReferenceInput{
			PostID:           post.ID,
			ReferencedPostID: referenced.ID,
		}); err != nil {
			return fmt.Errorf("failed to create the reference from the post %d to the post %d: %w", post.Number, number, err)
		}
	}

	return nil
}

// backdatePostStatement and backdateThreadStatement move a row the application
// wrote back to the moment the seed's conversation places it at.
//
// The application stamps a row with the moment it is written, which is right for
// the application and wrong for sample data: a board whose threads were all
// posted in the same second says nothing about how a thread list is ordered, or
// about how a time reads once it is a week old. The seed therefore writes its
// rows the way the application does and then moves them, rather than teaching
// the Infrastructure layer to accept a timestamp only the seed would pass.
//
// The references between posts keep the moment they were written, because
// nothing reads their timestamps: a reference is looked up by the post it points
// at and ordered by the ids of the two posts.
//
// [Ja] backdatePostStatement と backdateThreadStatement は、アプリケーションが書いた行を、
// シードの会話がそれを置いている時点まで戻します。
//
// アプリケーションは行に、それを書いた瞬間の時刻を押します。これはアプリケーションにとって
// 正しく、サンプルデータにとっては誤りです。すべてのスレッドが同じ 1 秒の中に投稿された
// 掲示板は、スレッド一覧がどう並ぶのかについても、1 週間前の時刻がどう読めるのかについても
// 何も語りません。そのためシードは、行をアプリケーションと同じやり方で書いてから動かします。
// シードしか渡さない時刻を受け取る術を Infrastructure 層へ教えることはしません。
//
// 投稿どうしの参照は書き込まれた瞬間の時刻を保ちます。その時刻を読むものが無いためです。
// 参照は指し先の投稿で引かれ、2 つの投稿の id で並びます。
const (
	backdatePostStatement   = "UPDATE posts SET created_at = ?, updated_at = ? WHERE id = ?"
	backdateThreadStatement = "UPDATE threads SET created_at = ?, updated_at = ? WHERE id = ?"
)

// backdate runs one of the statements above against the row with the given id.
//
// [Ja] backdate は、上の文のいずれかを、指定の id の行に対して実行します。
func backdate(ctx context.Context, tx *sql.Tx, statement string, id int64, createdAt, updatedAt time.Time) error {
	_, err := tx.ExecContext(ctx, statement, sqlitetime.Time(createdAt), sqlitetime.Time(updatedAt), id)

	return err
}
