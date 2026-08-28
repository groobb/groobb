package seed

import (
	"context"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
)

// TestRunner_GenerateBoards verifies that the community the seed describes is
// written as the categories and boards it names, each placed at the position its
// order in the description gives it, that a board described without a category
// is stored without one, and that the boards are handed on to the generator that
// fills them.
//
// [Ja] TestRunner_GenerateBoards は、シードが記述するコミュニティが、そこで名指しされた
// カテゴリーと掲示板として、記述の順序が与える position に置かれた形で書き込まれること、
// カテゴリーを書かずに記述した掲示板がカテゴリーを持たない形で保存されること、そして
// それらの掲示板が、それを埋める生成器へ引き渡されることを検証します。
func TestRunner_GenerateBoards(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testutil.SetupDB(t)
	st := &state{roster: testRoster()}

	tx := beginTx(t, db)
	if err := newTestRunner(db).generateBoards(ctx, tx, st); err != nil {
		t.Fatalf("generateBoards() error = %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit the transaction: %v", err)
	}

	categoryRepo := repository.NewCategoryRepository(db)
	for i, want := range matureCategories {
		category, err := categoryRepo.FindBySlug(ctx, want.slug)
		if err != nil {
			t.Fatalf("FindBySlug() error = %v", err)
		}
		if category == nil {
			t.Fatalf("the category %q was not created", want.slug)
		}
		if category.Name != want.name {
			t.Errorf("category %q name = %q, want %q", want.slug, category.Name, want.name)
		}
		if category.Position != i {
			t.Errorf("category %q position = %d, want %d", want.slug, category.Position, i)
		}
	}

	// The boards come back in the flat order the sidebar draws them, which is
	// the order the description writes them in.
	//
	// [Ja] 掲示板はサイドバーが描くフラットな順序で返る。それは記述がそれらを書いている
	// 順序である。
	boards, err := repository.NewBoardRepository(db).ListAll(ctx)
	if err != nil {
		t.Fatalf("ListAll() error = %v", err)
	}
	if len(boards) != len(matureBoards) {
		t.Fatalf("board count = %d, want %d", len(boards), len(matureBoards))
	}

	for position, want := range matureBoards {
		board := boards[position]
		if board.Slug != want.slug {
			t.Errorf("board %d slug = %q, want %q", position, board.Slug, want.slug)
		}
		if board.Position != position {
			t.Errorf("board %q position = %d, want %d", board.Slug, board.Position, position)
		}
		if board.Name != want.name || board.Description != want.description {
			t.Errorf("board %q = (%q, %q), want (%q, %q)", board.Slug, board.Name, board.Description, want.name, want.description)
		}

		if want.categorySlug == "" {
			if board.CategoryID != nil {
				t.Errorf("board %q category = %v, want nil", board.Slug, board.CategoryID)
			}
			continue
		}

		category, err := categoryRepo.FindBySlug(ctx, want.categorySlug)
		if err != nil {
			t.Fatalf("FindBySlug() error = %v", err)
		}
		if board.CategoryID == nil || *board.CategoryID != category.ID {
			t.Errorf("board %q category = %v, want %v", board.Slug, board.CategoryID, category.ID)
		}
	}

	if len(st.boards) != len(matureBoards) {
		t.Fatalf("the generator handed on %d boards, want %d", len(st.boards), len(matureBoards))
	}
	for i, seeded := range st.boards {
		if seeded.board.Slug != boards[i].Slug {
			t.Errorf("the board handed on at %d is %q, want %q", i, seeded.board.Slug, boards[i].Slug)
		}
	}
}

