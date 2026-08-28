package repository_test

import (
	"context"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
)

// newBoardRepo builds a BoardRepository over the database the test owns,
// together with the category repository the boards need an owning category from.
//
// [Ja] newBoardRepo はテストが所有するデータベース上に BoardRepository を作る。
// 掲示板が属するカテゴリーを用意するためのカテゴリーリポジトリも併せて返す。
func newBoardRepo(t *testing.T) (*repository.BoardRepository, *repository.CategoryRepository, context.Context) {
	t.Helper()
	db := testutil.SetupDB(t)
	return repository.NewBoardRepository(db), repository.NewCategoryRepository(db), context.Background()
}

// createBoard inserts a board in the given category, failing the test on error.
// A nil categoryID inserts a board sitting in no category.
//
// [Ja] createBoard は指定したカテゴリーに掲示板を挿入し、エラー時はテストを失敗させる。
// categoryID が nil の場合は、どのカテゴリーにも属さない掲示板を挿入する。
func createBoard(t *testing.T, ctx context.Context, repo *repository.BoardRepository, categoryID *model.CategoryID, slug string, position int) *model.Board {
	t.Helper()

	board, err := repo.Create(ctx, repository.CreateBoardInput{
		CategoryID: categoryID,
		Slug:       slug,
		Name:       "掲示板 " + slug,
		Position:   position,
	})
	if err != nil {
		t.Fatalf("テスト用掲示板の作成に失敗: %v", err)
	}

	return board
}

func TestBoardRepository_Create(t *testing.T) {
	t.Parallel()

	repo, categoryRepo, ctx := newBoardRepo(t)
	category := createCategory(t, ctx, categoryRepo, "general", 0)

	board, err := repo.Create(ctx, repository.CreateBoardInput{
		CategoryID:  &category.ID,
		Slug:        "tech",
		Name:        "技術",
		Description: "技術の話をする板",
		Position:    2,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if board.ID == 0 {
		t.Error("Create() board.ID は DB 採番で空でないはず")
	}
	if board.CategoryID == nil || *board.CategoryID != category.ID {
		t.Errorf("board.CategoryID = %v, want %v", board.CategoryID, category.ID)
	}
	if board.Slug != "tech" {
		t.Errorf("board.Slug = %q, want %q", board.Slug, "tech")
	}
	if board.Name != "技術" {
		t.Errorf("board.Name = %q, want %q", board.Name, "技術")
	}
	if board.Description != "技術の話をする板" {
		t.Errorf("board.Description = %q, want %q", board.Description, "技術の話をする板")
	}
	if board.Position != 2 {
		t.Errorf("board.Position = %d, want %d", board.Position, 2)
	}
	if board.CreatedAt.IsZero() {
		t.Error("board.CreatedAt は DB 既定値で設定されるはず")
	}
	if board.UpdatedAt.IsZero() {
		t.Error("board.UpdatedAt は DB 既定値で設定されるはず")
	}
}

// TestBoardRepository_Create_RejectsInvalidSlug verifies that a slug the address
// rule does not accept is refused instead of stored, for the reason
// TestCategoryRepository_Create_RejectsInvalidSlug documents: the schema keeps
// the spelling unique but not lowercase, and BoardPath places whatever is stored
// into the path as written.
//
// [Ja] TestBoardRepository_Create_RejectsInvalidSlug は、アドレスの規則が受理しない
// slug が保存されずに拒否されることを検証する。理由は
// TestCategoryRepository_Create_RejectsInvalidSlug が記すとおりで、スキーマは綴りを
// 一意には保つが小文字には保たず、BoardPath は保存されている綴りをそのままパスへ置く。
func TestBoardRepository_Create_RejectsInvalidSlug(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		slug string
	}{
		{name: "大文字を含む", slug: "Games"},
		{name: "クエリの開始文字を含む", slug: "games?x=1"},
		{name: "空文字", slug: ""},
		{name: "最大長超過", slug: strings.Repeat("a", model.SlugMaxLength+1)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo, categoryRepo, ctx := newBoardRepo(t)
			category := createCategory(t, ctx, categoryRepo, "hobby", 1)

			board, err := repo.Create(ctx, repository.CreateBoardInput{
				CategoryID: &category.ID,
				Slug:       tt.slug,
				Name:       "ゲーム",
			})
			if err == nil {
				t.Fatalf("Create() error = nil, want error (slug=%q)", tt.slug)
			}
			if board != nil {
				t.Errorf("Create() board = %+v, want nil", board)
			}

			stored, err := repo.ListAll(ctx)
			if err != nil {
				t.Fatalf("ListAll() error = %v", err)
			}
			if len(stored) != 0 {
				t.Errorf("len(ListAll()) = %d, want 0 (拒否された slug が保存されている)", len(stored))
			}
		})
	}
}

