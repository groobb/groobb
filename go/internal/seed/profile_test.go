package seed

import (
	"slices"
	"testing"
	"time"

	"github.com/groobb/groobb/go/internal/testutil"
)

// TestFindProfileは、コマンドラインがここに書かれた状態へ名前で辿り着けること、
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
				t.Fatalf("FindProfile(%q)のok = %v、期待値 = %v", tt.name, ok, tt.want)
			}
			if !tt.want {
				return
			}
			if profile.Name() != tt.name {
				t.Errorf("FindProfile(%q).Name() = %q、期待値 = %q", tt.name, profile.Name(), tt.name)
			}
			if len(profile.boards) != tt.boards {
				t.Errorf("FindProfile(%q)の掲示板の件数 = %d、期待値 = %d", tt.name, len(profile.boards), tt.boards)
			}
		})
	}
}

// TestProfileNamesは、名前を書き込むusageの行が、FindProfileが応じるものを
// 提示していること、そして既定がその中にあることを検証します。行が落とした名前は誰にも
// 見つけられず、行が提示しても引きが拒む名前は、開発者を失敗するコマンドへ送ることに
// なります。
func TestProfileNames(t *testing.T) {
	t.Parallel()

	names := ProfileNames()

	if len(names) != len(profiles) {
		t.Fatalf("ProfileNames() = %v、期待値 = %d 件の名前", names, len(profiles))
	}
	for _, name := range names {
		if _, ok := FindProfile(name); !ok {
			t.Errorf("ProfileNames()がFindProfileの応じない %q を示している", name)
		}
	}
	if want := DefaultProfile().Name(); !slices.Contains(names, want) {
		t.Errorf("ProfileNames() = %v、既定の %q を含むことを期待", names, want)
	}
}

// TestProfiles_SpreadTheirThreadsOverTimeは、どのプロファイルもスレッドを過去に
// 置くことを検証します。spanを書かずに作られたplanは、そのすべてに実行した瞬間の時刻を
// 押すため、どの行も同じ値を持つ列で並んだスレッド一覧と、どのスレッドも今しがた投稿された
// ように読める掲示板を残します。
func TestProfiles_SpreadTheirThreadsOverTime(t *testing.T) {
	t.Parallel()

	for _, profile := range profiles {
		if profile.plan.span <= 0 {
			t.Errorf("プロファイル %q の期間 = %s、正の値を期待", profile.name, profile.plan.span)
		}
	}
}

// TestProfiles_NameTheirCommunityは、どのプロファイルも生成するコミュニティの
// 名前を持つこと、そして2つが同じ名前を持たないことを検証します。名前はサイドバーの
// 見出しと各ページのタイトルの接尾辞に出るものであり、名前の無いプロファイルは、どこでも
// 自身を名乗らないコミュニティを生成します。同じ名前を共有すれば、名前が読まれる場所で
// 生成された状態を見分けられなくなります。
func TestProfiles_NameTheirCommunity(t *testing.T) {
	t.Parallel()

	namedBy := make(map[string]string, len(profiles))
	for _, profile := range profiles {
		if profile.communityName == "" {
			t.Errorf("プロファイル %q がコミュニティ名を持たない", profile.name)

			continue
		}
		if other, exists := namedBy[profile.communityName]; exists {
			t.Errorf("プロファイル %q と %q がどちらもコミュニティ名を %q にしている", other, profile.name, profile.communityName)

			continue
		}
		namedBy[profile.communityName] = profile.name
	}
}

// TestRunner_GenerateThreads_ColdStartは、cold-startプロファイルがインスタンスの
// 開くときの状態を生むことを検証します。カテゴリーを持たない掲示板1つ、そこに立つ
// 数本のスレッド、各スレッドの数件の投稿です。ADR 0010が画面を確かめる先として求める
// のがこの状態であるため、コミュニティが何ヶ月もかけて蓄積するもの (投稿数の上限に
// 達したスレッドや、書き下したスレッドが見せるやり取り) が現れてはなりません。英語の
// スレッドだけは例外です。両方の言語が並ぶ掲示板は、ラウンジが蓄積するものではなく、
// 開いた時点から持つものであるためです。
func TestRunner_GenerateThreads_ColdStart(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	st, ctx := generateContent(t, db, coldStartProfile)

	if len(st.boards) != 1 {
		t.Fatalf("掲示板の件数 = %d、期待値 = 1", len(st.boards))
	}

	board := st.boards[0].board
	if board.CategoryID != nil {
		t.Errorf("掲示板 %q のカテゴリー = %v、期待値は無し", board.Slug, *board.CategoryID)
	}

	var categoryCount int
	if err := db.Reader.QueryRowContext(ctx, "SELECT COUNT(*) FROM categories").Scan(&categoryCount); err != nil {
		t.Fatalf("カテゴリーの件数の取得に失敗: %v", err)
	}
	if categoryCount != 0 {
		t.Errorf("カテゴリーの件数 = %d、期待値 = 0", categoryCount)
	}

	threads := threadsOf(t, db, ctx, board.ID)
	if want := coldStartContentPlan.quietBoardThreads + writtenThreadCount(coldStartProfile); len(threads) != want {
		t.Fatalf("スレッドの件数 = %d、期待値 = %d", len(threads), want)
	}

	// インスタンスが開くときに持つ掲示板は、すでに2つの言語で読めます。そのため
	// 英語を持ち込むスレッドは、上の行のなかに無ければなりません。
	english := findThread(t, threads, englishScript.title)
	if english.Language != englishScript.language {
		t.Errorf("スレッド %q の言語 = %q、期待値 = %q", english.Title, english.Language, englishScript.language)
	}

	for _, thread := range threads {
		if thread.Title == fullThreadTitle {
			t.Errorf("スレッド %q が生成された (上限に達したスレッドは生成しないことを期待)", thread.Title)
		}
		if thread.Title == referenceScript.title || thread.Title == withdrawnScript.title || thread.Title == otherLanguageScript.title {
			t.Errorf("書き下したスレッド %q が生成された (matureのコミュニティに任せることを期待)", thread.Title)
		}

		posts := postsOf(t, db, ctx, thread.ID)

		// 書き下したスレッドがいくつの投稿を持つのかは、その台本が述べることです。
		// そのためplanの上下限は通常のスレッドにだけ問います。
		if thread.Title != englishScript.title &&
			(len(posts) < coldStartContentPlan.minPostsPerThread || len(posts) > coldStartContentPlan.maxPostsPerThread) {
			t.Errorf(
				"スレッド %q の投稿数 = %d、期待値 = %d 以上 %d 以下",
				thread.Title, len(posts), coldStartContentPlan.minPostsPerThread, coldStartContentPlan.maxPostsPerThread,
			)
		}
		if thread.PostsCount != len(posts) {
			t.Errorf("スレッド %q のPostsCount = %d、期待値 = %d", thread.Title, thread.PostsCount, len(posts))
		}

		// 最新のスレッドが数週間前にしか書かれていない掲示板は、そこに誰も
		// いないことを述べます。立ち上げ直後を、それとして生成してはなりません
		// (ADR 0010)。
		if age := time.Since(thread.LastPostedAt); age > coldStartContentPlan.span {
			t.Errorf(
				"スレッド %q の最終投稿からの経過時間 = %s、期待値 = %s 以内",
				thread.Title, age.Round(time.Hour), coldStartContentPlan.span,
			)
		}
	}
}
