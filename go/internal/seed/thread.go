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

// contentPlanは実行がどれだけ生成するのかを述べます。件数を1箇所にまとめて
// いるのは、生成する中身のうち、求める側によって変わるのがここだけだからです。開発者は
// 捲る価値のある重さの掲示板を求め、テストは同じ形をわずかな行数で求めます。
type contentPlan struct {
	busyBoardThreads  int
	quietBoardThreads int
	minPostsPerThread int
	maxPostsPerThread int

	// spanは、最も古いスレッドが最後に投稿された時点がどれだけ前かで、スレッドは
	// この幅に均等に散らばります。これがplanにあるのは、コミュニティがどれだけ続いて
	// きたのかを述べる値だからです。同じ3本のスレッドが、今週開いた掲示板に読めるか、
	// 2週間誰も書いていない掲示板に読めるかは、ここだけで決まります (ADR 0010)。
	span time.Duration

	// fullThreadPostsは、上限に達したスレッドが持つ投稿の数で、0はそうした
	// スレッドがコミュニティに1つも無いことを表します。千回書き込まれたスレッドは
	// コミュニティが蓄積するものであり、それを眺める状態は、プロファイルが省ける形で
	// なければなりません。
	fullThreadPosts int
}

// matureContentPlanはmatureプロファイルが生成する量です。賑わう掲示板は
// スレッド一覧の1ページ (M4) に収まらない数のスレッドを持ち、埋まったスレッドは
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

// coldStartContentPlanは、立ち上げ直後のコミュニティが持つ量です。スレッドは数本、
// 各スレッドの投稿も数件で、何ヶ月もかけて積み上がるものは何も含みません。最小のスレッドを
// 投稿1件のままにしているのは、まだ誰も答えていないスレッドが、開いたばかりの掲示板では
// ありふれた状態だからです。レス数も、投稿の下に付く逆参照も、1件しかないときには違う
// 読まれ方をします。
var coldStartContentPlan = contentPlan{
	quietBoardThreads: 3,
	minPostsPerThread: 1,
	maxPostsPerThread: 4,
	span:              3 * 24 * time.Hour,
	fullThreadPosts:   0,
}

// threadCountは、この賑わいの掲示板がいくつの通常のスレッドを得るのかを返します。
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

// contentSeedは生成が行う疑似乱数の選択を固定します。同じコードと同じ名簿からは
// 同じ行が同じ順序で生まれ、開発者が昨日開いたスレッドは今日も同じアドレスの先にあります。
// 実行のたびにidが入れ替われば、画面を見ながら取ったメモは翌朝には使えなくなります。
const contentSeed = 20260825

// postIntervalは、1つのスレッドの投稿どうしがどれだけ離れて書かれるかです。実行
// した瞬間の時刻をすべてに押した実行は、どの投稿も同じに読めるスレッドと、どの行も同じ値を
// 持つ列で並んだスレッド一覧を残します。
const postInterval = 11 * time.Minute

// scriptedThreadは投稿ごとに書き下したスレッド、scriptedPostはその投稿1件です。
// 見どころが言葉の働きそのものにあるスレッド (どの投稿がどれに答えるか、URLを含む本文、
// マークアップに見える本文) は、取り替えのきく文を並べても組み立てられません。
type scriptedThread struct {
	// languageはスレッドの主言語であり、題名の言語です。個々の投稿は別の言語を
	// 使えるため、下の本文すべての言語を表すものではありません。台本がこれを持つのは、
	// 文面そのものが言語だからです。別の言語へ書き直した台本が元の言語を宣言したままなら、
	// 題名には誤ったタグが付きます。宣言はそれを避けるために行うものです。
	language model.ThreadLanguage

	title string
	posts []scriptedPost
}

type scriptedPost struct {
	role seedRole
	body string
}

