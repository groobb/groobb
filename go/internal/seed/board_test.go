package seed

import (
	"context"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
)

// TestRunner_GenerateBoardsは、シードが記述するコミュニティが、そこで名指しされた
// カテゴリーと掲示板として、記述の順序が与えるpositionに置かれた形で書き込まれること、
// カテゴリーを書かずに記述した掲示板がカテゴリーを持たない形で保存されること、そして
// それらの掲示板が、それを埋める生成器へ引き渡されることを検証します。
func TestRunner_GenerateBoards(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testutil.SetupDB(t)
	st := &state{roster: testRoster()}

	tx := beginTx(t, db)
	if err := newTestRunner(db).generateBoards(ctx, tx, st); err != nil {
		t.Fatalf("generateBoards()のエラー = %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("トランザクションのコミットに失敗: %v", err)
	}

	categoryRepo := repository.NewCategoryRepository(db)
	for i, want := range matureCategories {
		category, err := categoryRepo.FindBySlug(ctx, want.slug)
		if err != nil {
			t.Fatalf("FindBySlug()のエラー = %v", err)
		}
		if category == nil {
			t.Fatalf("カテゴリー %q が作成されていない", want.slug)
		}
		if category.Name != want.name {
			t.Errorf("カテゴリー %q の名前 = %q、期待値 = %q", want.slug, category.Name, want.name)
		}
		if category.Position != i {
			t.Errorf("カテゴリー %q のposition = %d、期待値 = %d", want.slug, category.Position, i)
		}
	}

	// 掲示板はサイドバーが描くフラットな順序で返る。それは記述がそれらを書いている
	// 順序である。
	boards, err := repository.NewBoardRepository(db).ListAll(ctx)
	if err != nil {
		t.Fatalf("ListAll()のエラー = %v", err)
	}
	if len(boards) != len(matureBoards) {
		t.Fatalf("掲示板の件数 = %d、期待値 = %d", len(boards), len(matureBoards))
	}

	for position, want := range matureBoards {
		board := boards[position]
		if board.Slug != want.slug {
			t.Errorf("%d 番目の掲示板のslug = %q、期待値 = %q", position, board.Slug, want.slug)
		}
		if board.Position != position {
			t.Errorf("掲示板 %q のposition = %d、期待値 = %d", board.Slug, board.Position, position)
		}
		if board.Name != want.name || board.Description != want.description {
			t.Errorf("掲示板 %q の (名前, 説明) = (%q, %q)、期待値 = (%q, %q)", board.Slug, board.Name, board.Description, want.name, want.description)
		}

		if want.categorySlug == "" {
			if board.CategoryID != nil {
				t.Errorf("掲示板 %q のカテゴリー = %v、期待値 = nil", board.Slug, board.CategoryID)
			}
			continue
		}

		category, err := categoryRepo.FindBySlug(ctx, want.categorySlug)
		if err != nil {
			t.Fatalf("FindBySlug()のエラー = %v", err)
		}
		if board.CategoryID == nil || *board.CategoryID != category.ID {
			t.Errorf("掲示板 %q のカテゴリー = %v、期待値 = %v", board.Slug, board.CategoryID, category.ID)
		}
	}

	if len(st.boards) != len(matureBoards) {
		t.Fatalf("生成器が引き渡した掲示板の件数 = %d、期待値 = %d", len(st.boards), len(matureBoards))
	}
	for i, seeded := range st.boards {
		if seeded.board.Slug != boards[i].Slug {
			t.Errorf("%d 番目に引き渡された掲示板 = %q、期待値 = %q", i, seeded.board.Slug, boards[i].Slug)
		}
	}
}

// TestSeedBoards_DescribeEveryStateは、コミュニティがそれぞれの賑わいの掲示板を
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
			t.Errorf("slug %q が複数の掲示板で使われている", board.slug)
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
			t.Errorf("活動量 %d で書き下された掲示板が無い", activity)
		}
	}
	if found[boardBusy] != 1 {
		t.Errorf("活動量がbusyの掲示板の件数 = %d、期待値 = 1", found[boardBusy])
	}
	if categorized == 0 {
		t.Error("カテゴリーに属するものとして書き下された掲示板が無い")
	}
	if uncategorized == 0 {
		t.Error("どのカテゴリーにも属さないものとして書き下された掲示板が無い")
	}
}

// TestValidateSeedSlugsは、パスヘルパーがアドレスへ置けないslugや、掲示板が
// 名指しているのに書き下されていないカテゴリーが、書き込む前に実行を止め、問題の箇所を
// 名指しすることを検証します。このガードは後から本ファイルへ追加される行のためにあるため、
// 通過する書き下しのコミュニティだけでなく、ファイルが持たないデータでも動かします。
func TestValidateSeedSlugs(t *testing.T) {
	t.Parallel()

	if err := validateSeedSlugs(matureCategories, matureBoards); err != nil {
		t.Errorf("validateSeedSlugs(matureCategories, matureBoards)のエラー = %v、期待値 = nil", err)
	}

	tests := []struct {
		name       string
		categories []seedCategory
		boards     []seedBoard
		wantInErr  string
	}{
		{
			name:       "カテゴリーのslugが不正",
			categories: []seedCategory{{slug: "hobby/games", name: "趣味"}},
			wantInErr:  "hobby/games",
		},
		{
			name:       "カテゴリーのslugに大文字を含む",
			categories: []seedCategory{{slug: "Hobby", name: "趣味"}},
			wantInErr:  "Hobby",
		},
		{
			name:       "掲示板のslugが不正",
			categories: []seedCategory{{slug: "hobby", name: "趣味"}},
			boards:     []seedBoard{{categorySlug: "hobby", slug: "games?x=1", name: "ゲーム"}},
			wantInErr:  "games?x=1",
		},
		{
			name:       "掲示板のslugに大文字を含む",
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
				t.Fatal("URLにできないslugや未知のカテゴリーに対してvalidateSeedSlugs()が失敗することを期待したが、成功した")
			}
			if !strings.Contains(err.Error(), tt.wantInErr) {
				t.Errorf("validateSeedSlugs()のエラー = %q、%q を名指すことを期待", err, tt.wantInErr)
			}
		})
	}
}
