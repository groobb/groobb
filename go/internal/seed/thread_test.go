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

// testContentPlanは、中身のテストが生成に使うplanです。matureのplanが生む形を
// すべて備えたまま最も小さくしたもので、静かな掲示板より多くのスレッドを持つ掲示板、
// スレッドを1つも持たない掲示板、そしてplanが上限と呼ぶ数まで埋まったスレッドを
// 生みます。spanをmatureと同じにしているのは、スレッドが散らばる時刻がスレッド一覧の
// 並び順の拠り所であるためです。
var testContentPlan = contentPlan{
	busyBoardThreads:  3,
	quietBoardThreads: 2,
	minPostsPerThread: 2,
	maxPostsPerThread: 4,
	span:              30 * 24 * time.Hour,
	fullThreadPosts:   5,
}

// testProfileは、中身のテストが生成に使うプロファイルです。matureプロファイルが
// 述べるコミュニティを、それが生む形をすべて備えたまま最も小さくしたplanで埋めます。
func testProfile() Profile {
	profile := matureProfile
	profile.plan = testContentPlan

	return profile
}

// writtenThreadCountは、掲示板の賑わいが求める通常のスレッドに加えてプロファイルが
// 書き込むスレッドの数です。投稿ごとに書き下したものと、planがそれを求める場合の上限まで
// 埋まったものを合わせた数になります。書き下さずにプロファイルから数えるのは、プロファイル
// へ台本を足したときに、ここへも足す必要が生じないようにするためです。
func writtenThreadCount(profile Profile) int {
	count := len(profile.scripts)
	if profile.plan.fullThreadPosts > 0 {
		count++
	}

	return count
}

// generateContentは、中身が依存する生成器を、実行がそれらを走らせる順で走らせ、
// それらが埋めたstateを返します。
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
			t.Fatalf("generate%s()のエラー = %v", generator.name, err)
		}
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("トランザクションのコミットに失敗: %v", err)
	}

	return st, ctx
}

// threadsOfは掲示板のスレッドを、スレッド一覧が見せる順で返します。
func threadsOf(t *testing.T, db *database.DB, ctx context.Context, boardID model.BoardID) []*model.Thread {
	t.Helper()

	threads, err := repository.NewThreadRepository(db).ListByBoardID(ctx, boardID)
	if err != nil {
		t.Fatalf("ListByBoardID()のエラー = %v", err)
	}

	return threads
}

// postsOfはスレッドの投稿をレス番号順で返します。
func postsOf(t *testing.T, db *database.DB, ctx context.Context, threadID model.ThreadID) []*model.Post {
	t.Helper()

	posts, err := repository.NewPostRepository(db).ListByThreadID(ctx, threadID)
	if err != nil {
		t.Fatalf("ListByThreadID()のエラー = %v", err)
	}

	return posts
}

// findThreadは指定の題名を持つスレッドを返します。
func findThread(t *testing.T, threads []*model.Thread, title string) *model.Thread {
	t.Helper()

	for _, thread := range threads {
		if thread.Title == title {
			return thread
		}
	}

	t.Fatalf("タイトル %q のスレッドが生成されていない", title)

	return nil
}

// TestScriptedBoardは、賑わう掲示板があればそれを選び、無ければ唯一の掲示板を
// 選ぶこと、そして賑わっていない掲示板が複数あれば、恣意的に1つへ決めず拒否することを
// 検証します。
func TestScriptedBoard(t *testing.T) {
	t.Parallel()

	busy := &model.Board{Slug: "busy"}
	quiet := &model.Board{Slug: "quiet"}
	sole := &model.Board{Slug: "sole"}
	first := &model.Board{Slug: "first"}
	second := &model.Board{Slug: "second"}

	tests := []struct {
		name    string
		boards  []seededBoard
		want    *model.Board
		wantErr bool
	}{
		{
			name: "複数の掲示板のうちbusyの掲示板",
			boards: []seededBoard{
				{board: quiet, activity: boardQuiet},
				{board: busy, activity: boardBusy},
			},
			want: busy,
		},
		{
			name:   "busyが無いときの唯一の掲示板",
			boards: []seededBoard{{board: sole, activity: boardQuiet}},
			want:   sole,
		},
		{
			name: "busyが無い複数の掲示板",
			boards: []seededBoard{
				{board: first, activity: boardQuiet},
				{board: second, activity: boardEmpty},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := scriptedBoard(tt.boards)
			if tt.wantErr {
				if err == nil {
					t.Fatal("scriptedBoard()が失敗することを期待したが、成功した")
				}
				if got != nil {
					t.Errorf("scriptedBoard() = %q、掲示板が無いことを期待", got.Slug)
				}
				if !strings.Contains(err.Error(), "2 boards") {
					t.Errorf("scriptedBoard()のエラー = %q、2つの掲示板を名指すことを期待", err)
				}

				return
			}

			if err != nil {
				t.Fatalf("scriptedBoard()のエラー = %v", err)
			}
			if got != tt.want {
				t.Errorf("scriptedBoard() = %q、期待値 = %q", got.Slug, tt.want.Slug)
			}
		})
	}
}

