package usecase

import (
	"context"
	"fmt"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// GetBoardInputは読み取る掲示板を、/b/{slug} が運ぶslugで指定します。
type GetBoardInput struct {
	Slug string
}

// GetBoardOutputはslugが解決した掲示板と、それを並べるカテゴリーです。
// カテゴリーが伴うのは、/b/{slug} が掲示板をその在り処を言わずに名指しするため、
// ページ側がそれを述べる必要があるからです。訪問者がコミュニティのどの部分にいるのかを
// 知る手立ては、パンくずだけです。
//
// どのカテゴリーにも属さない掲示板ではCategoryがnilになります。これは欠落ではなく
// 正常な状態であり (ADR 0011)、その場合ページには掲示板の上位として名指す場所が
// ありません。
//
// そこに立つスレッドはGetBoardThreadsUsecaseが別に読みます。/b/{slug} は、そもそも
// ページを描画するかどうかを掲示板だけで決めるためです。
type GetBoardOutput struct {
	Board    *model.Board
	Category *model.Category
}

// GetBoardUsecaseはslugを掲示板1つと、それを並べるカテゴリーへ解決します。
// 読み取りUseCaseであり、リポジトリの取得系メソッドしか呼ばないため、validatorも
// トランザクションも必要としません。
type GetBoardUsecase struct {
	boardRepo    *repository.BoardRepository
	categoryRepo *repository.CategoryRepository
}

// NewGetBoardUsecaseは掲示板とカテゴリーの各リポジトリからGetBoardUsecaseを
// 構築します。
func NewGetBoardUsecase(boardRepo *repository.BoardRepository, categoryRepo *repository.CategoryRepository) *GetBoardUsecase {
	return &GetBoardUsecase{boardRepo: boardRepo, categoryRepo: categoryRepo}
}

// Executeはslugを掲示板へ解決し、それを並べるカテゴリーを読みます。
//
// どの掲示板も指さないslugはAppErrCodeResourceNotFoundを持つAppErrorとして報告し、
// ハンドラーが共通のnot-foundページで404を返せるようにします。これは手で打たれた・
// 推測された・削除された掲示板の残したURLの既知の結果であって失敗ではないため、
// ここではエラーとしてログに残しません。
//
// 一方、カテゴリーを名指しているのにそれを読み戻せない場合は失敗です。カテゴリーの削除は
// 列を、消えた行を指したままにするのではなく空にするため、まだ名指している掲示板が指す
// カテゴリーは存在します。これをページの不在として報告すれば、まだ存在する掲示板を落とす
// ようクローラーに伝えてしまいます。
func (uc *GetBoardUsecase) Execute(ctx context.Context, input GetBoardInput) (*GetBoardOutput, error) {
	board, err := uc.boardRepo.FindBySlug(ctx, input.Slug)
	if err != nil {
		return nil, fmt.Errorf("掲示板の取得に失敗: %w", err)
	}
	if board == nil {
		return nil, &model.AppError{
			Code:     model.AppErrCodeResourceNotFound,
			UserMsg:  i18n.T(ctx, "error_not_found_message"),
			Internal: fmt.Errorf("掲示板が見つからない: slug=%s", input.Slug),
			Metadata: map[string]string{"board_slug": input.Slug},
		}
	}

	if board.CategoryID == nil {
		return &GetBoardOutput{Board: board}, nil
	}

	category, err := uc.categoryRepo.FindByID(ctx, *board.CategoryID)
	if err != nil {
		return nil, fmt.Errorf("掲示板のカテゴリーの取得に失敗: %w", err)
	}
	if category == nil {
		return nil, fmt.Errorf("掲示板のカテゴリーが見つからない: board_id=%s category_id=%s", board.ID, *board.CategoryID)
	}

	return &GetBoardOutput{Board: board, Category: category}, nil
}