// referenceScriptはレス参照を眺めるためのスレッドです。後続の複数の投稿が答える
// 投稿 (その下に付く逆参照が一覧になります)、2つの投稿にまとめて答える投稿、同じ番号を
// 2度書いた本文 (参照は2つではなく1つです)、どの投稿も持たない番号 (テキストです) を
// 持ちます。
//
// URLを含む本文と、マークアップに見える本文もここに置いています。どちらも1つの投稿の
// 本文を描画するときに決まるものであり、本文が何を持てるのかを見せるスレッドがその
// 置き場所になります。
var referenceScript = scriptedThread{
	language: model.LocaleJa.ThreadLanguage(),
	title:    "レス参照の見え方を確かめるスレ",
	posts: []scriptedPost{
		{role: roleStarter, body: "1つ目の投稿です。ここに返信が集まると、この投稿の下に逆参照が並びます。"},
		{role: roleReplier, body: ">>1まずは1つ目の返信です。"},
		{role: roleStarter, body: ">>1 2つ目の返信です。これで1つ目の投稿には逆参照が2つ付きます。"},
		{role: roleReplier, body: ">>2 >>3 1つの投稿から2つの投稿を指すこともできます。"},
		{role: roleStarter, body: ">>1 >>1同じ番号を2度書いても、参照として残るのは1つだけです。"},
		{role: roleReplier, body: ">>999まだ誰も書いていない番号は、リンクにならずそのまま表示されます。"},
		{role: roleStarter, body: "https://example.com/help のように書いたURLはリンクになります。"},
		{role: roleReplier, body: "<b>タグに見える入力</b> や & のような記号も、書いたとおりに表示されます。"},
		{role: roleStarter, body: "改行を含む本文です。\n2行目はここから始まります。\n\n空行をはさむこともできます。"},
	},
}

// withdrawnScriptは、投稿を作者抜きで読むためのスレッドです。これを立てた
// アカウントは実行の最後に退会するため、スレッドとその投稿2件が作者を失う一方、それらに
// 答えた返信はその場に残ります。退会が外すのは書かれたものから名前であって、書かれたもの
// そのものではありません。
var withdrawnScript = scriptedThread{
	language: model.LocaleJa.ThreadLanguage(),
	title:    "退会した人の投稿が残っているスレ",
	posts: []scriptedPost{
		{role: roleWithdrawn, body: "このスレッドを立てたアカウントは、このあと退会します。"},
		{role: roleReplier, body: ">>1立てた人がいなくなっても、スレッドと投稿はここに残ります。"},
		{role: roleWithdrawn, body: ">>2返信の宛先も残るので、会話としてはそのまま読めるはずです。"},
		{role: roleStarter, body: ">>1 >>3作者のいない投稿がどう見えるかは、この2つで確かめられます。"},
	},
}

// englishScriptは、複数の言語が並んだ状態でスレッド一覧を読むためのスレッドです。
// 1インスタンスは1つのコミュニティを運営する (ADR 0006) ため、ラウンジは2つの言語に
// 2つのインスタンスで応えることをしません。両者は1つの掲示板に隣り合って並び、それが
// 掲示板の通常の見え方であって、例外として思い浮かべるものではありません。
//
// どのプロファイルにも置きます。立ち上げ直後も同様です。両方の言語を受け付けるコミュニティは
// 開いた日からそうしているため、日本語だけが並んだ一覧に照らして確かめた初日の画面は、
// ラウンジが一度も置かれない状態に照らしたものになります (ADR 0010)。
//
// 返信のうち1件は日本語です。返信はスレッドを立てたときの言語に縛られないためです。
// どの投稿もスレッド自身の言語と一致するのなら、言語が混ざったスレッドを眺める場所が
// どこにも無くなります。投稿の本文が言語を宣言しない理由は、そのスレッドでしか読み取れません。
var englishScript = scriptedThread{
	language: model.LocaleEn.ThreadLanguage(),
	title:    "Reading this board with a dictionary open",
	posts: []scriptedPost{
		{role: roleStarter, body: "Hello from the English side of the board. I can follow most of the Japanese threads, only slowly."},
		{role: roleReplier, body: ">>1 Welcome. Threads in either language belong here, so write in whichever one you think in."},
		{role: roleStarter, body: ">>2 Good to know. I will keep opening threads in English then."},
		{role: roleReplier, body: "英語のスレッドに日本語で返信しても構いません。読みに来る人はどちらの言語も見ています。"},
	},
}

// otherLanguageScriptは、アプリがロケールを持たない言語で書かれたスレッドで、
// model.ThreadLanguageOtherはこうしたスレッドのためにあります。ここではフランス語が
// その言語ですが、どの言語であるかにスレッドの何も依存しません。眺める対象は、どの表示言語
// にも解決しないスレッドであることのほうで、そのためバッジは訳語に退き、題名は何も宣言
// しません。
//
// 成熟したコミュニティにだけ置きます。ラウンジが受け付けると表明しているのは日本語と英語で
// あり、そのどちらでもないスレッドは、人が集まるにつれて現れるものであって、初日に必ず
// あるものではありません (ADR 0010)。
var otherLanguageScript = scriptedThread{
	language: model.ThreadLanguageOther,
	title:    "Est-ce que quelqu'un lit le français ici ?",
	posts: []scriptedPost{
		{role: roleStarter, body: "Bonjour à tous. J'écris en français, une langue que ce forum ne traduit pas."},
		{role: roleReplier, body: ">>1 Bienvenue. Le sujet restera marqué « autre », faute de traduction française."},
		{role: roleStarter, body: ">>2 Cela me convient très bien. Je repasserai écrire de temps en temps."},
	},
}

