package usecase_test

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
)

// newGetCategoryUsecase builds the UseCase over a database the test owns,
// returning the repository alongside it so the test can arrange the rows it is
// about to read back.
//
// [Ja] newGetCategoryUsecase はテストが所有するデータベース上に UseCase を構築し、
// これから読み戻す行をテストが用意できるよう、リポジトリも併せて返します。
func newGetCategoryUsecase(t *testing.T) (*usecase.GetCategoryUsecase, *repository.CategoryRepository) {
	t.Helper()

	db := testutil.SetupDB(t)
	categoryRepo := repository.NewCategoryRepository(db)

	return usecase.NewGetCategoryUsecase(categoryRepo), categoryRepo
}

// TestGetCategoryUsecase_Execute verifies that Execute resolves the slug to the
// category stored under it, and to that one only. A second category is created
// alongside so the assertion reads the slug's resolution rather than whichever
// row happens to come first.
//
// [Ja] TestGetCategoryUsecase_Execute は、Execute が slug をその下に保存された
// カテゴリーへ、そしてそれだけへ解決することを検証します。2 つ目のカテゴリーも併せて
// 作るのは、検証が「たまたま最初に来る行」ではなく slug の解決を読んでいることを示す
// ためです。
func TestGetCategoryUsecase_Execute(t *testing.T) {
	t.Parallel()

	uc, categoryRepo := newGetCategoryUsecase(t)
	ctx := context.Background()

	if _, err := categoryRepo.Create(ctx, repository.CreateCategoryInput{Slug: "music", Name: "音楽", Position: 1}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := categoryRepo.Create(ctx, repository.CreateCategoryInput{Slug: "hobby", Name: "趣味", Position: 2}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	output, err := uc.Execute(ctx, usecase.GetCategoryInput{Slug: "music"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if output.Category.Name != "音楽" {
		t.Errorf("output.Category.Name = %q, want %q", output.Category.Name, "音楽")
	}
	if output.Category.Slug != "music" {
		t.Errorf("output.Category.Slug = %q, want %q", output.Category.Slug, "music")
	}
}

// TestGetCategoryUsecase_Execute_UnknownSlug verifies that a slug naming no
// category is reported as an AppError carrying AppErrCodeResourceNotFound, which
// is what lets the handler answer 404 instead of rendering an empty page or a
// 500.
//
// [Ja] TestGetCategoryUsecase_Execute_UnknownSlug は、どのカテゴリーも指さない slug が
// AppErrCodeResourceNotFound を持つ AppError として報告されることを検証します。これに
// より、ハンドラーは空のページや 500 ではなく 404 で応答できます。
func TestGetCategoryUsecase_Execute_UnknownSlug(t *testing.T) {
	t.Parallel()

	uc, _ := newGetCategoryUsecase(t)

	output, err := uc.Execute(context.Background(), usecase.GetCategoryInput{Slug: "no-such-category"})
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
