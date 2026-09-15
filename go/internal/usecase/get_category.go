package usecase

import (
	"context"
	"fmt"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// GetCategoryInputは読み取るカテゴリーを、/c/{slug} が運ぶslugで指定します。
type GetCategoryInput struct {
	Slug string
}

// GetCategoryOutputはslugが解決したカテゴリーです。それが並べる掲示板は
// GetCategoryBoardsUsecaseが別に読みます。/c/{slug} は、そもそもページを描画するか
// どうかをこのカテゴリーだけで決めるためです。
type GetCategoryOutput struct {
	Category *model.Category
}

// GetCategoryUsecaseはslugをカテゴリー1つへ解決します。読み取りUseCaseで
// あり、リポジトリの取得系メソッドしか呼ばないため、validatorもトランザクションも
// 必要としません。
type GetCategoryUsecase struct {
	categoryRepo *repository.CategoryRepository
}

// NewGetCategoryUsecaseはカテゴリーのリポジトリからGetCategoryUsecaseを
// 構築します。
func NewGetCategoryUsecase(categoryRepo *repository.CategoryRepository) *GetCategoryUsecase {
	return &GetCategoryUsecase{categoryRepo: categoryRepo}
}

// Executeはslugをカテゴリーへ解決します。
//
// どのカテゴリーも指さないslugはAppErrCodeResourceNotFoundを持つAppErrorとして
// 報告し、ハンドラーが共通のnot-foundページで404を返せるようにします。これは手で
// 打たれた・推測された・削除されたカテゴリーの残したURLの既知の結果であって失敗では
// ないため、ここではエラーとしてログに残しません。
func (uc *GetCategoryUsecase) Execute(ctx context.Context, input GetCategoryInput) (*GetCategoryOutput, error) {
	category, err := uc.categoryRepo.FindBySlug(ctx, input.Slug)
	if err != nil {
		return nil, fmt.Errorf("カテゴリーの取得に失敗: %w", err)
	}
	if category == nil {
		return nil, &model.AppError{
			Code:     model.AppErrCodeResourceNotFound,
			UserMsg:  i18n.T(ctx, "error_not_found_message"),
			Internal: fmt.Errorf("カテゴリーが見つからない: slug=%s", input.Slug),
			Metadata: map[string]string{"category_slug": input.Slug},
		}
	}

	return &GetCategoryOutput{Category: category}, nil
}
