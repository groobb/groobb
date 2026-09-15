package usecase

import (
	"context"
	"fmt"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// HomeThreadsPerBoardは、コミュニティのホームが掲示板ごとに見せる最新スレッドの
// 件数です。1ページ分ではなく数件なのは、ホームがすべての掲示板を一度に並べる唯一の
// 場所であるためです。各区画が述べるのはその掲示板の会話が今どこにあるかであり、残りを
// 読む場所は掲示板自身のページです。
const HomeThreadsPerBoard = 5

// HomeBoardはホームページの区画1つで、掲示板と、その区画が並べるその掲示板の
// スレッドを持ちます。まだ誰も書き込んでいない掲示板ではThreadsが空になります。これは
// ページが描画する状態であって、落とす掲示板ではありません。サイドバーはその掲示板も
// 並べるため、ホームがサイドバーより少ない掲示板を見せれば欠落として読まれます。
type HomeBoard struct {
	Board   *model.Board
	Threads []*model.Thread
}

// GetCommunityHomeOutputはコミュニティのホームページの中身です。コミュニティの
// すべての掲示板を、それぞれの最新スレッドとともに、コミュニティが並べた順で持ちます。
//
// 掲示板を1本の時系列に混ぜず区画に分けるのは、コミュニティの掲示板が異なる速度で
// 動くためです。時刻で並べた1つの一覧は最も投稿量の多い掲示板で埋まり、週に数件しか
// 動かない掲示板はそこから落ちます。掲示板ごとの区画なら、動きの遅い掲示板の最新も
// その掲示板の最新として見えます (ADR 0010)。
type GetCommunityHomeOutput struct {
	Boards []HomeBoard
}

// GetCommunityHomeUsecaseはコミュニティのホームページが並べるものを読みます。
// 読み取りUseCaseであり、リポジトリの取得系メソッドしか呼ばないため、validatorも
// トランザクションも必要としません。
//
// サイドバーのUseCaseが返した掲示板を受け取らず自分で読むのは、2つの一覧が別の問いに
// 答えているためです。サイドバーは訪問者がどこへ行けるかを述べ、こちらはページが何に
// ついてのものかを述べます。読み取りを共有すると、ナビゲーションが何を見せるかという
// 後の判断にページの中身が縛られます。
type GetCommunityHomeUsecase struct {
	boardRepo  *repository.BoardRepository
	threadRepo *repository.ThreadRepository
}

// NewGetCommunityHomeUsecaseは掲示板とスレッドの各リポジトリから
// GetCommunityHomeUsecaseを構築します。
func NewGetCommunityHomeUsecase(
	boardRepo *repository.BoardRepository,
	threadRepo *repository.ThreadRepository,
) *GetCommunityHomeUsecase {
	return &GetCommunityHomeUsecase{boardRepo: boardRepo, threadRepo: threadRepo}
}

// Executeはコミュニティの掲示板と、そのすべての最新スレッドを読み、各掲示板に
// それに属するスレッドを渡します。スレッドはすべての掲示板を覆う1つの一覧として
// 得られるため、ページの費用は掲示板の数によらず2クエリです。
func (uc *GetCommunityHomeUsecase) Execute(ctx context.Context) (*GetCommunityHomeOutput, error) {
	boards, err := uc.boardRepo.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("掲示板一覧の取得に失敗: %w", err)
	}

	threads, err := uc.threadRepo.ListRecentPerBoard(ctx, HomeThreadsPerBoard)
	if err != nil {
		return nil, fmt.Errorf("掲示板ごとの最新スレッドの取得に失敗: %w", err)
	}

	threadsByBoard := make(map[model.BoardID][]*model.Thread, len(boards))
	for _, thread := range threads {
		threadsByBoard[thread.BoardID] = append(threadsByBoard[thread.BoardID], thread)
	}

	homeBoards := make([]HomeBoard, len(boards))
	for i, board := range boards {
		homeBoards[i] = HomeBoard{Board: board, Threads: threadsByBoard[board.ID]}
	}

	return &GetCommunityHomeOutput{Boards: homeBoards}, nil
}
