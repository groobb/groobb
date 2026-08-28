package usecase

import (
	"context"
	"fmt"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// HomeThreadsPerBoard is how many of a board's latest threads the community's
// home page shows. It is a handful rather than a page's worth because home is
// the one place every board is listed at once: what each section says is where
// its conversation is right now, and the board's own page is where the rest of
// it is read.
//
// [Ja] HomeThreadsPerBoard は、コミュニティのホームが掲示板ごとに見せる最新スレッドの
// 件数です。1 ページ分ではなく数件なのは、ホームがすべての掲示板を一度に並べる唯一の
// 場所であるためです。各区画が述べるのはその掲示板の会話が今どこにあるかであり、残りを
// 読む場所は掲示板自身のページです。
const HomeThreadsPerBoard = 5

// HomeBoard is one section of the home page: a board and the threads of it that
// the section lists. Threads is empty for a board nobody has posted in yet,
// which is a state the page renders rather than a board it leaves out — the
// sidebar lists that board too, and home showing fewer boards than the sidebar
// would read as a gap.
//
// [Ja] HomeBoard はホームページの区画 1 つで、掲示板と、その区画が並べるその掲示板の
// スレッドを持ちます。まだ誰も書き込んでいない掲示板では Threads が空になります。これは
// ページが描画する状態であって、落とす掲示板ではありません。サイドバーはその掲示板も
// 並べるため、ホームがサイドバーより少ない掲示板を見せれば欠落として読まれます。
type HomeBoard struct {
	Board   *model.Board
	Threads []*model.Thread
}

// GetCommunityHomeOutput is the content of the community's home page: every
// board of the community, each with its latest threads, in the order the
// community placed the boards.
//
// The boards are sectioned rather than merged into one timeline because a
// community's boards move at different speeds: a single list ordered by time
// would be filled by whichever board is busiest, and one that moves a few
// threads a week would fall off it. A section per board keeps the slow board's
// latest visible as its latest (ADR 0010).
//
// [Ja] GetCommunityHomeOutput はコミュニティのホームページの中身です。コミュニティの
// すべての掲示板を、それぞれの最新スレッドとともに、コミュニティが並べた順で持ちます。
//
// 掲示板を 1 本の時系列に混ぜず区画に分けるのは、コミュニティの掲示板が異なる速度で
// 動くためです。時刻で並べた 1 つの一覧は最も投稿量の多い掲示板で埋まり、週に数件しか
// 動かない掲示板はそこから落ちます。掲示板ごとの区画なら、動きの遅い掲示板の最新も
// その掲示板の最新として見えます (ADR 0010)。
type GetCommunityHomeOutput struct {
	Boards []HomeBoard
}

// GetCommunityHomeUsecase reads what the community's home page lists. It is a
// read UseCase: it only calls the lookup methods of its repositories, so it
// needs neither a validator nor a transaction.
//
// It reads the boards itself rather than taking the ones the sidebar's UseCase
// returned, because the two lists answer different questions: the sidebar says
// where a visitor can go, and this one says what the page is about. Sharing one
// read would tie the page's content to a later decision about what the
// navigation shows.
//
// [Ja] GetCommunityHomeUsecase はコミュニティのホームページが並べるものを読みます。
// 読み取り UseCase であり、リポジトリの取得系メソッドしか呼ばないため、validator も
// トランザクションも必要としません。
//
// サイドバーの UseCase が返した掲示板を受け取らず自分で読むのは、2 つの一覧が別の問いに
// 答えているためです。サイドバーは訪問者がどこへ行けるかを述べ、こちらはページが何に
// ついてのものかを述べます。読み取りを共有すると、ナビゲーションが何を見せるかという
// 後の判断にページの中身が縛られます。
type GetCommunityHomeUsecase struct {
	boardRepo  *repository.BoardRepository
	threadRepo *repository.ThreadRepository
}

// NewGetCommunityHomeUsecase builds a GetCommunityHomeUsecase over the board
// and thread repositories.
//
// [Ja] NewGetCommunityHomeUsecase は掲示板とスレッドの各リポジトリから
// GetCommunityHomeUsecase を構築します。
func NewGetCommunityHomeUsecase(
	boardRepo *repository.BoardRepository,
	threadRepo *repository.ThreadRepository,
) *GetCommunityHomeUsecase {
	return &GetCommunityHomeUsecase{boardRepo: boardRepo, threadRepo: threadRepo}
}

// Execute reads the community's boards and the latest threads of all of them,
// then hands each board the threads that belong to it. The threads arrive as
// one list covering every board, so the page costs two queries whatever the
// number of boards.
//
// [Ja] Execute はコミュニティの掲示板と、そのすべての最新スレッドを読み、各掲示板に
// それに属するスレッドを渡します。スレッドはすべての掲示板を覆う 1 つの一覧として
// 得られるため、ページの費用は掲示板の数によらず 2 クエリです。
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
