package usecase

import (
	"context"
	"fmt"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// GetCategoryBoardsInput addresses the boards to read by the category that
// lists them. It takes the id rather than the slug because the caller has
// already resolved the category, and reading it a second time would cost a
// query for something it is holding.
//
// [Ja] GetCategoryBoardsInput は読み取る掲示板を、それを並べるカテゴリーで指定します。
// slug ではなく id を受け取るのは、呼び出し側が既にカテゴリーを解決しており、
// 手元にあるものを引き直せばその分クエリが増えるためです。
type GetCategoryBoardsInput struct {
	CategoryID model.CategoryID
}

// GetCategoryBoardsOutput is the listing of a category's page: the boards it
// lists, in the order the community placed them.
//
// [Ja] GetCategoryBoardsOutput はカテゴリーページの一覧、すなわちそれが並べる掲示板を、
// コミュニティが並べた順で持ちます。
type GetCategoryBoardsOutput struct {
	Boards []*model.Board
}

// GetCategoryBoardsUsecase reads the boards one category lists. It is split
// from GetCategoryUsecase so that a request answered before the page is drawn —
// a slug naming no category, or one reaching the category through a
// case variant that is redirected to the canonical URL — pays for resolving the
// category and nothing more.
//
// It is a read UseCase: it only calls the lookup methods of its repository, so
// it needs neither a validator nor a transaction.
//
// [Ja] GetCategoryBoardsUsecase はカテゴリー 1 つが並べる掲示板を読みます。
// GetCategoryUsecase と分けているのは、ページを描く前に応答が決まるリクエスト
// (どのカテゴリーも指さない slug、および大文字小文字違いで到達して正規 URL へ
// リダイレクトされるもの) が、カテゴリーの解決の分だけを支払うようにするためです。
//
// 読み取り UseCase であり、リポジトリの取得系メソッドしか呼ばないため、validator も
// トランザクションも必要としません。
type GetCategoryBoardsUsecase struct {
	boardRepo *repository.BoardRepository
}

// NewGetCategoryBoardsUsecase builds a GetCategoryBoardsUsecase over the board
// repository.
//
// [Ja] NewGetCategoryBoardsUsecase は掲示板のリポジトリから GetCategoryBoardsUsecase を
// 構築します。
func NewGetCategoryBoardsUsecase(boardRepo *repository.BoardRepository) *GetCategoryBoardsUsecase {
	return &GetCategoryBoardsUsecase{boardRepo: boardRepo}
}

// Execute reads the boards the given category lists. A category the community
// has yet to place a board in yields an empty listing rather than an error: it
// is a state the page renders, not a failure.
//
// [Ja] Execute は指定されたカテゴリーが並べる掲示板を読みます。コミュニティがまだ
// 掲示板を置いていないカテゴリーはエラーではなく空の一覧になります。ページが描画する
// 状態であって失敗ではないためです。
func (uc *GetCategoryBoardsUsecase) Execute(ctx context.Context, input GetCategoryBoardsInput) (*GetCategoryBoardsOutput, error) {
	boards, err := uc.boardRepo.ListByCategoryID(ctx, input.CategoryID)
	if err != nil {
		return nil, fmt.Errorf("掲示板一覧の取得に失敗: %w", err)
	}

	return &GetCategoryBoardsOutput{Boards: boards}, nil
}
