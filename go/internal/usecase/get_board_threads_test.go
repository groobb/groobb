package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
)

// boardThreadRepos bundles the repositories a board's threads are built from,
// over one database the test owns. A thread needs a board, a board needs a
// category, and giving a thread a last-post time needs a post, so a test of the
// listing has to create everything underneath it as well.
//
// [Ja] boardThreadRepos は、テストが所有する 1 つのデータベース上に、掲示板のスレッドを
// 組み立てるリポジトリ群をまとめます。スレッドには掲示板が、掲示板にはカテゴリーが、
// スレッドに最終投稿の時刻を与えるには投稿が要るため、一覧を検証するテストもその下に
// あるものをすべて作ることになります。
type boardThreadRepos struct {
	category *repository.CategoryRepository
	board    *repository.BoardRepository
	thread   *repository.ThreadRepository
	post     *repository.PostRepository
}

// newGetBoardThreadsUsecase builds the UseCase over a database the test owns,
// returning the repositories alongside it so the test can arrange the rows it is
// about to read back.
//
// [Ja] newGetBoardThreadsUsecase はテストが所有するデータベース上に UseCase を構築し、
// これから読み戻す行をテストが用意できるよう、リポジトリも併せて返します。
func newGetBoardThreadsUsecase(t *testing.T) (*usecase.GetBoardThreadsUsecase, *boardThreadRepos) {
	t.Helper()

	db := testutil.SetupDB(t)
	repos := &boardThreadRepos{
		category: repository.NewCategoryRepository(db),
		board:    repository.NewBoardRepository(db),
		thread:   repository.NewThreadRepository(db),
		post:     repository.NewPostRepository(db),
	}

	return usecase.NewGetBoardThreadsUsecase(repos.thread), repos
}

// createBoard creates a board together with the category it has to belong to.
//
// [Ja] createBoard は掲示板を、それが属さなければならないカテゴリーと一緒に作ります。
func (r *boardThreadRepos) createBoard(t *testing.T, ctx context.Context, slug string) *model.Board {
	t.Helper()

	category, err := r.category.Create(ctx, repository.CreateCategoryInput{Slug: slug + "-category", Name: slug})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	board, err := r.board.Create(ctx, repository.CreateBoardInput{CategoryID: &category.ID, Slug: slug, Name: slug})
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
func (r *boardThreadRepos) createThread(t *testing.T, ctx context.Context, boardID model.BoardID, title string, postsCount int, lastPostedAt time.Time) *model.Thread {
	t.Helper()

	thread, err := r.thread.Create(ctx, repository.CreateThreadInput{BoardID: boardID, Title: title, Language: model.LocaleJa.ThreadLanguage()})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	post, err := r.post.Create(ctx, repository.CreatePostInput{ThreadID: thread.ID, Number: 1, Body: title + "の 1 つ目の投稿"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := r.thread.UpdateLastPost(ctx, thread.ID, repository.UpdateThreadLastPostInput{
		PostsCount:   postsCount,
		LastPostID:   post.ID,
		LastPostedAt: lastPostedAt,
	}); err != nil {
		t.Fatalf("UpdateLastPost() error = %v", err)
	}

	return thread
}

// TestGetBoardThreadsUsecase_Execute verifies that Execute returns the given
// board's threads with the most recently posted-in first, leaving out the
// threads of every other board. The threads are created in the reverse of the
// order they are expected back in, and a thread of a second board is created
// alongside them, so the assertion reads the last-post time and the board rather
// than the insertion order.
//
// [Ja] TestGetBoardThreadsUsecase_Execute は、Execute が指定した掲示板のスレッドを
// 最後に投稿されたものから順に返すこと、そして他の掲示板のスレッドを含めないことを検証
// します。スレッドは期待する順序と逆に作り、別の掲示板のスレッドも併せて作ります。
// 検証が挿入順ではなく最終投稿時刻と掲示板を読んでいることを示すためです。
func TestGetBoardThreadsUsecase_Execute(t *testing.T) {
	t.Parallel()

	uc, repos := newGetBoardThreadsUsecase(t)
	ctx := context.Background()

	jazz := repos.createBoard(t, ctx, "jazz")
	games := repos.createBoard(t, ctx, "games")

	base := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	repos.createThread(t, ctx, jazz.ID, "古いスレッド", 3, base)
	repos.createThread(t, ctx, jazz.ID, "新しいスレッド", 42, base.Add(time.Hour))
	repos.createThread(t, ctx, games.ID, "別の板のスレッド", 1, base.Add(2*time.Hour))

	output, err := uc.Execute(ctx, usecase.GetBoardThreadsInput{BoardID: jazz.ID})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if len(output.Threads) != 2 {
		t.Fatalf("len(output.Threads) = %d, want %d (この掲示板のスレッドのみ)", len(output.Threads), 2)
	}
	if output.Threads[0].Title != "新しいスレッド" {
		t.Errorf("output.Threads[0].Title = %q, want %q", output.Threads[0].Title, "新しいスレッド")
	}
	if output.Threads[1].Title != "古いスレッド" {
		t.Errorf("output.Threads[1].Title = %q, want %q", output.Threads[1].Title, "古いスレッド")
	}
	if output.Threads[0].PostsCount != 42 {
		t.Errorf("output.Threads[0].PostsCount = %d, want %d", output.Threads[0].PostsCount, 42)
	}
	if !output.Threads[0].LastPostedAt.Equal(base.Add(time.Hour)) {
		t.Errorf("output.Threads[0].LastPostedAt = %v, want %v", output.Threads[0].LastPostedAt, base.Add(time.Hour))
	}
}

// TestGetBoardThreadsUsecase_Execute_NoThreads verifies that a board nobody has
// posted in yet yields an empty listing rather than an error, since that is a
// state the page renders.
//
// [Ja] TestGetBoardThreadsUsecase_Execute_NoThreads は、まだ誰も書き込んでいない掲示板が
// エラーではなく空の一覧になることを検証します。ページが描画する状態であるためです。
func TestGetBoardThreadsUsecase_Execute_NoThreads(t *testing.T) {
	t.Parallel()

	uc, repos := newGetBoardThreadsUsecase(t)
	ctx := context.Background()

	board := repos.createBoard(t, ctx, "quiet")

	output, err := uc.Execute(ctx, usecase.GetBoardThreadsInput{BoardID: board.ID})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(output.Threads) != 0 {
		t.Errorf("len(output.Threads) = %d, want %d", len(output.Threads), 0)
	}
}