// fullThreadTitleは投稿数の上限に達したスレッドの題名です。数ではなく上限と
// 述べているのは、数がcontentPlanから来るもので、テストはずっと少ない投稿でこの
// スレッドを埋めるためです。
const fullThreadTitle = "上限まで埋まっていて書き込めないスレッド"

// threadTitles・openingBodies・replyBodiesは、通常のスレッドを書き起こす材料です。
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

// generatedThreadLanguageは、通常のスレッドと上限まで埋まったスレッドが書かれて
// いる言語で、上のコーパスが書かれている言語です。コーパスの隣に置くのは、文面と、それが
// 何語だと宣言されるかを一緒に変えられるようにするためです。コーパスだけを書き直して
// ここが取り残されれば、生成したどの行にも誤った言語のタグが付きます。
var generatedThreadLanguage = model.LocaleJa.ThreadLanguage()

// plannedThreadは書き込まれる形になったスレッドとその投稿、plannedPostはその
// 投稿1件です。スレッドを1つも書き込む前に丸ごと組み立てるのは、2つの工程を分けて
// おくためです。文面と乱数は組み立てにあり、idと時刻は書き込みにあります。
type plannedThread struct {
	board    *model.Board
	language model.ThreadLanguage
	title    string
	posts    []plannedPost
}

type plannedPost struct {
	author *model.User
	body   string
}

// contentGeneratorはコミュニティの会話を組み立て、書き込みます。
type contentGenerator struct {
	plan          contentPlan
	scripts       []scriptedThread
	rng           *rand.Rand
	users         *seededUsers
	threadRepo    *repository.ThreadRepository
	postRepo      *repository.PostRepository
	referenceRepo *repository.PostReferenceRepository
}

// generateThreadsはスレッドと、それが持つ投稿、そして投稿どうしの参照を作成
// します。
func (r *Runner) generateThreads(ctx context.Context, tx *sql.Tx, st *state) error {
	g := &contentGenerator{
		plan:    r.profile.plan,
		scripts: r.profile.scripts,
		// ここでの乱数が選ぶのは、投稿がどの文から書かれるかです。そのため予測
		// できる系列であることは欠陥ではなく、ここで求めているものです。実行は再現できる
		// 必要があります (contentSeedを参照)。この生成器が決めるものに秘密は無いため、
		// 暗号論的でない乱数源に対するgosecの指摘はここでは当たりません。
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

	// スレッドは古いものから書き込みます。作成された順と最後に投稿された順が
	// 一致するようにするためです。スレッド一覧は時刻の同着をidで解くため、両者が
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

// threadIntervalは、連続する2つのスレッドの最終投稿をどれだけ離して置くのかを
// 返します。スレッドの数がいくつであっても、それらがspanを埋めるようにするためです。
func threadInterval(span time.Duration, threadCount int) time.Duration {
	if threadCount == 0 {
		return 0
	}

	return span / time.Duration(threadCount)
}

// composeThreadsは実行が書き込むスレッドをすべて組み立てます。各掲示板が得る
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

	// 掲示板を探すのは、そこへ投稿するものがあるときだけです。何も書き下さない
	// プロファイルが、それを置ける掲示板を備えていることを求められないようにするため
	// です。
	hasFullThread := g.plan.fullThreadPosts > 0
	if len(g.scripts) == 0 && !hasFullThread {
		return threads, nil
	}

	board, err := scriptedBoard(boards)
	if err != nil {
		return nil, err
	}

	for _, script := range g.scripts {
		thread, err := g.composeScriptedThread(board, script)
		if err != nil {
			return nil, err
		}
		threads = append(threads, thread)
	}

	if hasFullThread {
		threads = append(threads, g.composeFullThread(board, speakers))
	}

	return threads, nil
}

// composeOrdinaryThreadは、掲示板を埋めるスレッドの1つを組み立てます。2人の
// うちどちらが立てるのかは交互に入れ替わります。スレッド一覧が1つのatnameの列に
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

	return plannedThread{board: board, language: generatedThreadLanguage, title: title, posts: posts}
}

// composeFullThreadは投稿数の上限に達したスレッドを組み立てます。最初の投稿が
// そう述べているのは、それ以外にそう述べるものが投稿の数しか無く、1000まで数える人は
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

	return plannedThread{board: board, language: generatedThreadLanguage, title: fullThreadTitle, posts: posts}
}