// TestSeedBoards_DescribeEveryState verifies that the community holds a board of
// each activity, and holds both a board that a category lists and a board that
// none does. A busy board is where the threads written to be opened are posted,
// an empty one is the only place an empty state can be looked at, and a board
// outside every category is the only place the sidebar's mixed listing (ADR
// 0011) can be looked at. A description that dropped any of them would leave a
// screen unreachable.
//
// [Ja] TestSeedBoards_DescribeEveryState は、コミュニティがそれぞれの賑わいの掲示板を
// 持つこと、そしてカテゴリーが並べる掲示板とどのカテゴリーも並べない掲示板の双方を持つ
// ことを検証します。賑わう掲示板は開いて眺めるために書き下したスレッドが立つ場所であり、
// 空の掲示板は空状態を眺められる唯一の場所であり、どのカテゴリーにも属さない掲示板は
// サイドバーの混ざった一覧 (ADR 0011) を眺められる唯一の手立てです。どれかを落とした
// 記述は、辿り着けない画面を残すことになります。
func TestSeedBoards_DescribeEveryState(t *testing.T) {
	t.Parallel()

	found := make(map[boardActivity]int)
	slugs := make(map[string]bool)
	categorized, uncategorized := 0, 0
	for _, board := range matureBoards {
		found[board.activity]++
		if slugs[board.slug] {
			t.Errorf("the slug %q is used by more than one board", board.slug)
		}
		slugs[board.slug] = true

		if board.categorySlug == "" {
			uncategorized++
		} else {
			categorized++
		}
	}

	for _, activity := range []boardActivity{boardEmpty, boardQuiet, boardBusy} {
		if found[activity] == 0 {
			t.Errorf("no board is described with the activity %d", activity)
		}
	}
	if found[boardBusy] != 1 {
		t.Errorf("board count with the busy activity = %d, want 1", found[boardBusy])
	}
	if categorized == 0 {
		t.Error("no board is described as belonging to a category")
	}
	if uncategorized == 0 {
		t.Error("no board is described as belonging to no category")
	}
}

// TestValidateSeedSlugs verifies that a slug the path helpers could not place
// into an address, or a category a board names but that is not written, stops a
// run before it writes and names what is at fault. The guard exists for a row
// added to this file later, so it is exercised here with data the file does not
// hold rather than only on the written-out community, which passes.
//
// [Ja] TestValidateSeedSlugs は、パスヘルパーがアドレスへ置けない slug や、掲示板が
// 名指しているのに書き下されていないカテゴリーが、書き込む前に実行を止め、問題の箇所を
// 名指しすることを検証します。このガードは後から本ファイルへ追加される行のためにあるため、
// 通過する書き下しのコミュニティだけでなく、ファイルが持たないデータでも動かします。
func TestValidateSeedSlugs(t *testing.T) {
	t.Parallel()

	if err := validateSeedSlugs(matureCategories, matureBoards); err != nil {
		t.Errorf("validateSeedSlugs(matureCategories, matureBoards) error = %v, want nil", err)
	}

	tests := []struct {
		name       string
		categories []seedCategory
		boards     []seedBoard
		wantInErr  string
	}{
		{
			name:       "カテゴリーの slug が不正",
			categories: []seedCategory{{slug: "hobby/games", name: "趣味"}},
			wantInErr:  "hobby/games",
		},
		{
			name:       "カテゴリーの slug に大文字を含む",
			categories: []seedCategory{{slug: "Hobby", name: "趣味"}},
			wantInErr:  "Hobby",
		},
		{
			name:       "掲示板の slug が不正",
			categories: []seedCategory{{slug: "hobby", name: "趣味"}},
			boards:     []seedBoard{{categorySlug: "hobby", slug: "games?x=1", name: "ゲーム"}},
			wantInErr:  "games?x=1",
		},
		{
			name:       "掲示板の slug に大文字を含む",
			categories: []seedCategory{{slug: "hobby", name: "趣味"}},
			boards:     []seedBoard{{categorySlug: "hobby", slug: "Games", name: "ゲーム"}},
			wantInErr:  "Games",
		},
		{
			name:       "掲示板が書き下されていないカテゴリーを名指している",
			categories: []seedCategory{{slug: "hobby", name: "趣味"}},
			boards:     []seedBoard{{categorySlug: "hoby", slug: "games", name: "ゲーム"}},
			wantInErr:  "hoby",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateSeedSlugs(tt.categories, tt.boards)
			if err == nil {
				t.Fatal("validateSeedSlugs() should fail on an unaddressable slug or an unknown category, but it succeeded")
			}
			if !strings.Contains(err.Error(), tt.wantInErr) {
				t.Errorf("validateSeedSlugs() error = %q, want it to name %q", err, tt.wantInErr)
			}
		})
	}
}
