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

// communityHomeReposは、テストが所有する1つのデータベース上に、ホームページの
// 中身を組み立てるリポジトリ群をまとめます。スレッドには掲示板が、スレッドに最終投稿の
// 時刻を与えるには投稿が要るため、一覧を検証するテストもその下にあるものをすべて作る
// ことになります。
type communityHomeRepos struct {
	board  *repository.BoardRepository
	thread *repository.ThreadRepository
	post   *repository.PostRepository
}

// newGetCommunityHomeUsecaseはテストが所有するデータベース上にUseCaseを構築し、
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

// createBoardはカテゴリーを持たない掲示板を作ります。掲示板が置かれてよい状態
// (ADR 0011) であり、このUseCaseが問わない状態でもあります。ホームはサイドバーと
// 同じく掲示板をフラットに並べるためです。
func (r *communityHomeRepos) createBoard(t *testing.T, ctx context.Context, slug string, position int) *model.Board {
	t.Helper()

	board, err := r.board.Create(ctx, repository.CreateBoardInput{Slug: slug, Name: slug, Position: position})
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}

	return board
}

// createThreadは、実際にスレッドが存在する形 — 最初の投稿を伴い、非正規化列が
// その投稿を表している状態 — でスレッドを作ります。一覧はその列で並べるため、それを
// 持たないスレッドは順序を決められません。
func (r *communityHomeRepos) createThread(t *testing.T, ctx context.Context, boardID model.BoardID, title string, lastPostedAt time.Time) {
	t.Helper()

	thread, err := r.thread.Create(ctx, repository.CreateThreadInput{BoardID: boardID, Title: title, Language: model.LocaleJa.ThreadLanguage()})
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}
	post, err := r.post.Create(ctx, repository.CreatePostInput{ThreadID: thread.ID, Number: 1, Body: title + "の1つ目の投稿"})
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}
	if err := r.thread.UpdateLastPost(ctx, thread.ID, repository.UpdateThreadLastPostInput{
		PostsCount:   1,
		LastPostID:   post.ID,
		LastPostedAt: lastPostedAt,
	}); err != nil {
		t.Fatalf("UpdateLastPost()のエラー = %v", err)
	}
}

// TestGetCommunityHomeUsecase_Executeは、Executeが掲示板ごとの区画をコミュニティが
// 並べた順で返し、各区画がその掲示板のスレッドを最後に投稿されたものから順に持つことを
// 検証します。掲示板は期待する順序と逆に作り、検証が挿入順ではなく掲示板のpositionを
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
		t.Fatalf("Execute()のエラー = %v", err)
	}

	if len(output.Boards) != 2 {
		t.Fatalf("len(output.Boards) = %d、期待値 = %d", len(output.Boards), 2)
	}
	if output.Boards[0].Board.Slug != "jazz" {
		t.Errorf("output.Boards[0].Board.Slug = %q、期待値 = %q", output.Boards[0].Board.Slug, "jazz")
	}
	if output.Boards[1].Board.Slug != "games" {
		t.Errorf("output.Boards[1].Board.Slug = %q、期待値 = %q", output.Boards[1].Board.Slug, "games")
	}

	wantTitles := []string{"新しいスレッド", "古いスレッド"}
	if len(output.Boards[0].Threads) != len(wantTitles) {
		t.Fatalf("len(output.Boards[0].Threads) = %d、期待値 = %d", len(output.Boards[0].Threads), len(wantTitles))
	}
	for i, want := range wantTitles {
		if output.Boards[0].Threads[i].Title != want {
			t.Errorf("output.Boards[0].Threads[%d].Title = %q、期待値 = %q", i, output.Boards[0].Threads[i].Title, want)
		}
	}
	if len(output.Boards[1].Threads) != 1 {
		t.Fatalf("len(output.Boards[1].Threads) = %d、期待値 = %d", len(output.Boards[1].Threads), 1)
	}
	if output.Boards[1].Threads[0].Title != "別の板のスレッド" {
		t.Errorf("output.Boards[1].Threads[0].Title = %q、期待値 = %q", output.Boards[1].Threads[0].Title, "別の板のスレッド")
	}
}

