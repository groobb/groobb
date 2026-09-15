package usecase_test

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
)

// newGetCategoryUsecaseはテストが所有するデータベース上にUseCaseを構築し、
// これから読み戻す行をテストが用意できるよう、リポジトリも併せて返します。
func newGetCategoryUsecase(t *testing.T) (*usecase.GetCategoryUsecase, *repository.CategoryRepository) {
	t.Helper()

	db := testutil.SetupDB(t)
	categoryRepo := repository.NewCategoryRepository(db)

	return usecase.NewGetCategoryUsecase(categoryRepo), categoryRepo
}

// TestGetCategoryUsecase_Executeは、Executeがslugをその下に保存された
// カテゴリーへ、そしてそれだけへ解決することを検証します。2つ目のカテゴリーも併せて
// 作るのは、検証が「たまたま最初に来る行」ではなくslugの解決を読んでいることを示す
// ためです。
func TestGetCategoryUsecase_Execute(t *testing.T) {
	t.Parallel()

	uc, categoryRepo := newGetCategoryUsecase(t)
	ctx := context.Background()

	if _, err := categoryRepo.Create(ctx, repository.CreateCategoryInput{Slug: "music", Name: "音楽", Position: 1}); err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}
	if _, err := categoryRepo.Create(ctx, repository.CreateCategoryInput{Slug: "hobby", Name: "趣味", Position: 2}); err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}

	output, err := uc.Execute(ctx, usecase.GetCategoryInput{Slug: "music"})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}

	if output.Category.Name != "音楽" {
		t.Errorf("output.Category.Name = %q、期待値 = %q", output.Category.Name, "音楽")
	}
	if output.Category.Slug != "music" {
		t.Errorf("output.Category.Slug = %q、期待値 = %q", output.Category.Slug, "music")
	}
}

// TestGetCategoryUsecase_Execute_UnknownSlugは、どのカテゴリーも指さないslugが
// AppErrCodeResourceNotFoundを持つAppErrorとして報告されることを検証します。これに
// より、ハンドラーは空のページや500ではなく404で応答できます。
func TestGetCategoryUsecase_Execute_UnknownSlug(t *testing.T) {
	t.Parallel()

	uc, _ := newGetCategoryUsecase(t)

	output, err := uc.Execute(context.Background(), usecase.GetCategoryInput{Slug: "no-such-category"})
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
