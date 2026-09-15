package usecase_test

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
)

// newGetBoardUsecaseはテストが所有するデータベース上にUseCaseを構築し、これから
// 読み戻す行をテストが用意できるよう、リポジトリも併せて返します。
func newGetBoardUsecase(t *testing.T) (*usecase.GetBoardUsecase, *repository.CategoryRepository, *repository.BoardRepository) {
	t.Helper()

	db := testutil.SetupDB(t)
	categoryRepo := repository.NewCategoryRepository(db)
	boardRepo := repository.NewBoardRepository(db)

	return usecase.NewGetBoardUsecase(boardRepo, categoryRepo), categoryRepo, boardRepo
}

// TestGetBoardUsecase_Executeは、Executeがslugをその下に保存された掲示板へ
// 解決し、それを並べるカテゴリーを併せて返すことを検証します。2つ目のカテゴリーの下に
// 2つ目の掲示板も作るのは、検証が「たまたま最初に来る行」ではなくslugの解決と、その
// 掲示板自身のカテゴリーを読んでいることを示すためです。
func TestGetBoardUsecase_Execute(t *testing.T) {
	t.Parallel()

	uc, categoryRepo, boardRepo := newGetBoardUsecase(t)
	ctx := context.Background()

	music, err := categoryRepo.Create(ctx, repository.CreateCategoryInput{Slug: "music", Name: "音楽", Position: 1})
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}
	hobby, err := categoryRepo.Create(ctx, repository.CreateCategoryInput{Slug: "hobby", Name: "趣味", Position: 2})
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}

	if _, err := boardRepo.Create(ctx, repository.CreateBoardInput{
		CategoryID:  &music.ID,
		Slug:        "jazz",
		Name:        "ジャズ",
		Description: "ジャズの話をする板",
	}); err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}
	if _, err := boardRepo.Create(ctx, repository.CreateBoardInput{
		CategoryID: &hobby.ID,
		Slug:       "games",
		Name:       "ゲーム",
	}); err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}

	output, err := uc.Execute(ctx, usecase.GetBoardInput{Slug: "jazz"})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}

	if output.Board.Name != "ジャズ" {
		t.Errorf("output.Board.Name = %q、期待値 = %q", output.Board.Name, "ジャズ")
	}
	if output.Board.Description != "ジャズの話をする板" {
		t.Errorf("output.Board.Description = %q、期待値 = %q", output.Board.Description, "ジャズの話をする板")
	}
	if output.Category.Slug != "music" {
		t.Errorf("output.Category.Slug = %q、期待値 = %q", output.Category.Slug, "music")
	}
	if output.Category.Name != "音楽" {
		t.Errorf("output.Category.Name = %q、期待値 = %q", output.Category.Name, "音楽")
	}
}

// TestGetBoardUsecase_Execute_WithoutACategoryは、どのカテゴリーにも属さない
// 掲示板が、失敗せずにカテゴリー無しで解決されることを検証します。どのカテゴリーにも
// 属さないことは正常な状態であり (ADR 0011)、ページは掲示板の在り処を述べる段を落として
// それを描画します。
func TestGetBoardUsecase_Execute_WithoutACategory(t *testing.T) {
	t.Parallel()

	uc, _, boardRepo := newGetBoardUsecase(t)
	ctx := context.Background()

	if _, err := boardRepo.Create(ctx, repository.CreateBoardInput{Slug: "jazz", Name: "ジャズ"}); err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}

	output, err := uc.Execute(ctx, usecase.GetBoardInput{Slug: "jazz"})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	if output.Board.Name != "ジャズ" {
		t.Errorf("output.Board.Name = %q、期待値 = %q", output.Board.Name, "ジャズ")
	}
	if output.Category != nil {
		t.Errorf("output.Category = %+v、期待値 = nil", output.Category)
	}
}

// TestGetBoardUsecase_Execute_UnknownSlugは、どの掲示板も指さないslugが
// AppErrCodeResourceNotFoundを持つAppErrorとして報告されることを検証します。これに
// より、ハンドラーは空のページや500ではなく404で応答できます。
func TestGetBoardUsecase_Execute_UnknownSlug(t *testing.T) {
	t.Parallel()

	uc, _, _ := newGetBoardUsecase(t)

	output, err := uc.Execute(context.Background(), usecase.GetBoardInput{Slug: "no-such-board"})
	if output != nil {
		t.Errorf("output = %+v、期待値 = nil", output)
	}

	ae := model.AsAppError(err)
	if ae == nil {
		t.Fatalf("Execute()のエラー = %v、期待値 = *model.AppError", err)
	}
	if ae.Code != model.AppErrCodeResourceNotFound {
		t.Errorf("ae.Code = %d、期待値 = %d (AppErrCodeResourceNotFound)", ae.Code, model.AppErrCodeResourceNotFound)
	}
}

// TestGetBoardUsecase_Execute_UnresolvableCategoryは、カテゴリーを名指している
// のにそれを読み戻せない掲示板が、AppErrorではなく素のエラーとして報告されることを
// 検証します。これによりハンドラーは404ではなく500で応答します。カテゴリーの削除は
// 名指しを、消えた行を指したままにするのではなく空にするため、まだ名指しているのに
// 読めない状態はページの不在ではなくデータの不整合です。404で応答すれば、まだ配信されて
// いる掲示板を落とすようクローラーに伝えてしまいます。
//
// カテゴリーのリポジトリには空の2つ目のデータベースを渡します。掲示板を持つほうの
// データベースでは、外部キーによりこの状態に到達できないためです。
func TestGetBoardUsecase_Execute_UnresolvableCategory(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	boardDB := testutil.SetupDB(t)

	music, err := repository.NewCategoryRepository(boardDB).Create(ctx, repository.CreateCategoryInput{Slug: "music", Name: "音楽"})
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}
	boardRepo := repository.NewBoardRepository(boardDB)
	if _, err := boardRepo.Create(ctx, repository.CreateBoardInput{
		CategoryID: &music.ID,
		Slug:       "jazz",
		Name:       "ジャズ",
	}); err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}

	uc := usecase.NewGetBoardUsecase(boardRepo, repository.NewCategoryRepository(testutil.SetupDB(t)))

	output, err := uc.Execute(ctx, usecase.GetBoardInput{Slug: "jazz"})
	if output != nil {
		t.Errorf("output = %+v、期待値 = nil", output)
	}
	if err == nil {
		t.Fatal("Execute()のエラー = nil、エラーを期待")
	}
	if ae := model.AsAppError(err); ae != nil {
		t.Errorf("Execute()のエラー = *model.AppError (Code = %d)、ハンドラーが500を返せるよう素のエラーを期待", ae.Code)
	}
}
