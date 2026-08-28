package seed

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
)

// testContentPlan is the plan the content tests generate with. It is the
// smallest plan that still produces every shape the mature one does: a board
// with more threads than the quiet ones, a board without any, and a thread
// filled to what the plan calls its limit. The span is the mature one, because
// the times the threads are spread over are what a thread list is ordered by.
//
// [Ja] testContentPlan は、中身のテストが生成に使う plan です。mature の plan が生む形を
// すべて備えたまま最も小さくしたもので、静かな掲示板より多くのスレッドを持つ掲示板、
// スレッドを 1 つも持たない掲示板、そして plan が上限と呼ぶ数まで埋まったスレッドを
// 生みます。span を mature と同じにしているのは、スレッドが散らばる時刻がスレッド一覧の
// 並び順の拠り所であるためです。
var testContentPlan = contentPlan{
	busyBoardThreads:  3,
	quietBoardThreads: 2,
	minPostsPerThread: 2,
	maxPostsPerThread: 4,
	span:              30 * 24 * time.Hour,
	fullThreadPosts:   5,
}

// testProfile is the profile the content tests generate with: the community the
// mature profile describes, filled by the smallest plan that still produces
// every shape it does.
//
// [Ja] testProfile は、中身のテストが生成に使うプロファイルです。mature プロファイルが
// 述べるコミュニティを、それが生む形をすべて備えたまま最も小さくした plan で埋めます。
func testProfile() Profile {
	profile := matureProfile
	profile.plan = testContentPlan

	return profile
}

// scriptedThreadCount is how many threads a run writes on top of the ordinary
// ones a board's activity asks for.
//
// [Ja] scriptedThreadCount は、掲示板の賑わいが求める通常のスレッドに加えて実行が
// 書き込むスレッドの数です。
const scriptedThreadCount = 3

// generateContent runs the generators the content depends on, in the order a run
// runs them, and returns the state they filled.
//
// [Ja] generateContent は、中身が依存する生成器を、実行がそれらを走らせる順で走らせ、
// それらが埋めた state を返します。
func generateContent(t *testing.T, db *database.DB, profile Profile) (*state, context.Context) {
	t.Helper()

	ctx := context.Background()
	runner := newTestRunner(db)
	runner.profile = profile

	st := &state{roster: testRoster()}
	tx := beginTx(t, db)

	for _, generator := range []struct {
		name     string
		generate func(context.Context, *sql.Tx, *state) error
	}{
		{name: "users", generate: runner.generateUsers},
		{name: "boards", generate: runner.generateBoards},
		{name: "threads", generate: runner.generateThreads},
	} {
		if err := generator.generate(ctx, tx, st); err != nil {
			t.Fatalf("generate%s() error = %v", generator.name, err)
		}
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit the transaction: %v", err)
	}

	return st, ctx
}

// threadsOf returns the board's threads in the order a thread list shows them.
//
// [Ja] threadsOf は掲示板のスレッドを、スレッド一覧が見せる順で返します。
func threadsOf(t *testing.T, db *database.DB, ctx context.Context, boardID model.BoardID) []*model.Thread {
	t.Helper()

	threads, err := repository.NewThreadRepository(db).ListByBoardID(ctx, boardID)
	if err != nil {
		t.Fatalf("ListByBoardID() error = %v", err)
	}

	return threads
}

// postsOf returns the thread's posts in reply-number order.
//
// [Ja] postsOf はスレッドの投稿をレス番号順で返します。
func postsOf(t *testing.T, db *database.DB, ctx context.Context, threadID model.ThreadID) []*model.Post {
	t.Helper()

	posts, err := repository.NewPostRepository(db).ListByThreadID(ctx, threadID)
	if err != nil {
		t.Fatalf("ListByThreadID() error = %v", err)
	}

	return posts
}

// findThread returns the thread with the given title.
//
// [Ja] findThread は指定の題名を持つスレッドを返します。
func findThread(t *testing.T, threads []*model.Thread, title string) *model.Thread {
	t.Helper()

	for _, thread := range threads {
		if thread.Title == title {
			return thread
		}
	}

	t.Fatalf("no thread was generated with the title %q", title)

	return nil
}

