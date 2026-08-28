package usecase_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
)

// communityHomeRepos bundles the repositories the home page's content is built
// from, over one database the test owns. A thread needs a board, and giving a
// thread a last-post time needs a post, so a test of the listing has to create
// everything underneath it as well.
//
// [Ja] communityHomeRepos は、テストが所有する 1 つのデータベース上に、ホームページの
// 中身を組み立てるリポジトリ群をまとめます。スレッドには掲示板が、スレッドに最終投稿の
// 時刻を与えるには投稿が要るため、一覧を検証するテストもその下にあるものをすべて作る
// ことになります。
type communityHomeRepos struct {
	board  *repository.BoardRepository
	thread *repository.ThreadRepository
	post   *repository.PostRepository
}

// newGetCommunityHomeUsecase builds the UseCase over a database the test owns,
// returning the repositories alongside it so the test can arrange the rows it is
// about to read back.
//
// [Ja] newGetCommunityHomeUsecase はテストが所有するデータベース上に UseCase を構築し、
// これから読み戻す行をテストが用意できるよう、リポジトリも併せて返します。
func newGetCommunityHomeUsecase(t *testing.T) (*usecase.GetCommunityHomeUsecase, *communityHomeRepos) {
	t.Helper()

	db := testutil.SetupDB(t)
	repos := &communityHomeRepos{
		board:  repository.NewBoardRepository(db),
		thread: repository.NewThreadRepository(db),
		post:   repository.NewPostRepository(db),
	}

	return usecase.NewGetCommunityHomeUsecase(repos.board, repos.thread), repos
}

// createBoard creates a board with no category, which is the state a board is
// allowed to sit in (ADR 0011) and the one this UseCase never asks about: home
// lists boards flat, the same way the sidebar does.
//
// [Ja] createBoard はカテゴリーを持たない掲示板を作ります。掲示板が置かれてよい状態
// (ADR 0011) であり、この UseCase が問わない状態でもあります。ホームはサイドバーと
// 同じく掲示板をフラットに並べるためです。
func (r *communityHomeRepos) createBoard(t *testing.T, ctx context.Context, slug string, position int) *model.Board {
	t.Helper()

	board, err := r.board.Create(ctx, repository.CreateBoardInput{Slug: slug, Name: slug, Position: position})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	return board
}