// composeScriptedThreadは台本をスレッドにします。各投稿が誰のものになるのかは、
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

	return plannedThread{board: board, language: script.language, title: script.title, posts: posts}, nil
}

// composeBodyは、指定のレス番号を持つことになる投稿の本文を書きます。返信が
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

// accountはその役割で作成されたアカウントを返します。アカウントの無い役割を、
// 作者のいない投稿にせずエラーにするのは、作者が不在であることが退会の結果だからです。
// 生成器が何かを取りこぼしてその状態へ辿り着いてはなりません。
func (g *contentGenerator) account(role seedRole) (*model.User, error) {
	user := g.users.user(role)
	if user == nil {
		return nil, fmt.Errorf("no account was created for the role %s", role)
	}

	return user, nil
}

// scriptedBoardは、開いて眺めるために書き下したスレッドが立つ掲示板を返します。
// 賑わう掲示板であり、賑わう掲示板を持たないコミュニティでは、ただ1つある掲示板です。
// 立ち上げ直後のコミュニティは掲示板を1つだけ提供し、それでも書き下したスレッドを
// 持つため、賑わう掲示板だけを規則にはできません。
//
// 掲示板が複数あって賑わうものが無い状態は、ここでの選択ではなくエラーにします。
// そのどれに書き下したスレッドが属するのかは掲示板からは決められず、そのスレッドを
// 探す開発者は、どこへ入ったのかを見つけるためにすべての掲示板を開くことになります。
func scriptedBoard(boards []seededBoard) (*model.Board, error) {
	for _, board := range boards {
		if board.activity == boardBusy {
			return board.board, nil
		}
	}

	if len(boards) == 1 {
		return boards[0].board, nil
	}

	return nil, fmt.Errorf("no board was created to hold the threads that are written out: none of the %d boards is the busy one", len(boards))
}

// writeThreadは組み立て済みのスレッドを書き込みます。スレッド、レス番号順の投稿、
// それらの本文が作る参照、そして結果として持つことになった投稿についてスレッドが持つ
// 非正規化された姿です。
func (g *contentGenerator) writeThread(ctx context.Context, tx *sql.Tx, planned plannedThread, lastPostedAt time.Time) error {
	thread, err := g.threadRepo.Create(ctx, repository.CreateThreadInput{
		BoardID:  planned.board.ID,
		UserID:   &planned.posts[0].author.ID,
		Title:    planned.title,
		Language: planned.language,
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

		// 参照は、この投稿が番号の解決先に加わる前に書き込みます。自分自身の番号を
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

	// スレッドが始まるのは最初の投稿が書かれた時点、最後に変わったのは最新の投稿が
	// 書かれた時点です。これは投稿自身が持つのと同じ2つの時刻です。
	if err := backdate(ctx, tx, backdateThreadStatement, int64(thread.ID), firstPostedAt, lastPostedAt); err != nil {
		return fmt.Errorf("failed to backdate the thread %q: %w", planned.title, err)
	}

	return nil
}

// writeReferencesは、本文が参照する投稿を、そのスレッドがそこまでに持っている
// 投稿の中から記録します。どの投稿も持たない番号はそのままにします。書かれたものの先を
// 指す >>Nはテキストであり、テキストのままです。
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

// backdatePostStatementとbackdateThreadStatementは、アプリケーションが書いた行を、
// シードの会話がそれを置いている時点まで戻します。
//
// アプリケーションは行に、それを書いた瞬間の時刻を押します。これはアプリケーションにとって
// 正しく、サンプルデータにとっては誤りです。すべてのスレッドが同じ1秒の中に投稿された
// 掲示板は、スレッド一覧がどう並ぶのかについても、1週間前の時刻がどう読めるのかについても
// 何も語りません。そのためシードは、行をアプリケーションと同じやり方で書いてから動かします。
// シードしか渡さない時刻を受け取る術をInfrastructure層へ教えることはしません。
//
// 投稿どうしの参照は書き込まれた瞬間の時刻を保ちます。その時刻を読むものが無いためです。
// 参照は指し先の投稿で引かれ、2つの投稿のidで並びます。
const (
	backdatePostStatement   = "UPDATE posts SET created_at = ?, updated_at = ? WHERE id = ?"
	backdateThreadStatement = "UPDATE threads SET created_at = ?, updated_at = ? WHERE id = ?"
)

// backdateは、上の文のいずれかを、指定のidの行に対して実行します。
func backdate(ctx context.Context, tx *sql.Tx, statement string, id int64, createdAt, updatedAt time.Time) error {
	_, err := tx.ExecContext(ctx, statement, sqlitetime.Time(createdAt), sqlitetime.Time(updatedAt), id)

	return err
}