// TestRunner_GenerateThreads verifies that each board is filled to the amount
// its activity asks for, that a thread's denormalized view of its posts matches
// the posts it holds, and that the threads are placed in the past so that a
// thread list has an order to show.
//
// [Ja] TestRunner_GenerateThreads は、各掲示板がその賑わいの求める量まで埋まること、
// スレッドが持つ投稿の非正規化された姿がそのスレッドの実際の投稿と一致すること、そして
// スレッド一覧が見せる順序を持てるようスレッドが過去に置かれることを検証します。
func TestRunner_GenerateThreads(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	st, ctx := generateContent(t, db, testProfile())

	for _, seeded := range st.boards {
		threads := threadsOf(t, db, ctx, seeded.board.ID)

		want := testContentPlan.threadCount(seeded.activity)
		if seeded.activity == boardBusy {
			want += scriptedThreadCount
		}
		if len(threads) != want {
			t.Errorf("the board %q holds %d threads, want %d", seeded.board.Slug, len(threads), want)
		}

		for i, thread := range threads {
			posts := postsOf(t, db, ctx, thread.ID)
			if len(posts) == 0 {
				t.Fatalf("the thread %q holds no post", thread.Title)
			}

			if thread.PostsCount != len(posts) {
				t.Errorf("the thread %q counts %d posts, want %d", thread.Title, thread.PostsCount, len(posts))
			}

			last := posts[len(posts)-1]
			if thread.LastPostID == nil || *thread.LastPostID != last.ID {
				t.Errorf("the thread %q points at %v as its last post, want %v", thread.Title, thread.LastPostID, last.ID)
			}

			// The time a thread list orders by is the time of the post the
			// thread points at, and a thread begins when its first post was
			// written. A row that disagreed with its own posts would read as
			// posted in a moment nothing was written in.
			//
			// [Ja] スレッド一覧が並び順の拠り所にする時刻は、スレッドが指す投稿の時刻
			// であり、スレッドが始まるのは最初の投稿が書かれた時点です。自身の投稿と
			// 食い違う行は、何も書かれていない時点に投稿されたものとして読めてしまいます。
			if !thread.LastPostedAt.Equal(last.CreatedAt) {
				t.Errorf("the thread %q was last posted in at %v, want %v", thread.Title, thread.LastPostedAt, last.CreatedAt)
			}
			if !thread.CreatedAt.Equal(posts[0].CreatedAt) {
				t.Errorf("the thread %q was created at %v, want %v", thread.Title, thread.CreatedAt, posts[0].CreatedAt)
			}

			for j := 1; j < len(posts); j++ {
				if !posts[j].CreatedAt.After(posts[j-1].CreatedAt) {
					t.Errorf("the post %d of the thread %q was written at %v, want a time after %v",
						posts[j].Number, thread.Title, posts[j].CreatedAt, posts[j-1].CreatedAt)
				}
			}

			// The list comes back with the most recently posted-in first, so a
			// row that shares its time with the one above it means the threads
			// were stamped with a single moment.
			//
			// [Ja] 一覧は最後に投稿されたものから順に返るため、上の行と時刻を共有する行が
			// あることは、スレッドが 1 つの時点で押されたことを意味します。
			if i > 0 && !threads[i-1].LastPostedAt.After(thread.LastPostedAt) {
				t.Errorf("the thread %q was last posted in at %v, want a time before %v",
					thread.Title, thread.LastPostedAt, threads[i-1].LastPostedAt)
			}
		}
	}
}

