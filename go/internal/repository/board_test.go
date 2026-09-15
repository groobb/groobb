package repository_test

import (
	"context"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
)

// newBoardRepoはテストが所有するデータベース上にBoardRepositoryを作る。
// 掲示板が属するカテゴリーを用意するためのカテゴリーリポジトリも併せて返す。
func newBoardRepo(t *testing.T) (*repository.BoardRepository, *repository.CategoryRepository, context.Context) {
	t.Helper()
	db := testutil.SetupDB(t)
	return repository.NewBoardRepository(db), repository.NewCategoryRepository(db), context.Background()
}

// createBoardは指定したカテゴリーに掲示板を挿入し、エラー時はテストを失敗させる。
// categoryIDがnilの場合は、どのカテゴリーにも属さない掲示板を挿入する。
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
		t.Fatalf("Create()のエラー = %v", err)
	}

	if board.ID == 0 {
		t.Error("Create() board.IDはDB採番で空でないはず")
	}
	if board.CategoryID == nil || *board.CategoryID != category.ID {
		t.Errorf("board.CategoryID = %v、期待値 = %v", board.CategoryID, category.ID)
	}
	if board.Slug != "tech" {
		t.Errorf("board.Slug = %q、期待値 = %q", board.Slug, "tech")
	}
	if board.Name != "技術" {
		t.Errorf("board.Name = %q、期待値 = %q", board.Name, "技術")
	}
	if board.Description != "技術の話をする板" {
		t.Errorf("board.Description = %q、期待値 = %q", board.Description, "技術の話をする板")
	}
	if board.Position != 2 {
		t.Errorf("board.Position = %d、期待値 = %d", board.Position, 2)
	}
	if board.CreatedAt.IsZero() {
		t.Error("board.CreatedAtはDB既定値で設定されるはず")
	}
	if board.UpdatedAt.IsZero() {
		t.Error("board.UpdatedAtはDB既定値で設定されるはず")
	}
}

// TestBoardRepository_Create_RejectsInvalidSlugは、アドレスの規則が受理しない
// slugが保存されずに拒否されることを検証する。理由は
// TestCategoryRepository_Create_RejectsInvalidSlugが記すとおりで、スキーマは綴りを
// 一意には保つが小文字には保たず、BoardPathは保存されている綴りをそのままパスへ置く。
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
				t.Fatalf("Create()のエラー = nil、期待値はエラー (slug=%q)", tt.slug)
			}
			if board != nil {
				t.Errorf("Create() board = %+v、期待値 = nil", board)
			}

			stored, err := repo.ListAll(ctx)
			if err != nil {
				t.Fatalf("ListAll()のエラー = %v", err)
			}
			if len(stored) != 0 {
				t.Errorf("len(ListAll()) = %d、期待値 = 0 (拒否されたslugが保存されている)", len(stored))
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
		t.Errorf("board.Description = %q、期待値 = %q", board.Description, "")
	}
}

// TestBoardRepository_Create_WithoutACategoryは、どのカテゴリーにも属さない形で
// 掲示板を作成でき、そう述べる形で読み戻せることを検証する。どのカテゴリーにも属さない
// ことは欠落ではなく正常な状態であり (ADR 0011)、カテゴリーを一度も作っていない
// コミュニティは、すべての掲示板をその状態に置く。
func TestBoardRepository_Create_WithoutACategory(t *testing.T) {
	t.Parallel()

	repo, _, ctx := newBoardRepo(t)

	created := createBoard(t, ctx, repo, nil, "tech", 0)
	if created.CategoryID != nil {
		t.Errorf("Create() board.CategoryID = %v、期待値 = nil", created.CategoryID)
	}

	board, err := repo.FindBySlug(ctx, "tech")
	if err != nil {
		t.Fatalf("FindBySlug()のエラー = %v", err)
	}
	if board == nil {
		t.Fatal("FindBySlug() = nil、期待値は掲示板")
	}
	if board.CategoryID != nil {
		t.Errorf("FindBySlug() board.CategoryID = %v、期待値 = nil", board.CategoryID)
	}
}

// TestBoardRepository_FindByIDは、スレッドのページが掲示板を解決するときの
// ルックアップを検証する。スレッドは自身の掲示板を、掲示板自身のアドレスが運ぶslugでは
// なくidで名指すためである。
func TestBoardRepository_FindByID(t *testing.T) {
	t.Parallel()

	repo, categoryRepo, ctx := newBoardRepo(t)
	category := createCategory(t, ctx, categoryRepo, "general", 0)
	created := createBoard(t, ctx, repo, &category.ID, "tech", 0)

	t.Run("idで掲示板を取得できる", func(t *testing.T) {
		board, err := repo.FindByID(ctx, created.ID)
		if err != nil {
			t.Fatalf("FindByID()のエラー = %v", err)
		}
		if board == nil {
			t.Fatal("FindByID() = nil、期待値は掲示板")
		}
		if board.Slug != "tech" {
			t.Errorf("board.Slug = %q、期待値 = %q", board.Slug, "tech")
		}
		if board.CategoryID == nil || *board.CategoryID != category.ID {
			t.Errorf("board.CategoryID = %v、期待値 = %v", board.CategoryID, category.ID)
		}
	})

	t.Run("存在しないidは (nil, nil) を返す", func(t *testing.T) {
		board, err := repo.FindByID(ctx, created.ID+1000)
		if err != nil {
			t.Fatalf("FindByID()のエラー = %v、期待値 = nil", err)
		}
		if board != nil {
			t.Errorf("FindByID() = %v、期待値 = nil", board)
		}
	})
}

