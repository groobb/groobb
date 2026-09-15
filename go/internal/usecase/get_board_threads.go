package usecase

import (
	"context"
	"fmt"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// GetBoardThreadsInputは読み取るスレッドを、それが立った掲示板で指定します。
// slugではなくidを受け取るのは、呼び出し側が既に掲示板を解決しており、手元にある
// ものを引き直せばその分クエリが増えるためです。
type GetBoardThreadsInput struct {
	BoardID model.BoardID
}

// GetBoardThreadsOutputは掲示板ページの一覧、すなわちそのスレッドを、最後に
// 投稿されたものから順に持ちます。
type GetBoardThreadsOutput struct {
	Threads []*model.Thread
}

// GetBoardThreadsUsecaseは掲示板1つが持つスレッドを読みます。GetBoardUsecaseと
// 分けているのは、ページを描く前に応答が決まるリクエスト (どの掲示板も指さないslug、
// および大文字小文字違いで到達して正規URLへリダイレクトされるもの) が、件数の決まった
// 掲示板とパンくず用カテゴリーの解決だけを支払い、コミュニティのナビゲーションと件数に
// 上限の無いスレッド一覧の分を支払わないようにするためです。
//
// 読み取りUseCaseであり、リポジトリの取得系メソッドしか呼ばないため、validatorも
// トランザクションも必要としません。
type GetBoardThreadsUsecase struct {
	threadRepo *repository.ThreadRepository
}

// NewGetBoardThreadsUsecaseはスレッドのリポジトリからGetBoardThreadsUsecaseを
// 構築します。
func NewGetBoardThreadsUsecase(threadRepo *repository.ThreadRepository) *GetBoardThreadsUsecase {
	return &GetBoardThreadsUsecase{threadRepo: threadRepo}
}

// Executeは指定された掲示板が持つスレッドを読みます。まだ誰も書き込んでいない
// 掲示板はエラーではなく空の一覧になります。ページが描画する状態であって失敗では
// ないためです。
func (uc *GetBoardThreadsUsecase) Execute(ctx context.Context, input GetBoardThreadsInput) (*GetBoardThreadsOutput, error) {
	threads, err := uc.threadRepo.ListByBoardID(ctx, input.BoardID)
	if err != nil {
		return nil, fmt.Errorf("スレッド一覧の取得に失敗: %w", err)
	}

	return &GetBoardThreadsOutput{Threads: threads}, nil
}
