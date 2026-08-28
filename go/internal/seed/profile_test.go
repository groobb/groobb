package seed

import (
	"slices"
	"testing"
	"time"

	"github.com/groobb/groobb/go/internal/testutil"
)

// TestFindProfile verifies that a command line reaches the states written here
// by name, and that a name nothing is written under is reported as such rather
// than resolved to something.
//
// [Ja] TestFindProfile は、コマンドラインがここに書かれた状態へ名前で辿り着けること、
// そして何も書かれていない名前が、何かへ解決されるのではなくそのように報告されることを
// 検証します。
func TestFindProfile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		want   bool
		boards int
	}{
		{name: "mature", want: true, boards: len(matureBoards)},
		{name: "cold-start", want: true, boards: len(coldStartBoards)},
		{name: "coldstart", want: false},
		{name: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			profile, ok := FindProfile(tt.name)

			if ok != tt.want {
				t.Fatalf("FindProfile(%q) ok = %v, want %v", tt.name, ok, tt.want)
			}
			if !tt.want {
				return
			}
			if profile.Name() != tt.name {
				t.Errorf("FindProfile(%q).Name() = %q, want %q", tt.name, profile.Name(), tt.name)
			}
			if len(profile.boards) != tt.boards {
				t.Errorf("FindProfile(%q) board count = %d, want %d", tt.name, len(profile.boards), tt.boards)
			}
		})
	}
}

// TestProfileNames verifies that the usage line the names are written into
// offers what FindProfile answers to, and that the default is among them: a name
// the line omits is one nobody finds, and one it offers but the lookup refuses
// would send a developer to a command that fails.
//
// [Ja] TestProfileNames は、名前を書き込む usage の行が、FindProfile が応じるものを
// 提示していること、そして既定がその中にあることを検証します。行が落とした名前は誰にも
// 見つけられず、行が提示しても引きが拒む名前は、開発者を失敗するコマンドへ送ることに
// なります。
func TestProfileNames(t *testing.T) {
	t.Parallel()

	names := ProfileNames()

	if len(names) != len(profiles) {
		t.Fatalf("ProfileNames() = %v, want %d names", names, len(profiles))
	}
	for _, name := range names {
		if _, ok := FindProfile(name); !ok {
			t.Errorf("ProfileNames() offers %q, which FindProfile does not answer to", name)
		}
	}
	if want := DefaultProfile().Name(); !slices.Contains(names, want) {
		t.Errorf("ProfileNames() = %v, want it to hold the default %q", names, want)
	}
}

// TestProfiles_SpreadTheirThreadsOverTime verifies that every profile places
// its threads in the past. A plan written without a span would stamp them all
// with the moment the run happened, leaving a thread list ordered on a column
// every row shares and a board whose every thread reads as posted just now.
//
// [Ja] TestProfiles_SpreadTheirThreadsOverTime は、どのプロファイルもスレッドを過去に
// 置くことを検証します。span を書かずに作られた plan は、そのすべてに実行した瞬間の時刻を
// 押すため、どの行も同じ値を持つ列で並んだスレッド一覧と、どのスレッドも今しがた投稿された
// ように読める掲示板を残します。
func TestProfiles_SpreadTheirThreadsOverTime(t *testing.T) {
	t.Parallel()

	for _, profile := range profiles {
		if profile.plan.span <= 0 {
			t.Errorf("the profile %q has the span %s, want a positive one", profile.name, profile.plan.span)
		}
	}
}