// TestGetCommunityHomeUsecase_Execute_LimitsThreadsPerBoardは、ページが見せる
// 件数より多くのスレッドを持つ掲示板が最新のものだけを寄せること、そして上限が一覧全体
// ではなく掲示板ごとに適用されることを検証します。動きの多い掲示板だけで既にページの
// 枠が埋まっていても、静かな掲示板の1件は返ります。
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
		t.Fatalf("Execute()のエラー = %v", err)
	}

	if len(output.Boards) != 2 {
		t.Fatalf("len(output.Boards) = %d、期待値 = %d", len(output.Boards), 2)
	}
	if got := len(output.Boards[0].Threads); got != usecase.HomeThreadsPerBoard {
		t.Errorf("len(output.Boards[0].Threads) = %d、期待値 = %d", got, usecase.HomeThreadsPerBoard)
	}
	if got := len(output.Boards[1].Threads); got != 1 {
		t.Errorf("len(output.Boards[1].Threads) = %d、期待値 = %d", got, 1)
	}
}

// TestGetCommunityHomeUsecase_Execute_BoardWithoutThreadsは、まだ誰も書き込んで
// いない掲示板が落とされるのではなくスレッドを持たない区画として返ることを検証します。
// サイドバーがそれを並べており、ホームが落とせば欠落として読まれるためです。
func TestGetCommunityHomeUsecase_Execute_BoardWithoutThreads(t *testing.T) {
	t.Parallel()

	uc, repos := newGetCommunityHomeUsecase(t)
	ctx := context.Background()

	repos.createBoard(t, ctx, "quiet", 1)

	output, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}

	if len(output.Boards) != 1 {
		t.Fatalf("len(output.Boards) = %d、期待値 = %d", len(output.Boards), 1)
	}
	if len(output.Boards[0].Threads) != 0 {
		t.Errorf("len(output.Boards[0].Threads) = %d、期待値 = %d", len(output.Boards[0].Threads), 0)
	}
}

// TestGetCommunityHomeUsecase_Execute_NoBoardsは、まだ掲示板を1つも持たない
// コミュニティがエラーではなく空の一覧になることを検証します。インスタンスが立ち上がった
// ときの状態 (ADR 0010) であり、ページが描画するものであるためです。
func TestGetCommunityHomeUsecase_Execute_NoBoards(t *testing.T) {
	t.Parallel()

	uc, _ := newGetCommunityHomeUsecase(t)

	output, err := uc.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	if len(output.Boards) != 0 {
		t.Errorf("len(output.Boards) = %d、期待値 = %d", len(output.Boards), 0)
	}
}

// TestGetCommunityHomeUsecase_Execute_BoardLookupFailureは、掲示板の読み取り失敗が
// 失敗した段階を示す文脈とともに返り、部分的な出力を返さないことを検証します。
func TestGetCommunityHomeUsecase_Execute_BoardLookupFailure(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	if err := db.Reader.Close(); err != nil {
		t.Fatalf("ReaderのClose()のエラー = %v", err)
	}
	uc := usecase.NewGetCommunityHomeUsecase(
		repository.NewBoardRepository(db),
		repository.NewThreadRepository(db),
	)

	output, err := uc.Execute(context.Background())
	if output != nil {
		t.Errorf("Execute() = %v、期待値 = nil", output)
	}
	if err == nil {
		t.Fatal("Execute()のエラー = nil、エラーを期待")
	}
	if !strings.Contains(err.Error(), "掲示板一覧の取得に失敗") {
		t.Errorf("Execute()のエラー = %q、掲示板一覧の取得の文脈を含むことを期待", err)
	}
}

// TestGetCommunityHomeUsecase_Execute_ThreadLookupFailureは、掲示板の読み取り成功後に
// 最新スレッドの読み取りが失敗した場合、失敗した段階を示す文脈とともにエラーが返り、
// 部分的な出力を返さないことを検証します。
func TestGetCommunityHomeUsecase_Execute_ThreadLookupFailure(t *testing.T) {
	t.Parallel()

	boardDB := testutil.SetupDB(t)
	threadDB := testutil.SetupDB(t)
	if err := threadDB.Reader.Close(); err != nil {
		t.Fatalf("ReaderのClose()のエラー = %v", err)
	}
	uc := usecase.NewGetCommunityHomeUsecase(
		repository.NewBoardRepository(boardDB),
		repository.NewThreadRepository(threadDB),
	)

	output, err := uc.Execute(context.Background())
	if output != nil {
		t.Errorf("Execute() = %v、期待値 = nil", output)
	}
	if err == nil {
		t.Fatal("Execute()のエラー = nil、エラーを期待")
	}
	if !strings.Contains(err.Error(), "掲示板ごとの最新スレッドの取得に失敗") {
		t.Errorf("Execute()のエラー = %q、最近のスレッドの取得の文脈を含むことを期待", err)
	}
}