func TestBoardRepository_FindBySlug(t *testing.T) {
	t.Parallel()

	repo, categoryRepo, ctx := newBoardRepo(t)
	category := createCategory(t, ctx, categoryRepo, "general", 0)
	created := createBoard(t, ctx, repo, &category.ID, "tech", 0)

	t.Run("slugで掲示板を取得できる", func(t *testing.T) {
		board, err := repo.FindBySlug(ctx, "tech")
		if err != nil {
			t.Fatalf("FindBySlug()のエラー = %v", err)
		}
		if board == nil {
			t.Fatal("FindBySlug() = nil、期待値は掲示板")
		}
		if board.ID != created.ID {
			t.Errorf("board.ID = %v、期待値 = %v", board.ID, created.ID)
		}
		if board.CategoryID == nil || *board.CategoryID != category.ID {
			t.Errorf("board.CategoryID = %v、期待値 = %v", board.CategoryID, category.ID)
		}
	})

	t.Run("大文字小文字が違うslugでも同じ掲示板を取得できる", func(t *testing.T) {
		board, err := repo.FindBySlug(ctx, "TECH")
		if err != nil {
			t.Fatalf("FindBySlug()のエラー = %v", err)
		}
		if board == nil {
			t.Fatal("FindBySlug() = nil、期待値は掲示板")
		}
		if board.ID != created.ID {
			t.Errorf("board.ID = %v、期待値 = %v", board.ID, created.ID)
		}
	})

	t.Run("存在しないslugは (nil, nil) を返す", func(t *testing.T) {
		board, err := repo.FindBySlug(ctx, "unknown")
		if err != nil {
			t.Fatalf("FindBySlug()のエラー = %v、期待値 = nil", err)
		}
		if board != nil {
			t.Errorf("FindBySlug() = %v、期待値 = nil", board)
		}
	})
}

// TestBoardRepository_ListAllは、サイドバーの描画元となる一覧を検証する。
// コミュニティのすべての掲示板を、カテゴリーに属するかどうかによらずposition順で返す。
// サイドバーはそれらをフラットに並べるため (ADR 0011)、どのカテゴリーにも属さない掲示板も
// 属するものと並んで返らなければならない。
func TestBoardRepository_ListAll(t *testing.T) {
	t.Parallel()

	repo, categoryRepo, ctx := newBoardRepo(t)
	general := createCategory(t, ctx, categoryRepo, "general", 0)
	tech := createCategory(t, ctx, categoryRepo, "tech", 1)

	// 挿入順はカテゴリーとpositionをまたいで交差させてあり、挿入順をそのまま
	// 返すだけの結果では通らないようにしている。
	createBoard(t, ctx, repo, &tech.ID, "go", 3)
	createBoard(t, ctx, repo, nil, "questions", 1)
	createBoard(t, ctx, repo, &general.ID, "chat", 0)
	createBoard(t, ctx, repo, &tech.ID, "sqlite", 2)

	boards, err := repo.ListAll(ctx)
	if err != nil {
		t.Fatalf("ListAll()のエラー = %v", err)
	}

	wantSlugs := []string{"chat", "questions", "sqlite", "go"}
	if len(boards) != len(wantSlugs) {
		t.Fatalf("len(ListAll()) = %d、期待値 = %d", len(boards), len(wantSlugs))
	}
	for i, want := range wantSlugs {
		if boards[i].Slug != want {
			t.Errorf("ListAll()[%d].Slug = %q、期待値 = %q", i, boards[i].Slug, want)
		}
	}

	if boards[1].CategoryID != nil {
		t.Errorf("ListAll()[1].CategoryID = %v、期待値 = nil (どのカテゴリーにも属さない掲示板)", boards[1].CategoryID)
	}
}

func TestBoardRepository_ListByCategoryID(t *testing.T) {
	t.Parallel()

	t.Run("渡したカテゴリーの掲示板だけをposition順で返す", func(t *testing.T) {
		t.Parallel()

		repo, categoryRepo, ctx := newBoardRepo(t)
		general := createCategory(t, ctx, categoryRepo, "general", 0)
		excluded := createCategory(t, ctx, categoryRepo, "excluded", 1)

		// 挿入順はカテゴリーとpositionをまたいで交差させてあり、挿入順をそのまま
		// 返すだけの結果では通らないようにしている。どのカテゴリーにも属さない掲示板を
		// 置いているのは、それがカテゴリーの一覧に混ざってもならないためである。
		createBoard(t, ctx, repo, &general.ID, "questions", 3)
		createBoard(t, ctx, repo, &excluded.ID, "secret", 1)
		createBoard(t, ctx, repo, nil, "unfiled", 2)
		createBoard(t, ctx, repo, &general.ID, "chat", 0)

		boards, err := repo.ListByCategoryID(ctx, general.ID)
		if err != nil {
			t.Fatalf("ListByCategoryID()のエラー = %v", err)
		}

		wantSlugs := []string{"chat", "questions"}
		if len(boards) != len(wantSlugs) {
			t.Fatalf("len(ListByCategoryID()) = %d、期待値 = %d", len(boards), len(wantSlugs))
		}
		for i, want := range wantSlugs {
			if boards[i].Slug != want {
				t.Errorf("ListByCategoryID()[%d].Slug = %q、期待値 = %q", i, boards[i].Slug, want)
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
			t.Fatalf("ListByCategoryID()のエラー = %v", err)
		}
		if len(boards) != 0 {
			t.Errorf("len(ListByCategoryID()) = %d、期待値 = 0", len(boards))
		}
	})
}