func TestBoardRepository_Create_LeavesDescriptionEmptyWhenUnset(t *testing.T) {
	t.Parallel()

	repo, categoryRepo, ctx := newBoardRepo(t)
	category := createCategory(t, ctx, categoryRepo, "general", 0)

	board := createBoard(t, ctx, repo, &category.ID, "tech", 0)

	if board.Description != "" {
		t.Errorf("board.Description = %q, want %q", board.Description, "")
	}
}

// TestBoardRepository_Create_WithoutACategory verifies that a board can be
// created outside every category and reads back saying so. Belonging to no
// category is a normal state rather than a gap (ADR 0011), and it is the state a
// community that has never made a category leaves all of its boards in.
//
// [Ja] TestBoardRepository_Create_WithoutACategory は、どのカテゴリーにも属さない形で
// 掲示板を作成でき、そう述べる形で読み戻せることを検証する。どのカテゴリーにも属さない
// ことは欠落ではなく正常な状態であり (ADR 0011)、カテゴリーを一度も作っていない
// コミュニティは、すべての掲示板をその状態に置く。
func TestBoardRepository_Create_WithoutACategory(t *testing.T) {
	t.Parallel()

	repo, _, ctx := newBoardRepo(t)

	created := createBoard(t, ctx, repo, nil, "tech", 0)
	if created.CategoryID != nil {
		t.Errorf("Create() board.CategoryID = %v, want nil", created.CategoryID)
	}

	board, err := repo.FindBySlug(ctx, "tech")
	if err != nil {
		t.Fatalf("FindBySlug() error = %v", err)
	}
	if board == nil {
		t.Fatal("FindBySlug() = nil, want board")
	}
	if board.CategoryID != nil {
		t.Errorf("FindBySlug() board.CategoryID = %v, want nil", board.CategoryID)
	}
}

// TestBoardRepository_FindByID verifies the lookup a thread's page resolves its
// board through: a thread names its board by id, not by the slug the board's own
// address carries.
//
// [Ja] TestBoardRepository_FindByID は、スレッドのページが掲示板を解決するときの
// ルックアップを検証する。スレッドは自身の掲示板を、掲示板自身のアドレスが運ぶ slug では
// なく id で名指すためである。
func TestBoardRepository_FindByID(t *testing.T) {
	t.Parallel()

	repo, categoryRepo, ctx := newBoardRepo(t)
	category := createCategory(t, ctx, categoryRepo, "general", 0)
	created := createBoard(t, ctx, repo, &category.ID, "tech", 0)

	t.Run("id で掲示板を取得できる", func(t *testing.T) {
		board, err := repo.FindByID(ctx, created.ID)
		if err != nil {
			t.Fatalf("FindByID() error = %v", err)
		}
		if board == nil {
			t.Fatal("FindByID() = nil, want board")
		}
		if board.Slug != "tech" {
			t.Errorf("board.Slug = %q, want %q", board.Slug, "tech")
		}
		if board.CategoryID == nil || *board.CategoryID != category.ID {
			t.Errorf("board.CategoryID = %v, want %v", board.CategoryID, category.ID)
		}
	})

	t.Run("存在しない id は (nil, nil) を返す", func(t *testing.T) {
		board, err := repo.FindByID(ctx, created.ID+1000)
		if err != nil {
			t.Fatalf("FindByID() error = %v, want nil", err)
		}
		if board != nil {
			t.Errorf("FindByID() = %v, want nil", board)
		}
	})
}

func TestBoardRepository_FindBySlug(t *testing.T) {
	t.Parallel()

	repo, categoryRepo, ctx := newBoardRepo(t)
	category := createCategory(t, ctx, categoryRepo, "general", 0)
	created := createBoard(t, ctx, repo, &category.ID, "tech", 0)

	t.Run("slug で掲示板を取得できる", func(t *testing.T) {
		board, err := repo.FindBySlug(ctx, "tech")
		if err != nil {
			t.Fatalf("FindBySlug() error = %v", err)
		}
		if board == nil {
			t.Fatal("FindBySlug() = nil, want board")
		}
		if board.ID != created.ID {
			t.Errorf("board.ID = %v, want %v", board.ID, created.ID)
		}
		if board.CategoryID == nil || *board.CategoryID != category.ID {
			t.Errorf("board.CategoryID = %v, want %v", board.CategoryID, category.ID)
		}
	})

	t.Run("大文字小文字が違う slug でも同じ掲示板を取得できる", func(t *testing.T) {
		board, err := repo.FindBySlug(ctx, "TECH")
		if err != nil {
			t.Fatalf("FindBySlug() error = %v", err)
		}
		if board == nil {
			t.Fatal("FindBySlug() = nil, want board")
		}
		if board.ID != created.ID {
			t.Errorf("board.ID = %v, want %v", board.ID, created.ID)
		}
	})

	t.Run("存在しない slug は (nil, nil) を返す", func(t *testing.T) {
		board, err := repo.FindBySlug(ctx, "unknown")
		if err != nil {
			t.Fatalf("FindBySlug() error = %v, want nil", err)
		}
		if board != nil {
			t.Errorf("FindBySlug() = %v, want nil", board)
		}
	})
}