// TestRunner_GenerateThreadsは、各掲示板がその賑わいの求める量まで埋まること、
// スレッドが持つ投稿の非正規化された姿がそのスレッドの実際の投稿と一致すること、そして
// スレッド一覧が見せる順序を持てるようスレッドが過去に置かれることを検証します。
func TestRunner_GenerateThreads(t *testing.T) {
	t.Parallel()

	profile := testProfile()
	db := testutil.SetupDB(t)
	st, ctx := generateContent(t, db, profile)

	written, err := scriptedBoard(st.boards)
	if err != nil {
		t.Fatalf("scriptedBoard()のエラー = %v", err)
	}

	for _, seeded := range st.boards {
		threads := threadsOf(t, db, ctx, seeded.board.ID)

		want := testContentPlan.threadCount(seeded.activity)
		if seeded.board.ID == written.ID {
			want += writtenThreadCount(profile)
		}
		if len(threads) != want {
			t.Errorf("掲示板 %q のスレッド数 = %d、期待値 = %d", seeded.board.Slug, len(threads), want)
		}

		for i, thread := range threads {
			posts := postsOf(t, db, ctx, thread.ID)
			if len(posts) == 0 {
				t.Fatalf("スレッド %q に投稿が無い", thread.Title)
			}

			if thread.PostsCount != len(posts) {
				t.Errorf("スレッド %q のPostsCount = %d、期待値 = %d", thread.Title, thread.PostsCount, len(posts))
			}

			last := posts[len(posts)-1]
			if thread.LastPostID == nil || *thread.LastPostID != last.ID {
				t.Errorf("スレッド %q のLastPostID = %v、期待値 = %v", thread.Title, thread.LastPostID, last.ID)
			}

			// スレッド一覧が並び順の拠り所にする時刻は、スレッドが指す投稿の時刻
			// であり、スレッドが始まるのは最初の投稿が書かれた時点です。自身の投稿と
			// 食い違う行は、何も書かれていない時点に投稿されたものとして読めてしまいます。
			if !thread.LastPostedAt.Equal(last.CreatedAt) {
				t.Errorf("スレッド %q のLastPostedAt = %v、期待値 = %v", thread.Title, thread.LastPostedAt, last.CreatedAt)
			}
			if !thread.CreatedAt.Equal(posts[0].CreatedAt) {
				t.Errorf("スレッド %q のCreatedAt = %v、期待値 = %v", thread.Title, thread.CreatedAt, posts[0].CreatedAt)
			}

			for j := 1; j < len(posts); j++ {
				if !posts[j].CreatedAt.After(posts[j-1].CreatedAt) {
					t.Errorf("%d 番の投稿 (スレッド %q) のCreatedAt = %v、%v より後の時刻を期待",
						posts[j].Number, thread.Title, posts[j].CreatedAt, posts[j-1].CreatedAt)
				}
			}

			// 一覧は最後に投稿されたものから順に返るため、上の行と時刻を共有する行が
			// あることは、スレッドが1つの時点で押されたことを意味します。
			if i > 0 && !threads[i-1].LastPostedAt.After(thread.LastPostedAt) {
				t.Errorf("スレッド %q のLastPostedAt = %v、%v より前の時刻を期待",
					thread.Title, thread.LastPostedAt, threads[i-1].LastPostedAt)
			}
		}
	}
}

