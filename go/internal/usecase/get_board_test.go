package usecase_test

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
)

// newGetBoardUsecase builds the UseCase over a database the test owns, returning
// the repositories alongside it so the test can arrange the rows it is about to
// read back.
//
// [Ja] newGetBoardUsecase はテストが所有するデータベース上に UseCase を構築し、これから
// 読み戻す行をテストが用意できるよう、リポジトリも併せて返します。
func newGetBoardUsecase(t *testing.T) (*usecase.GetBoardUsecase, *repository.CategoryRepository, *repository.BoardRepository) {
	t.Helper()

	db := testutil.SetupDB(t)
	categoryRepo := repository.NewCategoryRepository(db)
	boardRepo := repository.NewBoardRepository(db)

	return usecase.NewGetBoardUsecase(boardRepo, categoryRepo), categoryRepo, boardRepo
}

// TestGetBoardUsecase_Execute verifies that Execute resolves the slug to the
// board stored under it, and hands back the category that lists it. A second
// board under a second category is created alongside so the assertion reads the
// slug's resolution and the board's own category rather than whichever row
// happens to come first.
//
// [Ja] TestGetBoardUsecase_Execute は、Execute が slug をその下に保存された掲示板へ
// 解決し、それを並べるカテゴリーを併せて返すことを検証します。2 つ目のカテゴリーの下に
// 2 つ目の掲示板も作るのは、検証が「たまたま最初に来る行」ではなく slug の解決と、その
// 掲示板自身のカテゴリーを読んでいることを示すためです。
func TestGetBoardUsecase_Execute(t *testing.T) {
	t.Parallel()

	uc, categoryRepo, boardRepo := newGetBoardUsecase(t)
	ctx := context.Background()

	music, err := categoryRepo.Create(ctx, repository.CreateCategoryInput{Slug: "music", Name: "音楽", Position: 1})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	hobby, err := categoryRepo.Create(ctx, repository.CreateCategoryInput{Slug: "hobby", Name: "趣味", Position: 2})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if _, err := boardRepo.Create(ctx, repository.CreateBoardInput{
		CategoryID:  &music.ID,
		Slug:        "jazz",
		Name:        "ジャズ",
		Description: "ジャズの話をする板",
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := boardRepo.Create(ctx, repository.CreateBoardInput{
		CategoryID: &hobby.ID,
		Slug:       "games",
		Name:       "ゲーム",
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	output, err := uc.Execute(ctx, usecase.GetBoardInput{Slug: "jazz"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if output.Board.Name != "ジャズ" {
		t.Errorf("output.Board.Name = %q, want %q", output.Board.Name, "ジャズ")
	}
	if output.Board.Description != "ジャズの話をする板" {
		t.Errorf("output.Board.Description = %q, want %q", output.Board.Description, "ジャズの話をする板")
	}
	if output.Category.Slug != "music" {
		t.Errorf("output.Category.Slug = %q, want %q", output.Category.Slug, "music")
	}
	if output.Category.Name != "音楽" {
		t.Errorf("output.Category.Name = %q, want %q", output.Category.Name, "音楽")
	}
}

// TestGetBoardUsecase_Execute_WithoutACategory verifies that a board sitting in
// no category resolves with no category rather than failing. Belonging to none
// is a normal state (ADR 0011), and the page renders it by leaving out the step
// that would have named where the board sits.
//
// [Ja] TestGetBoardUsecase_Execute_WithoutACategory は、どのカテゴリーにも属さない
// 掲示板が、失敗せずにカテゴリー無しで解決されることを検証します。どのカテゴリーにも
// 属さないことは正常な状態であり (ADR 0011)、ページは掲示板の在り処を述べる段を落として
// それを描画します。
func TestGetBoardUsecase_Execute_WithoutACategory(t *testing.T) {
	t.Parallel()

	uc, _, boardRepo := newGetBoardUsecase(t)
	ctx := context.Background()

	if _, err := boardRepo.Create(ctx, repository.CreateBoardInput{Slug: "jazz", Name: "ジャズ"}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	output, err := uc.Execute(ctx, usecase.GetBoardInput{Slug: "jazz"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if output.Board.Name != "ジャズ" {
		t.Errorf("output.Board.Name = %q, want %q", output.Board.Name, "ジャズ")
	}
	if output.Category != nil {
		t.Errorf("output.Category = %+v, want nil", output.Category)
	}
}

// TestGetBoardUsecase_Execute_UnknownSlug verifies that a slug naming no board
// is reported as an AppError carrying AppErrCodeResourceNotFound, which is what
// lets the handler answer 404 instead of rendering an empty page or a 500.
//
// [Ja] TestGetBoardUsecase_Execute_UnknownSlug は、どの掲示板も指さない slug が
// AppErrCodeResourceNotFound を持つ AppError として報告されることを検証します。これに
// より、ハンドラーは空のページや 500 ではなく 404 で応答できます。
func TestGetBoardUsecase_Execute_UnknownSlug(t *testing.T) {
	t.Parallel()

	uc, _, _ := newGetBoardUsecase(t)

	output, err := uc.Execute(context.Background(), usecase.GetBoardInput{Slug: "no-such-board"})
	if output != nil {
		t.Errorf("output = %+v, want nil", output)
	}

	ae := model.AsAppError(err)
	if ae == nil {
		t.Fatalf("Execute() error = %v, want *model.AppError", err)
	}
	if ae.Code != model.AppErrCodeResourceNotFound {
		t.Errorf("ae.Code = %d, want %d (AppErrCodeResourceNotFound)", ae.Code, model.AppErrCodeResourceNotFound)
	}
}

// TestGetBoardUsecase_Execute_UnresolvableCategory verifies that a board naming
// a category that cannot be read back is reported as a plain error rather than
// as an AppError, which is what keeps the handler answering 500 instead of 404.
// Deleting a category clears the naming rather than leaving it pointing at a row
// that is gone, so a board still naming one that cannot be read is an
// inconsistency and not a missing page: answering 404 would tell a crawler to
// drop a board that is still served.
//
// The category repository is pointed at a second, empty database, because the
// foreign key makes this state unreachable through the one holding the board.
//
// [Ja] TestGetBoardUsecase_Execute_UnresolvableCategory は、カテゴリーを名指している
// のにそれを読み戻せない掲示板が、AppError ではなく素のエラーとして報告されることを
// 検証します。これによりハンドラーは 404 ではなく 500 で応答します。カテゴリーの削除は
// 名指しを、消えた行を指したままにするのではなく空にするため、まだ名指しているのに
// 読めない状態はページの不在ではなくデータの不整合です。404 で応答すれば、まだ配信されて
// いる掲示板を落とすようクローラーに伝えてしまいます。
//
// カテゴリーのリポジトリには空の 2 つ目のデータベースを渡します。掲示板を持つほうの
// データベースでは、外部キーによりこの状態に到達できないためです。
func TestGetBoardUsecase_Execute_UnresolvableCategory(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	boardDB := testutil.SetupDB(t)

	music, err := repository.NewCategoryRepository(boardDB).Create(ctx, repository.CreateCategoryInput{Slug: "music", Name: "音楽"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	boardRepo := repository.NewBoardRepository(boardDB)
	if _, err := boardRepo.Create(ctx, repository.CreateBoardInput{
		CategoryID: &music.ID,
		Slug:       "jazz",
		Name:       "ジャズ",
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	uc := usecase.NewGetBoardUsecase(boardRepo, repository.NewCategoryRepository(testutil.SetupDB(t)))

	output, err := uc.Execute(ctx, usecase.GetBoardInput{Slug: "jazz"})
	if output != nil {
		t.Errorf("output = %+v, want nil", output)
	}
	if err == nil {
		t.Fatal("Execute() error = nil, want error")
	}
	if ae := model.AsAppError(err); ae != nil {
		t.Errorf("Execute() error = *model.AppError (Code = %d), want a plain error so the handler answers 500", ae.Code)
	}
}