// createThread creates a thread the way one exists in practice: with a first
// post, and with the denormalized columns describing that post. The listing
// orders by those columns, so a thread without them could not be placed.
//
// [Ja] createThread は、実際にスレッドが存在する形 — 最初の投稿を伴い、非正規化列が
// その投稿を表している状態 — でスレッドを作ります。一覧はその列で並べるため、それを
// 持たないスレッドは順序を決められません。
func (r *communityHomeRepos) createThread(t *testing.T, ctx context.Context, boardID model.BoardID, title string, lastPostedAt time.Time) {
	t.Helper()

	thread, err := r.thread.Create(ctx, repository.CreateThreadInput{BoardID: boardID, Title: title})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	post, err := r.post.Create(ctx, repository.CreatePostInput{ThreadID: thread.ID, Number: 1, Body: title + "の 1 つ目の投稿"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := r.thread.UpdateLastPost(ctx, thread.ID, repository.UpdateThreadLastPostInput{
		PostsCount:   1,
		LastPostID:   post.ID,
		LastPostedAt: lastPostedAt,
	}); err != nil {
		t.Fatalf("UpdateLastPost() error = %v", err)
	}
}

// TestGetCommunityHomeUsecase_Execute verifies that Execute returns a section
// per board in the order the community placed them, each holding that board's
// threads with the most recently posted-in first. The boards are created in the
// reverse of the order they are expected back in, so the assertion reads the
// board's position rather than the insertion order.
//
// [Ja] TestGetCommunityHomeUsecase_Execute は、Execute が掲示板ごとの区画をコミュニティが
// 並べた順で返し、各区画がその掲示板のスレッドを最後に投稿されたものから順に持つことを
// 検証します。掲示板は期待する順序と逆に作り、検証が挿入順ではなく掲示板の position を
// 読んでいることを示します。
func TestGetCommunityHomeUsecase_Execute(t *testing.T) {
	t.Parallel()

	uc, repos := newGetCommunityHomeUsecase(t)
	ctx := context.Background()

	games := repos.createBoard(t, ctx, "games", 2)
	jazz := repos.createBoard(t, ctx, "jazz", 1)

	base := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	repos.createThread(t, ctx, jazz.ID, "古いスレッド", base)
	repos.createThread(t, ctx, jazz.ID, "新しいスレッド", base.Add(time.Hour))
	repos.createThread(t, ctx, games.ID, "別の板のスレッド", base.Add(2*time.Hour))

	output, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if len(output.Boards) != 2 {
		t.Fatalf("len(output.Boards) = %d, want %d", len(output.Boards), 2)
	}
	if output.Boards[0].Board.Slug != "jazz" {
		t.Errorf("output.Boards[0].Board.Slug = %q, want %q", output.Boards[0].Board.Slug, "jazz")
	}
	if output.Boards[1].Board.Slug != "games" {
		t.Errorf("output.Boards[1].Board.Slug = %q, want %q", output.Boards[1].Board.Slug, "games")
	}

	wantTitles := []string{"新しいスレッド", "古いスレッド"}
	if len(output.Boards[0].Threads) != len(wantTitles) {
		t.Fatalf("len(output.Boards[0].Threads) = %d, want %d", len(output.Boards[0].Threads), len(wantTitles))
	}
	for i, want := range wantTitles {
		if output.Boards[0].Threads[i].Title != want {
			t.Errorf("output.Boards[0].Threads[%d].Title = %q, want %q", i, output.Boards[0].Threads[i].Title, want)
		}
	}
	if len(output.Boards[1].Threads) != 1 {
		t.Fatalf("len(output.Boards[1].Threads) = %d, want %d", len(output.Boards[1].Threads), 1)
	}
	if output.Boards[1].Threads[0].Title != "別の板のスレッド" {
		t.Errorf("output.Boards[1].Threads[0].Title = %q, want %q", output.Boards[1].Threads[0].Title, "別の板のスレッド")
	}
}

// TestGetCommunityHomeUsecase_Execute_LimitsThreadsPerBoard verifies that a
// board holding more threads than the page shows contributes only its latest
// ones, and that the limit is applied to each board rather than to the listing
// as a whole: the quiet board's single thread comes back even though the busy
// board alone already fills the page's allowance.
//
// [Ja] TestGetCommunityHomeUsecase_Execute_LimitsThreadsPerBoard は、ページが見せる
// 件数より多くのスレッドを持つ掲示板が最新のものだけを寄せること、そして上限が一覧全体
// ではなく掲示板ごとに適用されることを検証します。動きの多い掲示板だけで既にページの
// 枠が埋まっていても、静かな掲示板の 1 件は返ります。
func TestGetCommunityHomeUsecase_Execute_LimitsThreadsPerBoard(t *testing.T) {
	t.Parallel()

	uc, repos := newGetCommunityHomeUsecase(t)
	ctx := context.Background()

	busy := repos.createBoard(t, ctx, "busy", 1)
	quiet := repos.createBoard(t, ctx, "quiet", 2)

	base := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	for i := range usecase.HomeThreadsPerBoard + 3 {
		repos.createThread(t, ctx, busy.ID, "賑わいのスレッド", base.Add(time.Duration(i)*time.Hour))
	}
	repos.createThread(t, ctx, quiet.ID, "静かなスレッド", base.Add(-24*time.Hour))

	output, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if len(output.Boards) != 2 {
		t.Fatalf("len(output.Boards) = %d, want %d", len(output.Boards), 2)
	}
	if got := len(output.Boards[0].Threads); got != usecase.HomeThreadsPerBoard {
		t.Errorf("len(output.Boards[0].Threads) = %d, want %d", got, usecase.HomeThreadsPerBoard)
	}
	if got := len(output.Boards[1].Threads); got != 1 {
		t.Errorf("len(output.Boards[1].Threads) = %d, want %d", got, 1)
	}
}

// TestGetCommunityHomeUsecase_Execute_BoardWithoutThreads verifies that a board
// nobody has posted in yet comes back as a section with no threads rather than
// being left out, since the sidebar lists it and home leaving it out would read
// as a gap.
//
// [Ja] TestGetCommunityHomeUsecase_Execute_BoardWithoutThreads は、まだ誰も書き込んで
// いない掲示板が落とされるのではなくスレッドを持たない区画として返ることを検証します。
// サイドバーがそれを並べており、ホームが落とせば欠落として読まれるためです。
func TestGetCommunityHomeUsecase_Execute_BoardWithoutThreads(t *testing.T) {
	t.Parallel()

	uc, repos := newGetCommunityHomeUsecase(t)
	ctx := context.Background()

	repos.createBoard(t, ctx, "quiet", 1)

	output, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if len(output.Boards) != 1 {
		t.Fatalf("len(output.Boards) = %d, want %d", len(output.Boards), 1)
	}
	if len(output.Boards[0].Threads) != 0 {
		t.Errorf("len(output.Boards[0].Threads) = %d, want %d", len(output.Boards[0].Threads), 0)
	}
}

// TestGetCommunityHomeUsecase_Execute_NoBoards verifies that a community with no
// board yet yields an empty listing rather than an error: it is the state an
// instance opens in (ADR 0010), which the page renders.
//
// [Ja] TestGetCommunityHomeUsecase_Execute_NoBoards は、まだ掲示板を 1 つも持たない
// コミュニティがエラーではなく空の一覧になることを検証します。インスタンスが立ち上がった
// ときの状態 (ADR 0010) であり、ページが描画するものであるためです。
func TestGetCommunityHomeUsecase_Execute_NoBoards(t *testing.T) {
	t.Parallel()

	uc, _ := newGetCommunityHomeUsecase(t)

	output, err := uc.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(output.Boards) != 0 {
		t.Errorf("len(output.Boards) = %d, want %d", len(output.Boards), 0)
	}
}

// TestGetCommunityHomeUsecase_Execute_BoardLookupFailure verifies that failure
// to read the boards is returned with the context identifying the failed step
// and without a partial output.
//
// [Ja] TestGetCommunityHomeUsecase_Execute_BoardLookupFailure は、掲示板の読み取り失敗が
// 失敗した段階を示す文脈とともに返り、部分的な出力を返さないことを検証します。
func TestGetCommunityHomeUsecase_Execute_BoardLookupFailure(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	if err := db.Reader.Close(); err != nil {
		t.Fatalf("Reader の Close() error = %v", err)
	}
	uc := usecase.NewGetCommunityHomeUsecase(
		repository.NewBoardRepository(db),
		repository.NewThreadRepository(db),
	)

	output, err := uc.Execute(context.Background())
	if output != nil {
		t.Errorf("Execute() = %v, want nil", output)
	}
	if err == nil {
		t.Fatal("Execute() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "掲示板一覧の取得に失敗") {
		t.Errorf("Execute() error = %q, want board lookup context", err)
	}
}

// TestGetCommunityHomeUsecase_Execute_ThreadLookupFailure verifies that failure
// to read recent threads after reading the boards is returned with the context
// identifying the failed step and without a partial output.
//
// [Ja] TestGetCommunityHomeUsecase_Execute_ThreadLookupFailure は、掲示板の読み取り成功後に
// 最新スレッドの読み取りが失敗した場合、失敗した段階を示す文脈とともにエラーが返り、
// 部分的な出力を返さないことを検証します。
func TestGetCommunityHomeUsecase_Execute_ThreadLookupFailure(t *testing.T) {
	t.Parallel()

	boardDB := testutil.SetupDB(t)
	threadDB := testutil.SetupDB(t)
	if err := threadDB.Reader.Close(); err != nil {
		t.Fatalf("Reader の Close() error = %v", err)
	}
	uc := usecase.NewGetCommunityHomeUsecase(
		repository.NewBoardRepository(boardDB),
		repository.NewThreadRepository(threadDB),
	)

	output, err := uc.Execute(context.Background())
	if output != nil {
		t.Errorf("Execute() = %v, want nil", output)
	}
	if err == nil {
		t.Fatal("Execute() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "掲示板ごとの最新スレッドの取得に失敗") {
		t.Errorf("Execute() error = %q, want recent thread lookup context", err)
	}
}