// TestRunner_GenerateThreads_WritesTheReferencesTheBodiesMake verifies that the
// references stored are the ones the bodies write: a post several later posts
// answer collects one back reference per answering post, a number written twice
// is one reference, and a number no post carries is none.
//
// [Ja] TestRunner_GenerateThreads_WritesTheReferencesTheBodiesMake は、保存される参照が
// 本文の書いたものであることを検証します。後続の複数の投稿が答える投稿は答えた投稿の数だけ
// 逆参照を集め、2 度書かれた番号は 1 つの参照になり、どの投稿も持たない番号は参照になり
// ません。
func TestRunner_GenerateThreads_WritesTheReferencesTheBodiesMake(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	st, ctx := generateContent(t, db, testProfile())

	busy, err := busyBoard(st.boards)
	if err != nil {
		t.Fatalf("busyBoard() error = %v", err)
	}

	thread := findThread(t, threadsOf(t, db, ctx, busy.ID), referenceScript.title)
	posts := postsOf(t, db, ctx, thread.ID)
	if len(posts) != len(referenceScript.posts) {
		t.Fatalf("the thread %q holds %d posts, want %d", thread.Title, len(posts), len(referenceScript.posts))
	}

	byNumber := make(map[int]*model.Post, len(posts))
	ids := make([]model.PostID, 0, len(posts))
	for _, post := range posts {
		byNumber[post.Number] = post
		ids = append(ids, post.ID)
	}

	references, err := repository.NewPostReferenceRepository(db).ListByReferencedPostIDs(ctx, ids)
	if err != nil {
		t.Fatalf("ListByReferencedPostIDs() error = %v", err)
	}

	// The script is the statement of what the thread refers to, so the expected
	// references are read back out of it rather than written down a second time
	// here: a script edited to show something else has to keep agreeing with
	// what was stored.
	//
	// [Ja] 台本はスレッドが何を参照するのかの記述そのものであるため、期待する参照は
	// ここへ二度書かず台本から読み直します。別のものを見せるために台本を書き換えても、
	// 保存された内容との一致は保たれる必要があります。
	want := make(map[model.PostID][]model.PostID)
	for i, scripted := range referenceScript.posts {
		for _, number := range model.ReferencedPostNumbers(scripted.body) {
			referenced, ok := byNumber[number]
			if !ok || number > i {
				continue
			}
			want[referenced.ID] = append(want[referenced.ID], byNumber[i+1].ID)
		}
	}

	got := make(map[model.PostID][]model.PostID)
	for _, reference := range references {
		got[reference.ReferencedPostID] = append(got[reference.ReferencedPostID], reference.PostID)
	}

	if len(got) != len(want) {
		t.Errorf("references point at %d posts, want %d", len(got), len(want))
	}
	for referencedID, wantIDs := range want {
		gotIDs := got[referencedID]
		if len(gotIDs) != len(wantIDs) {
			t.Errorf("the post %v is referred to by %d posts, want %d", referencedID, len(gotIDs), len(wantIDs))
			continue
		}
		for i, wantID := range wantIDs {
			if gotIDs[i] != wantID {
				t.Errorf("the post %v is referred to by %v, want %v", referencedID, gotIDs[i], wantID)
			}
		}
	}

	// The thread is there to be looked at with more than one back reference
	// under a single post, which is the case a list of them has to be rendered
	// for.
	//
	// [Ja] このスレッドは、1 つの投稿の下に逆参照が 2 つ以上付いた状態を眺めるために
	// あります。それらを一覧として描画する必要が生じるのはその場合です。
	if len(want[byNumber[1].ID]) < 2 {
		t.Errorf("the first post of %q is referred to by %d posts, want at least 2", thread.Title, len(want[byNumber[1].ID]))
	}
}

// TestRunner_GenerateThreads_FillsAThreadToTheLimit verifies that the thread
// that has reached the post limit holds exactly what the plan calls the limit,
// which is what a screen saying the thread is full is checked against.
//
// [Ja] TestRunner_GenerateThreads_FillsAThreadToTheLimit は、投稿数の上限に達した
// スレッドが、plan が上限と呼ぶ数をちょうど持つことを検証します。スレッドが埋まっている
// と述べる画面は、それに照らして確かめられます。
func TestRunner_GenerateThreads_FillsAThreadToTheLimit(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	st, ctx := generateContent(t, db, testProfile())

	busy, err := busyBoard(st.boards)
	if err != nil {
		t.Fatalf("busyBoard() error = %v", err)
	}

	thread := findThread(t, threadsOf(t, db, ctx, busy.ID), fullThreadTitle)
	if thread.PostsCount != testContentPlan.fullThreadPosts {
		t.Errorf("the full thread counts %d posts, want %d", thread.PostsCount, testContentPlan.fullThreadPosts)
	}

	posts := postsOf(t, db, ctx, thread.ID)
	if len(posts) != testContentPlan.fullThreadPosts {
		t.Errorf("the full thread holds %d posts, want %d", len(posts), testContentPlan.fullThreadPosts)
	}
}