// TestRunner_GenerateThreads_WritesTheReferencesTheBodiesMakeは、保存される参照が
// 本文の書いたものであることを検証します。後続の複数の投稿が答える投稿は答えた投稿の数だけ
// 逆参照を集め、2度書かれた番号は1つの参照になり、どの投稿も持たない番号は参照になり
// ません。
func TestRunner_GenerateThreads_WritesTheReferencesTheBodiesMake(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	st, ctx := generateContent(t, db, testProfile())

	busy, err := scriptedBoard(st.boards)
	if err != nil {
		t.Fatalf("scriptedBoard()のエラー = %v", err)
	}

	thread := findThread(t, threadsOf(t, db, ctx, busy.ID), referenceScript.title)
	posts := postsOf(t, db, ctx, thread.ID)
	if len(posts) != len(referenceScript.posts) {
		t.Fatalf("スレッド %q の投稿数 = %d、期待値 = %d", thread.Title, len(posts), len(referenceScript.posts))
	}

	byNumber := make(map[int]*model.Post, len(posts))
	ids := make([]model.PostID, 0, len(posts))
	for _, post := range posts {
		byNumber[post.Number] = post
		ids = append(ids, post.ID)
	}

	references, err := repository.NewPostReferenceRepository(db).ListByReferencedPostIDs(ctx, ids)
	if err != nil {
		t.Fatalf("ListByReferencedPostIDs()のエラー = %v", err)
	}

	// 台本はスレッドが何を参照するのかの記述そのものであるため、期待する参照は
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
		t.Errorf("参照先の投稿数 = %d、期待値 = %d", len(got), len(want))
	}
	for referencedID, wantIDs := range want {
		gotIDs := got[referencedID]
		if len(gotIDs) != len(wantIDs) {
			t.Errorf("投稿 %v を参照する投稿数 = %d、期待値 = %d", referencedID, len(gotIDs), len(wantIDs))
			continue
		}
		for i, wantID := range wantIDs {
			if gotIDs[i] != wantID {
				t.Errorf("投稿 %v を参照する投稿 = %v、期待値 = %v", referencedID, gotIDs[i], wantID)
			}
		}
	}

	// このスレッドは、1つの投稿の下に逆参照が2つ以上付いた状態を眺めるために
	// あります。それらを一覧として描画する必要が生じるのはその場合です。
	if len(want[byNumber[1].ID]) < 2 {
		t.Errorf("%q の最初の投稿を参照する投稿数 = %d、期待値 = 2以上", thread.Title, len(want[byNumber[1].ID]))
	}
}

// TestRunner_GenerateThreads_FillsAThreadToTheLimitは、投稿数の上限に達した
// スレッドが、planが上限と呼ぶ数をちょうど持つことを検証します。スレッドが埋まっている
// と述べる画面は、それに照らして確かめられます。
func TestRunner_GenerateThreads_FillsAThreadToTheLimit(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	st, ctx := generateContent(t, db, testProfile())

	busy, err := scriptedBoard(st.boards)
	if err != nil {
		t.Fatalf("scriptedBoard()のエラー = %v", err)
	}

	thread := findThread(t, threadsOf(t, db, ctx, busy.ID), fullThreadTitle)
	if want := model.LocaleJa.ThreadLanguage(); thread.Language != want {
		t.Errorf("上限に達したスレッドの言語 = %q、期待値 = %q", thread.Language, want)
	}
	if thread.PostsCount != testContentPlan.fullThreadPosts {
		t.Errorf("上限に達したスレッドのPostsCount = %d、期待値 = %d", thread.PostsCount, testContentPlan.fullThreadPosts)
	}

	posts := postsOf(t, db, ctx, thread.ID)
	if len(posts) != testContentPlan.fullThreadPosts {
		t.Errorf("上限に達したスレッドの投稿数 = %d、期待値 = %d", len(posts), testContentPlan.fullThreadPosts)
	}
}

