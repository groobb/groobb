package usecase_test

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
)

// newGetCategoryBoardsUsecase builds the UseCase over a database the test owns,
// returning the repositories alongside it so the test can arrange the rows it is
// about to read back.
//
// [Ja] newGetCategoryBoardsUsecase はテストが所有するデータベース上に UseCase を構築し、
// これから読み戻す行をテストが用意できるよう、リポジトリも併せて返します。
func newGetCategoryBoardsUsecase(t *testing.T) (*usecase.GetCategoryBoardsUsecase, *repository.CategoryRepository, *repository.BoardRepository) {
	t.Helper()

	db := testutil.SetupDB(t)
	categoryRepo := repository.NewCategoryRepository(db)
	boardRepo := repository.NewBoardRepository(db)

	return usecase.NewGetCategoryBoardsUsecase(boardRepo), categoryRepo, boardRepo
}

// TestGetCategoryBoardsUsecase_Execute verifies that Execute returns the given
// category's boards in the order the community placed them, leaving out the
// boards of every other category. The boards are created in the reverse of their
// intended order and a board of a second category is created alongside them, so
// the assertion reads the position and the grouping rather than the insertion
// order.
//
// [Ja] TestGetCategoryBoardsUsecase_Execute は、Execute が指定したカテゴリーの掲示板を
// コミュニティが並べた順で返すこと、そして他のカテゴリーの掲示板を含めないことを検証
// します。掲示板は意図した順序と逆に作り、別のカテゴリーの掲示板も併せて作ります。
// 検証が挿入順ではなく position と束ね方を読んでいることを示すためです。
func TestGetCategoryBoardsUsecase_Execute(t *testing.T) {
	t.Parallel()

	uc, categoryRepo, boardRepo := newGetCategoryBoardsUsecase(t)
	ctx := context.Background()

	music, err := categoryRepo.Create(ctx, repository.CreateCategoryInput{Slug: "music", Name: "音楽", Position: 1})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	hobby, err := categoryRepo.Create(ctx, repository.CreateCategoryInput{Slug: "hobby", Name: "趣味", Position: 2})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	createBoard := func(categoryID model.CategoryID, slug, name string, position int) {
		t.Helper()
		if _, err := boardRepo.Create(ctx, repository.CreateBoardInput{
			CategoryID:  &categoryID,
			Slug:        slug,
			Name:        name,
			Description: name + "の板",
			Position:    position,
		}); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
	}
	createBoard(music.ID, "rock", "ロック", 2)
	createBoard(music.ID, "jazz", "ジャズ", 1)
	createBoard(hobby.ID, "games", "ゲーム", 1)

	output, err := uc.Execute(ctx, usecase.GetCategoryBoardsInput{CategoryID: music.ID})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	wantBoards := []string{"ジャズ", "ロック"}
	if len(output.Boards) != len(wantBoards) {
		t.Fatalf("len(output.Boards) = %d, want %d", len(output.Boards), len(wantBoards))
	}
	for i, want := range wantBoards {
		if output.Boards[i].Name != want {
			t.Errorf("output.Boards[%d].Name = %q, want %q", i, output.Boards[i].Name, want)
		}
	}
}

// TestGetCategoryBoardsUsecase_Execute_EmptyCategory verifies that a category
// holding no board yields an empty listing rather than an error: the community
// placed the category and has yet to place a board in it, which is a state its
// page has to render.
//
// [Ja] TestGetCategoryBoardsUsecase_Execute_EmptyCategory は、掲示板を 1 つも持たない
// カテゴリーがエラーではなく空の一覧になることを検証します。コミュニティがカテゴリーを
// 置き、まだそこに掲示板を置いていない状態であり、そのページが描画しなければならない
// 状態だからです。
func TestGetCategoryBoardsUsecase_Execute_EmptyCategory(t *testing.T) {
	t.Parallel()

	uc, categoryRepo, _ := newGetCategoryBoardsUsecase(t)
	ctx := context.Background()

	empty, err := categoryRepo.Create(ctx, repository.CreateCategoryInput{Slug: "empty", Name: "準備中", Position: 1})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	output, err := uc.Execute(ctx, usecase.GetCategoryBoardsInput{CategoryID: empty.ID})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(output.Boards) != 0 {
		t.Errorf("len(output.Boards) = %d, want 0", len(output.Boards))
	}
}
