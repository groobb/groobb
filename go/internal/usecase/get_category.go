package usecase

import (
	"context"
	"fmt"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// GetCategoryInput addresses the category to read by the slug /c/{slug}
// carries.
//
// [Ja] GetCategoryInput は読み取るカテゴリーを、/c/{slug} が運ぶ slug で指定します。
type GetCategoryInput struct {
	Slug string
}

// GetCategoryOutput is the category the slug resolved to. The boards it lists
// are read separately by GetCategoryBoardsUsecase, because /c/{slug} decides
// whether the page is going to be rendered at all from this category alone.
//
// [Ja] GetCategoryOutput は slug が解決したカテゴリーです。それが並べる掲示板は
// GetCategoryBoardsUsecase が別に読みます。/c/{slug} は、そもそもページを描画するか
// どうかをこのカテゴリーだけで決めるためです。
type GetCategoryOutput struct {
	Category *model.Category
}

// GetCategoryUsecase resolves a slug to one category. It is a read UseCase: it
// only calls the lookup methods of its repository, so it needs neither a
// validator nor a transaction.
//
// [Ja] GetCategoryUsecase は slug をカテゴリー 1 つへ解決します。読み取り UseCase で
// あり、リポジトリの取得系メソッドしか呼ばないため、validator もトランザクションも
// 必要としません。
type GetCategoryUsecase struct {
	categoryRepo *repository.CategoryRepository
}

// NewGetCategoryUsecase builds a GetCategoryUsecase over the category
// repository.
//
// [Ja] NewGetCategoryUsecase はカテゴリーのリポジトリから GetCategoryUsecase を
// 構築します。
func NewGetCategoryUsecase(categoryRepo *repository.CategoryRepository) *GetCategoryUsecase {
	return &GetCategoryUsecase{categoryRepo: categoryRepo}
}

// Execute resolves the slug to a category.
//
// A slug naming no category is reported as an AppError carrying
// AppErrCodeResourceNotFound, which is what lets the handler answer 404 with the
// shared not-found page. It is a known outcome of a URL that was typed, guessed,
// or left behind by a deleted category rather than a failure, so it is not
// logged as an error here.
//
// [Ja] Execute は slug をカテゴリーへ解決します。
//
// どのカテゴリーも指さない slug は AppErrCodeResourceNotFound を持つ AppError として
// 報告し、ハンドラーが共通の not-found ページで 404 を返せるようにします。これは手で
// 打たれた・推測された・削除されたカテゴリーの残した URL の既知の結果であって失敗では
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