// TestRunner_GenerateThreads_AttributesTheWithdrawnThread verifies that the
// thread meant to be read without an author is written by the account that
// withdraws, both as the thread's author and as the author of posts inside it.
// The withdrawal itself comes later in the run, so what is checked here is that
// there is something for it to take the name off.
//
// [Ja] TestRunner_GenerateThreads_AttributesTheWithdrawnThread は、作者抜きで読まれる
// ためのスレッドが、退会するアカウントによって、スレッドの作者としても、その中の投稿の
// 作者としても書かれることを検証します。退会そのものは実行の後の段階で起きるため、ここで
// 確かめるのは、退会が名前を外す相手が存在することです。
func TestRunner_GenerateThreads_AttributesTheWithdrawnThread(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	st, ctx := generateContent(t, db, testProfile())

	busy, err := busyBoard(st.boards)
	if err != nil {
		t.Fatalf("busyBoard() error = %v", err)
	}

	withdrawing := st.users.user(roleWithdrawn)
	if withdrawing == nil {
		t.Fatal("no account was created for the withdrawing role")
	}

	thread := findThread(t, threadsOf(t, db, ctx, busy.ID), withdrawnScript.title)
	if thread.UserID == nil || *thread.UserID != withdrawing.ID {
		t.Errorf("the thread %q was started by %v, want %v", thread.Title, thread.UserID, withdrawing.ID)
	}

	written := 0
	for _, post := range postsOf(t, db, ctx, thread.ID) {
		if post.UserID != nil && *post.UserID == withdrawing.ID {
			written++
		}
	}
	if written < 2 {
		t.Errorf("the withdrawing account wrote %d posts in %q, want at least 2", written, thread.Title)
	}
}

// TestRunner_GenerateThreads_IsRepeatable verifies that two generations produce
// the same conversations. The addresses a developer noted while looking at a
// screen have to still lead to what they were looking at after the database has
// been rebuilt.
//
// [Ja] TestRunner_GenerateThreads_IsRepeatable は、2 回の生成が同じ会話を生むことを
// 検証します。開発者が画面を見ながら書き留めたアドレスは、データベースを作り直した後も、
// 見ていたものへ辿り着けなければなりません。
func TestRunner_GenerateThreads_IsRepeatable(t *testing.T) {
	t.Parallel()

	first := generatedConversations(t)
	second := generatedConversations(t)

	if len(first) != len(second) {
		t.Fatalf("the two generations produced %d and %d posts", len(first), len(second))
	}
	for i, post := range first {
		if post != second[i] {
			t.Fatalf("the two generations differ at %d: %q and %q", i, post, second[i])
		}
	}
}

// generatedConversations returns every post one generation wrote, as the thread
// it belongs to, its reply number and its body.
//
// [Ja] generatedConversations は、1 回の生成が書いたすべての投稿を、それが属する
// スレッド・レス番号・本文の形で返します。
func generatedConversations(t *testing.T) []string {
	t.Helper()

	db := testutil.SetupDB(t)
	st, ctx := generateContent(t, db, testProfile())

	var conversations []string
	for _, seeded := range st.boards {
		for _, thread := range threadsOf(t, db, ctx, seeded.board.ID) {
			for _, post := range postsOf(t, db, ctx, thread.ID) {
				conversations = append(conversations, thread.Title+"\x00"+post.Body)
			}
		}
	}

	return conversations
}

// TestRunner_GenerateThreads_ReportsAMissingRole verifies that a run whose
// accounts do not cover the roles the conversations name stops before writing,
// rather than attributing a post to nobody.
//
// [Ja] TestRunner_GenerateThreads_ReportsAMissingRole は、アカウントが会話の名指しする
// 役割を満たしていない実行が、投稿を誰のものでもない状態にするのではなく、書き込む前に
// 止まることを検証します。
func TestRunner_GenerateThreads_ReportsAMissingRole(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testutil.SetupDB(t)
	runner := newTestRunner(db)
	runner.profile = testProfile()

	st := &state{
		roster: testRoster(),
		users:  &seededUsers{byRole: map[seedRole]*model.User{}},
	}
	tx := beginTx(t, db)
	if err := runner.generateBoards(ctx, tx, st); err != nil {
		t.Fatalf("generateBoards() error = %v", err)
	}

	err := runner.generateThreads(ctx, tx, st)
	if err == nil {
		t.Fatal("generateThreads() should fail when a role has no account, but it succeeded")
	}
	if !strings.Contains(err.Error(), string(roleStarter)) {
		t.Errorf("generateThreads() error = %q, want it to name the role it could not resolve", err)
	}
}