// TestRunner_GenerateThreads_AttributesTheWithdrawnThreadは、作者抜きで読まれる
// ためのスレッドが、退会するアカウントによって、スレッドの作者としても、その中の投稿の
// 作者としても書かれることを検証します。退会そのものは実行の後の段階で起きるため、ここで
// 確かめるのは、退会が名前を外す相手が存在することです。
func TestRunner_GenerateThreads_AttributesTheWithdrawnThread(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	st, ctx := generateContent(t, db, testProfile())

	busy, err := scriptedBoard(st.boards)
	if err != nil {
		t.Fatalf("scriptedBoard()のエラー = %v", err)
	}

	withdrawing := st.users.user(roleWithdrawn)
	if withdrawing == nil {
		t.Fatal("退会するロールのアカウントが作成されていない")
	}

	thread := findThread(t, threadsOf(t, db, ctx, busy.ID), withdrawnScript.title)
	if thread.UserID == nil || *thread.UserID != withdrawing.ID {
		t.Errorf("スレッド %q を立てたユーザー = %v、期待値 = %v", thread.Title, thread.UserID, withdrawing.ID)
	}

	written := 0
	for _, post := range postsOf(t, db, ctx, thread.ID) {
		if post.UserID != nil && *post.UserID == withdrawing.ID {
			written++
		}
	}
	if written < 2 {
		t.Errorf("退会するアカウントの投稿数 = %d (スレッド %q)、期待値 = 2以上", written, thread.Title)
	}
}

// TestRunner_GenerateThreads_MixesTheLanguagesは、書き下したスレッドが立つ掲示板が
// スレッドを書ける言語をすべて備えること、そして通常のスレッドだけを持つ掲示板が、それらが
// 書かれている言語を持つことを検証します。どの行も同じに読める掲示板は、複数の言語が並ぶ
// 一覧の見え方を何も見せません。ラウンジの掲示板が置かれているのはその状態です。
func TestRunner_GenerateThreads_MixesTheLanguages(t *testing.T) {
	t.Parallel()

	profile := testProfile()
	db := testutil.SetupDB(t)
	st, ctx := generateContent(t, db, profile)

	written, err := scriptedBoard(st.boards)
	if err != nil {
		t.Fatalf("scriptedBoard()のエラー = %v", err)
	}

	threads := threadsOf(t, db, ctx, written.ID)

	languages := make(map[model.ThreadLanguage]int, len(threads))
	for _, thread := range threads {
		languages[thread.Language]++
	}
	for _, language := range model.ThreadLanguages() {
		if languages[language] == 0 {
			t.Errorf("掲示板 %q に言語 %q のスレッドが無い", written.Slug, language)
		}
	}

	// 台本はそのスレッドが何語で書かれているのかを述べるものであるため、保存された
	// 内容はここへ二度書かず台本に照らして読み直します。
	for _, script := range profile.scripts {
		thread := findThread(t, threads, script.title)
		if thread.Language != script.language {
			t.Errorf("スレッド %q の言語 = %q、期待値 = %q", thread.Title, thread.Language, script.language)
		}
	}

	for _, seeded := range st.boards {
		if seeded.board.ID == written.ID {
			continue
		}
		for _, thread := range threadsOf(t, db, ctx, seeded.board.ID) {
			if want := model.LocaleJa.ThreadLanguage(); thread.Language != want {
				t.Errorf("通常のスレッド %q の言語 = %q、期待値 = %q", thread.Title, thread.Language, want)
			}
		}
	}
}

// TestRunner_GenerateThreads_IsRepeatableは、2回の生成が同じ会話を生むことを
// 検証します。開発者が画面を見ながら書き留めたアドレスは、データベースを作り直した後も、
// 見ていたものへ辿り着けなければなりません。
func TestRunner_GenerateThreads_IsRepeatable(t *testing.T) {
	t.Parallel()

	first := generatedConversations(t)
	second := generatedConversations(t)

	if len(first) != len(second) {
		t.Fatalf("2回の生成の投稿数が %d と %d で異なる", len(first), len(second))
	}
	for i, post := range first {
		if post != second[i] {
			t.Fatalf("2回の生成が %d 番目で異なる: %q と %q", i, post, second[i])
		}
	}
}

// generatedConversationsは、1回の生成が書いたすべての投稿を、それが属する
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

// TestRunner_GenerateThreads_ReportsAMissingRoleは、アカウントが会話の名指しする
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
		t.Fatalf("generateBoards()のエラー = %v", err)
	}

	err := runner.generateThreads(ctx, tx, st)
	if err == nil {
		t.Fatal("ロールにアカウントが無いときにgenerateThreads()が失敗することを期待したが、成功した")
	}
	if !strings.Contains(err.Error(), string(roleStarter)) {
		t.Errorf("generateThreads()のエラー = %q、解決できなかったロールを名指すことを期待", err)
	}
}