// TestProfiles_NameTheirCommunity verifies that every profile carries a name for
// the community it generates, and that no two carry the same one. The name is
// what the sidebar heading and the suffix of every page title show, so a profile
// without one would generate a community that names itself nowhere, and two
// profiles sharing one would leave the states they generate indistinguishable at
// the place the name is read.
//
// [Ja] TestProfiles_NameTheirCommunity は、どのプロファイルも生成するコミュニティの
// 名前を持つこと、そして 2 つが同じ名前を持たないことを検証します。名前はサイドバーの
// 見出しと各ページのタイトルの接尾辞に出るものであり、名前の無いプロファイルは、どこでも
// 自身を名乗らないコミュニティを生成します。同じ名前を共有すれば、名前が読まれる場所で
// 生成された状態を見分けられなくなります。
func TestProfiles_NameTheirCommunity(t *testing.T) {
	t.Parallel()

	namedBy := make(map[string]string, len(profiles))
	for _, profile := range profiles {
		if profile.communityName == "" {
			t.Errorf("the profile %q names no community", profile.name)

			continue
		}
		if other, exists := namedBy[profile.communityName]; exists {
			t.Errorf("the profiles %q and %q both name their community %q", other, profile.name, profile.communityName)

			continue
		}
		namedBy[profile.communityName] = profile.name
	}
}

// TestRunner_GenerateThreads_ColdStart verifies that the cold-start profile
// produces the state an instance opens in: one board with no category, a few
// threads in it, and a few posts in each. It is the state ADR 0010 asks the
// screens to be checked in, so nothing a community accumulates over months — a
// thread at the post limit, the threads written out post by post — may appear
// in it.
//
// [Ja] TestRunner_GenerateThreads_ColdStart は、cold-start プロファイルがインスタンスの
// 開くときの状態を生むことを検証します。カテゴリーを持たない掲示板 1 つ、そこに立つ
// 数本のスレッド、各スレッドの数件の投稿です。ADR 0010 が画面を確かめる先として求める
// のがこの状態であるため、コミュニティが何ヶ月もかけて蓄積するもの (投稿数の上限に
// 達したスレッドや、投稿ごとに書き下したスレッド) が現れてはなりません。
func TestRunner_GenerateThreads_ColdStart(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	st, ctx := generateContent(t, db, coldStartProfile)

	if len(st.boards) != 1 {
		t.Fatalf("board count = %d, want 1", len(st.boards))
	}

	board := st.boards[0].board
	if board.CategoryID != nil {
		t.Errorf("the board %q has the category %v, want none", board.Slug, *board.CategoryID)
	}

	var categoryCount int
	if err := db.Reader.QueryRowContext(ctx, "SELECT COUNT(*) FROM categories").Scan(&categoryCount); err != nil {
		t.Fatalf("failed to count the categories: %v", err)
	}
	if categoryCount != 0 {
		t.Errorf("category count = %d, want 0", categoryCount)
	}

	threads := threadsOf(t, db, ctx, board.ID)
	if want := coldStartContentPlan.quietBoardThreads; len(threads) != want {
		t.Fatalf("thread count = %d, want %d", len(threads), want)
	}

	for _, thread := range threads {
		if thread.Title == fullThreadTitle {
			t.Errorf("the thread %q was generated, want no thread that has reached the limit", thread.Title)
		}
		if thread.Title == referenceScript.title || thread.Title == withdrawnScript.title {
			t.Errorf("the written-out thread %q was generated, want only the ordinary ones", thread.Title)
		}

		posts := postsOf(t, db, ctx, thread.ID)
		if len(posts) < coldStartContentPlan.minPostsPerThread || len(posts) > coldStartContentPlan.maxPostsPerThread {
			t.Errorf(
				"the thread %q holds %d posts, want between %d and %d",
				thread.Title, len(posts), coldStartContentPlan.minPostsPerThread, coldStartContentPlan.maxPostsPerThread,
			)
		}
		if thread.PostsCount != len(posts) {
			t.Errorf("the thread %q reports %d posts, want %d", thread.Title, thread.PostsCount, len(posts))
		}

		// A board whose newest thread was last written in weeks ago says that
		// nobody is there, which is the one thing the first days must not be
		// generated as (ADR 0010).
		//
		// [Ja] 最新のスレッドが数週間前にしか書かれていない掲示板は、そこに誰も
		// いないことを述べます。立ち上げ直後を、それとして生成してはなりません
		// (ADR 0010)。
		if age := time.Since(thread.LastPostedAt); age > coldStartContentPlan.span {
			t.Errorf(
				"the thread %q was last posted in %s ago, want no more than %s",
				thread.Title, age.Round(time.Hour), coldStartContentPlan.span,
			)
		}
	}
}
