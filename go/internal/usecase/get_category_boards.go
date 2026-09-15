package usecase

import (
	"context"
	"fmt"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// GetCategoryBoardsInputは読み取る掲示板を、それを並べるカテゴリーで指定します。
// slugではなくidを受け取るのは、呼び出し側が既にカテゴリーを解決しており、
// 手元にあるものを引き直せばその分クエリが増えるためです。
type GetCategoryBoardsInput struct {
	CategoryID model.CategoryID
}

// GetCategoryBoardsOutputはカテゴリーページの一覧、すなわちそれが並べる掲示板を、
// コミュニティが並べた順で持ちます。
type GetCategoryBoardsOutput struct {
	Boards []*model.Board
}

// GetCategoryBoardsUsecaseはカテゴリー1つが並べる掲示板を読みます。
// GetCategoryUsecaseと分けているのは、ページを描く前に応答が決まるリクエスト
// (どのカテゴリーも指さないslug、および大文字小文字違いで到達して正規URLへ
// リダイレクトされるもの) が、カテゴリーの解決の分だけを支払うようにするためです。
//
// 読み取りUseCaseであり、リポジトリの取得系メソッドしか呼ばないため、validatorも
// トランザクションも必要としません。
type GetCategoryBoardsUsecase struct {
	boardRepo *repository.BoardRepository
}

// NewGetCategoryBoardsUsecaseは掲示板のリポジトリからGetCategoryBoardsUsecaseを
// 構築します。
func NewGetCategoryBoardsUsecase(boardRepo *repository.BoardRepository) *GetCategoryBoardsUsecase {
	return &GetCategoryBoardsUsecase{boardRepo: boardRepo}
}

// Executeは指定されたカテゴリーが並べる掲示板を読みます。コミュニティがまだ
// 掲示板を置いていないカテゴリーはエラーではなく空の一覧になります。ページが描画する
// 状態であって失敗ではないためです。
func (uc *GetCategoryBoardsUsecase) Execute(ctx context.Context, input GetCategoryBoardsInput) (*GetCategoryBoardsOutput, error) {
	boards, err := uc.boardRepo.ListByCategoryID(ctx, input.CategoryID)
	if err != nil {
		return nil, fmt.Errorf("掲示板一覧の取得に失敗: %w", err)
	}

	return &GetCategoryBoardsOutput{Boards: boards}, nil
}
