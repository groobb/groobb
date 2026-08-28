package usecase

import (
	"context"
	"fmt"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// GetBoardThreadsInput addresses the threads to read by the board they were
// posted in. It takes the id rather than the slug because the caller has already
// resolved the board, and reading it a second time would cost a query for
// something it is holding.
//
// [Ja] GetBoardThreadsInput は読み取るスレッドを、それが立った掲示板で指定します。
// slug ではなく id を受け取るのは、呼び出し側が既に掲示板を解決しており、手元にある
// ものを引き直せばその分クエリが増えるためです。
type GetBoardThreadsInput struct {
	BoardID model.BoardID
}

// GetBoardThreadsOutput is the listing of a board's page: its threads, the most
// recently posted-in first.
//
// [Ja] GetBoardThreadsOutput は掲示板ページの一覧、すなわちそのスレッドを、最後に
// 投稿されたものから順に持ちます。
type GetBoardThreadsOutput struct {
	Threads []*model.Thread
}

// GetBoardThreadsUsecase reads the threads one board holds. It is split from
// GetBoardUsecase so that a request answered before the page is drawn — a slug
// naming no board, or one reaching the board through a case variant that is
// redirected to the canonical URL — pays only for the bounded board and
// breadcrumb-category resolution, not for the community navigation or the
// unbounded thread listing.
//
// It is a read UseCase: it only calls the lookup methods of its repository, so
// it needs neither a validator nor a transaction.
//
// [Ja] GetBoardThreadsUsecase は掲示板 1 つが持つスレッドを読みます。GetBoardUsecase と
// 分けているのは、ページを描く前に応答が決まるリクエスト (どの掲示板も指さない slug、
// および大文字小文字違いで到達して正規 URL へリダイレクトされるもの) が、件数の決まった
// 掲示板とパンくず用カテゴリーの解決だけを支払い、コミュニティのナビゲーションと件数に
// 上限の無いスレッド一覧の分を支払わないようにするためです。
//
// 読み取り UseCase であり、リポジトリの取得系メソッドしか呼ばないため、validator も
// トランザクションも必要としません。
type GetBoardThreadsUsecase struct {
	threadRepo *repository.ThreadRepository
}

// NewGetBoardThreadsUsecase builds a GetBoardThreadsUsecase over the thread
// repository.
//
// [Ja] NewGetBoardThreadsUsecase はスレッドのリポジトリから GetBoardThreadsUsecase を
// 構築します。
func NewGetBoardThreadsUsecase(threadRepo *repository.ThreadRepository) *GetBoardThreadsUsecase {
	return &GetBoardThreadsUsecase{threadRepo: threadRepo}
}

// Execute reads the threads the given board holds. A board nobody has posted in
// yet yields an empty listing rather than an error: it is a state the page
// renders, not a failure.
//
// [Ja] Execute は指定された掲示板が持つスレッドを読みます。まだ誰も書き込んでいない
// 掲示板はエラーではなく空の一覧になります。ページが描画する状態であって失敗では
// ないためです。
func (uc *GetBoardThreadsUsecase) Execute(ctx context.Context, input GetBoardThreadsInput) (*GetBoardThreadsOutput, error) {
	threads, err := uc.threadRepo.ListByBoardID(ctx, input.BoardID)
	if err != nil {
		return nil, fmt.Errorf("スレッド一覧の取得に失敗: %w", err)
	}

	return &GetBoardThreadsOutput{Threads: threads}, nil
}