// TestBoardRepository_ListAll verifies the listing the sidebar is drawn from:
// every board of the community in position order, whether or not it sits in a
// category. The sidebar lists them flat (ADR 0011), so a board left outside
// every category has to come back alongside the ones that have one.
//
// [Ja] TestBoardRepository_ListAll は、サイドバーの描画元となる一覧を検証する。
// コミュニティのすべての掲示板を、カテゴリーに属するかどうかによらず position 順で返す。
// サイドバーはそれらをフラットに並べるため (ADR 0011)、どのカテゴリーにも属さない掲示板も
// 属するものと並んで返らなければならない。
func TestBoardRepository_ListAll(t *testing.T) {
	t.Parallel()

	repo, categoryRepo, ctx := newBoardRepo(t)
	general := createCategory(t, ctx, categoryRepo, "general", 0)
	tech := createCategory(t, ctx, categoryRepo, "tech", 1)

	// The insertion order crosses categories and positions, so a result that
	// merely echoes it cannot pass.
	//
	// [Ja] 挿入順はカテゴリーと position をまたいで交差させてあり、挿入順をそのまま
	// 返すだけの結果では通らないようにしている。
	createBoard(t, ctx, repo, &tech.ID, "go", 3)
	createBoard(t, ctx, repo, nil, "questions", 1)
	createBoard(t, ctx, repo, &general.ID, "chat", 0)
	createBoard(t, ctx, repo, &tech.ID, "sqlite", 2)

	boards, err := repo.ListAll(ctx)
	if err != nil {
		t.Fatalf("ListAll() error = %v", err)
	}

	wantSlugs := []string{"chat", "questions", "sqlite", "go"}
	if len(boards) != len(wantSlugs) {
		t.Fatalf("len(ListAll()) = %d, want %d", len(boards), len(wantSlugs))
	}
	for i, want := range wantSlugs {
		if boards[i].Slug != want {
			t.Errorf("ListAll()[%d].Slug = %q, want %q", i, boards[i].Slug, want)
		}
	}

	if boards[1].CategoryID != nil {
		t.Errorf("ListAll()[1].CategoryID = %v, want nil (どのカテゴリーにも属さない掲示板)", boards[1].CategoryID)
	}
}

func TestBoardRepository_ListByCategoryID(t *testing.T) {
	t.Parallel()

	t.Run("渡したカテゴリーの掲示板だけを position 順で返す", func(t *testing.T) {
		t.Parallel()

		repo, categoryRepo, ctx := newBoardRepo(t)
		general := createCategory(t, ctx, categoryRepo, "general", 0)
		excluded := createCategory(t, ctx, categoryRepo, "excluded", 1)

		// The insertion order crosses categories and positions, so a result that
		// merely echoes it cannot pass. The board with no category is here because
		// it must not be picked up by a category's listing either.
		//
		// [Ja] 挿入順はカテゴリーと position をまたいで交差させてあり、挿入順をそのまま
		// 返すだけの結果では通らないようにしている。どのカテゴリーにも属さない掲示板を
		// 置いているのは、それがカテゴリーの一覧に混ざってもならないためである。
		createBoard(t, ctx, repo, &general.ID, "questions", 3)
		createBoard(t, ctx, repo, &excluded.ID, "secret", 1)
		createBoard(t, ctx, repo, nil, "unfiled", 2)
		createBoard(t, ctx, repo, &general.ID, "chat", 0)

		boards, err := repo.ListByCategoryID(ctx, general.ID)
		if err != nil {
			t.Fatalf("ListByCategoryID() error = %v", err)
		}

		wantSlugs := []string{"chat", "questions"}
		if len(boards) != len(wantSlugs) {
			t.Fatalf("len(ListByCategoryID()) = %d, want %d", len(boards), len(wantSlugs))
		}
		for i, want := range wantSlugs {
			if boards[i].Slug != want {
				t.Errorf("ListByCategoryID()[%d].Slug = %q, want %q", i, boards[i].Slug, want)
			}
		}
	})

	t.Run("掲示板を持たないカテゴリーは空を返す", func(t *testing.T) {
		t.Parallel()

		repo, categoryRepo, ctx := newBoardRepo(t)
		empty := createCategory(t, ctx, categoryRepo, "empty", 0)
		createBoard(t, ctx, repo, nil, "unfiled", 0)

		boards, err := repo.ListByCategoryID(ctx, empty.ID)
		if err != nil {
			t.Fatalf("ListByCategoryID() error = %v", err)
		}
		if len(boards) != 0 {
			t.Errorf("len(ListByCategoryID()) = %d, want 0", len(boards))
		}
	})
}
