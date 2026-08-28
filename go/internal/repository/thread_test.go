package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
)

// contentRepos bundles the repositories a community's content is built from,
// over one database the test owns. None of them stands alone: a thread needs a
// board, a board needs a category, a post needs a thread, and a reference needs
// two posts, so a test of any one of them has to create everything underneath
// it as well.
//
// [Ja] contentRepos は、テストが所有する 1 つのデータベース上に、コミュニティの中身を
// 組み立てるリポジトリ群をまとめる。どれも単独では成り立たない。スレッドには掲示板が、
// 掲示板にはカテゴリーが、投稿にはスレッドが、参照には 2 つの投稿が要るため、どれを
// 検証するテストもその下にあるものをすべて作ることになる。
type contentRepos struct {
	db            *database.DB
	category      *repository.CategoryRepository
	board         *repository.BoardRepository
	thread        *repository.ThreadRepository
	post          *repository.PostRepository
	postReference *repository.PostReferenceRepository
}

// newContentRepos builds the repositories over a fresh database.
//
// [Ja] newContentRepos は新しいデータベース上にリポジトリ群を作る。
func newContentRepos(t *testing.T) (*contentRepos, context.Context) {
	t.Helper()

	db := testutil.SetupDB(t)
	return &contentRepos{
		db:            db,
		category:      repository.NewCategoryRepository(db),
		board:         repository.NewBoardRepository(db),
		thread:        repository.NewThreadRepository(db),
		post:          repository.NewPostRepository(db),
		postReference: repository.NewPostReferenceRepository(db),
	}, context.Background()
}

// createBoardWithCategory creates a board together with the category it has to
// belong to, for tests whose subject is what a board contains rather than the
// board itself.
//
// [Ja] createBoardWithCategory は掲示板を、それが属さなければならないカテゴリーと
// 一緒に作る。掲示板そのものではなく掲示板の中身を問うテストのためのものである。
func (r *contentRepos) createBoardWithCategory(t *testing.T, ctx context.Context, slug string) *model.Board {
	t.Helper()

	category := createCategory(t, ctx, r.category, slug+"-category", 0)
	return createBoard(t, ctx, r.board, &category.ID, slug, 0)
}

// createThread inserts a thread with no author, failing the test on error. The
// author is left unset because no assertion that uses this helper depends on it,
// which keeps a test's fixtures down to what it is actually about.
//
// [Ja] createThread は作者を持たないスレッドを挿入し、エラー時はテストを失敗させる。
// 作者を設定しないのは、このヘルパーを使う検証がどれもそれに依存しないためで、テストの
// フィクスチャをそのテストが実際に問うているものだけに保つ。
func (r *contentRepos) createThread(t *testing.T, ctx context.Context, boardID model.BoardID, title string) *model.Thread {
	t.Helper()

	thread, err := r.thread.Create(ctx, repository.CreateThreadInput{
		BoardID: boardID,
		Title:   title,
	})
	if err != nil {
		t.Fatalf("テスト用スレッドの作成に失敗: %v", err)
	}

	return thread
}

// createThreadPostedAt creates a thread the way one exists in practice: with a
// first post, and with the denormalized columns describing that post. It is what
// a test needs in order to give threads distinct last-post times.
//
// [Ja] createThreadPostedAt は、実際にスレッドが存在する形 — 最初の投稿を伴い、非正規化
// 列がその投稿を表している状態 — でスレッドを作る。スレッドごとに異なる最終投稿時刻を
// 与えたいテストが必要とするものである。
func (r *contentRepos) createThreadPostedAt(t *testing.T, ctx context.Context, boardID model.BoardID, title string, lastPostedAt time.Time) *model.Thread {
	t.Helper()

	thread := r.createThread(t, ctx, boardID, title)
	post := r.createPost(t, ctx, thread.ID, 1, title+"の 1 つ目の投稿")

	err := r.thread.UpdateLastPost(ctx, thread.ID, repository.UpdateThreadLastPostInput{
		PostsCount:   1,
		LastPostID:   post.ID,
		LastPostedAt: lastPostedAt,
	})
	if err != nil {
		t.Fatalf("テスト用スレッドの最終投稿の更新に失敗: %v", err)
	}

	return thread
}

func TestThreadRepository_Create(t *testing.T) {
	t.Parallel()

	repos, ctx := newContentRepos(t)
	board := repos.createBoardWithCategory(t, ctx, "tech")
	userID := testutil.NewUserBuilder(t, repos.db).Build()

	thread, err := repos.thread.Create(ctx, repository.CreateThreadInput{
		BoardID: board.ID,
		UserID:  &userID,
		Title:   "SQLite の話",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if thread.ID == 0 {
		t.Error("Create() thread.ID は DB 採番で空でないはず")
	}
	if thread.BoardID != board.ID {
		t.Errorf("thread.BoardID = %v, want %v", thread.BoardID, board.ID)
	}
	if thread.UserID == nil {
		t.Fatal("thread.UserID = nil, want the author")
	}
	if *thread.UserID != userID {
		t.Errorf("*thread.UserID = %v, want %v", *thread.UserID, userID)
	}
	if thread.Title != "SQLite の話" {
		t.Errorf("thread.Title = %q, want %q", thread.Title, "SQLite の話")
	}
	if thread.PostsCount != 0 {
		t.Errorf("thread.PostsCount = %d, want %d", thread.PostsCount, 0)
	}
	if thread.LastPostID != nil {
		t.Errorf("thread.LastPostID = %v, want nil", *thread.LastPostID)
	}
	if thread.LastPostedAt.IsZero() {
		t.Error("thread.LastPostedAt は DB 既定値で設定されるはず")
	}
	if thread.CreatedAt.IsZero() {
		t.Error("thread.CreatedAt は DB 既定値で設定されるはず")
	}
	if thread.UpdatedAt.IsZero() {
		t.Error("thread.UpdatedAt は DB 既定値で設定されるはず")
	}
}

func TestThreadRepository_Create_LeavesAuthorUnsetForAWithdrawnUser(t *testing.T) {
	t.Parallel()

	repos, ctx := newContentRepos(t)
	board := repos.createBoardWithCategory(t, ctx, "tech")

	thread := repos.createThread(t, ctx, board.ID, "退会した人が立てたスレッド")

	if thread.UserID != nil {
		t.Errorf("thread.UserID = %v, want nil", *thread.UserID)
	}
}

func TestThreadRepository_FindByID(t *testing.T) {
	t.Parallel()

	repos, ctx := newContentRepos(t)
	board := repos.createBoardWithCategory(t, ctx, "tech")
	created := repos.createThread(t, ctx, board.ID, "SQLite の話")

	t.Run("id でスレッドを取得できる", func(t *testing.T) {
		thread, err := repos.thread.FindByID(ctx, created.ID)
		if err != nil {
			t.Fatalf("FindByID() error = %v", err)
		}
		if thread == nil {
			t.Fatal("FindByID() = nil, want thread")
		}
		if thread.ID != created.ID {
			t.Errorf("thread.ID = %v, want %v", thread.ID, created.ID)
		}
		if thread.Title != created.Title {
			t.Errorf("thread.Title = %q, want %q", thread.Title, created.Title)
		}
	})

	t.Run("存在しない id は (nil, nil) を返す", func(t *testing.T) {
		thread, err := repos.thread.FindByID(ctx, created.ID+1)
		if err != nil {
			t.Fatalf("FindByID() error = %v, want nil", err)
		}
		if thread != nil {
			t.Errorf("FindByID() = %v, want nil", thread)
		}
	})
}

func TestThreadRepository_ListByBoardID(t *testing.T) {
	t.Parallel()

	t.Run("最終投稿が新しい順に並び、同じ時刻は後から作られたほうが先に来る", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newContentRepos(t)
		board := repos.createBoardWithCategory(t, ctx, "tech")
		other := repos.createBoardWithCategory(t, ctx, "chat")

		// The insertion order is deliberately neither the expected order nor its
		// reverse, so a result that merely echoes it cannot pass.
		//
		// [Ja] 挿入順は期待する並びともその逆とも異なるようにしてあり、挿入順をそのまま
		// 返すだけの結果では通らないようにしている。
		noon := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
		repos.createThreadPostedAt(t, ctx, board.ID, "昼のスレッド", noon)
		repos.createThreadPostedAt(t, ctx, board.ID, "朝のスレッド", noon.Add(-3*time.Hour))
		repos.createThreadPostedAt(t, ctx, other.ID, "別の板のスレッド", noon.Add(time.Hour))
		repos.createThreadPostedAt(t, ctx, board.ID, "夜のスレッド", noon.Add(6*time.Hour))
		repos.createThreadPostedAt(t, ctx, board.ID, "昼のもう 1 つのスレッド", noon)

		threads, err := repos.thread.ListByBoardID(ctx, board.ID)
		if err != nil {
			t.Fatalf("ListByBoardID() error = %v", err)
		}

		wantTitles := []string{"夜のスレッド", "昼のもう 1 つのスレッド", "昼のスレッド", "朝のスレッド"}
		if len(threads) != len(wantTitles) {
			t.Fatalf("len(ListByBoardID()) = %d, want %d", len(threads), len(wantTitles))
		}
		for i, want := range wantTitles {
			if threads[i].Title != want {
				t.Errorf("ListByBoardID()[%d].Title = %q, want %q", i, threads[i].Title, want)
			}
		}
	})

	t.Run("スレッドを持たない掲示板は空を返す", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newContentRepos(t)
		board := repos.createBoardWithCategory(t, ctx, "tech")

		threads, err := repos.thread.ListByBoardID(ctx, board.ID)
		if err != nil {
			t.Fatalf("ListByBoardID() error = %v", err)
		}
		if len(threads) != 0 {
			t.Errorf("len(ListByBoardID()) = %d, want 0", len(threads))
		}
	})
}

func TestThreadRepository_ListRecentPerBoard(t *testing.T) {
	t.Parallel()

	t.Run("掲示板ごとに指定件数までを、板は position 順・スレッドは最終投稿が新しい順で返す", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newContentRepos(t)
		category := createCategory(t, ctx, repos.category, "general", 0)
		second := createBoard(t, ctx, repos.board, &category.ID, "chat", 2)
		first := createBoard(t, ctx, repos.board, &category.ID, "tech", 1)

		// The insertion order is deliberately neither the expected order nor its
		// reverse, so a result that merely echoes it cannot pass.
		//
		// [Ja] 挿入順は期待する並びともその逆とも異なるようにしてあり、挿入順をそのまま
		// 返すだけの結果では通らないようにしている。
		noon := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
		repos.createThreadPostedAt(t, ctx, first.ID, "tech の 2 番目", noon.Add(-time.Hour))
		repos.createThreadPostedAt(t, ctx, second.ID, "chat の最新", noon.Add(3*time.Hour))
		repos.createThreadPostedAt(t, ctx, first.ID, "tech の最新", noon)
		repos.createThreadPostedAt(t, ctx, first.ID, "tech の 3 番目", noon.Add(-2*time.Hour))

		threads, err := repos.thread.ListRecentPerBoard(ctx, 2)
		if err != nil {
			t.Fatalf("ListRecentPerBoard() error = %v", err)
		}

		wantTitles := []string{"tech の最新", "tech の 2 番目", "chat の最新"}
		if len(threads) != len(wantTitles) {
			t.Fatalf("len(ListRecentPerBoard()) = %d, want %d", len(threads), len(wantTitles))
		}
		for i, want := range wantTitles {
			if threads[i].Title != want {
				t.Errorf("ListRecentPerBoard()[%d].Title = %q, want %q", i, threads[i].Title, want)
			}
		}
	})

	t.Run("スレッドを持たない掲示板は 1 行も持たない", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newContentRepos(t)
		category := createCategory(t, ctx, repos.category, "general", 0)
		posted := createBoard(t, ctx, repos.board, &category.ID, "tech", 1)
		createBoard(t, ctx, repos.board, &category.ID, "quiet", 2)

		repos.createThreadPostedAt(t, ctx, posted.ID, "唯一のスレッド", time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC))

		threads, err := repos.thread.ListRecentPerBoard(ctx, 5)
		if err != nil {
			t.Fatalf("ListRecentPerBoard() error = %v", err)
		}
		if len(threads) != 1 {
			t.Fatalf("len(ListRecentPerBoard()) = %d, want %d", len(threads), 1)
		}
		if threads[0].BoardID != posted.ID {
			t.Errorf("ListRecentPerBoard()[0].BoardID = %v, want %v", threads[0].BoardID, posted.ID)
		}
	})

	t.Run("同じ position の板は id 順、同じ最終投稿時刻のスレッドは id の降順で上限まで返す", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newContentRepos(t)
		category := createCategory(t, ctx, repos.category, "general", 0)
		first := createBoard(t, ctx, repos.board, &category.ID, "first", 1)
		second := createBoard(t, ctx, repos.board, &category.ID, "second", 1)

		// All three threads on the first board share a timestamp, so the per-board
		// limit cuts through the tie and must retain the two greatest IDs. The
		// boards also share a position, so their groups must be ordered by ID.
		//
		// [Ja] 最初の掲示板にある 3 スレッドはすべて同じ時刻なので、掲示板ごとの上限が
		// 同順位の途中に入り、最大の 2 ID を残す必要があります。掲示板同士も position が
		// 同じなので、それぞれのグループは ID 順に並ぶ必要があります。
		sameTime := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
		repos.createThreadPostedAt(t, ctx, first.ID, "first の小さい id", sameTime)
		repos.createThreadPostedAt(t, ctx, first.ID, "first の中央の id", sameTime)
		repos.createThreadPostedAt(t, ctx, first.ID, "first の大きい id", sameTime)
		repos.createThreadPostedAt(t, ctx, second.ID, "second のスレッド", sameTime)

		threads, err := repos.thread.ListRecentPerBoard(ctx, 2)
		if err != nil {
			t.Fatalf("ListRecentPerBoard() error = %v", err)
		}

		want := []struct {
			boardID model.BoardID
			title   string
		}{
			{boardID: first.ID, title: "first の大きい id"},
			{boardID: first.ID, title: "first の中央の id"},
			{boardID: second.ID, title: "second のスレッド"},
		}
		if len(threads) != len(want) {
			t.Fatalf("len(ListRecentPerBoard()) = %d, want %d", len(threads), len(want))
		}
		for i, wantThread := range want {
			if threads[i].BoardID != wantThread.boardID {
				t.Errorf("ListRecentPerBoard()[%d].BoardID = %v, want %v", i, threads[i].BoardID, wantThread.boardID)
			}
			if threads[i].Title != wantThread.title {
				t.Errorf("ListRecentPerBoard()[%d].Title = %q, want %q", i, threads[i].Title, wantThread.title)
			}
		}
	})
}

func TestThreadRepository_UpdateLastPost(t *testing.T) {
	t.Parallel()

	repos, ctx := newContentRepos(t)
	board := repos.createBoardWithCategory(t, ctx, "tech")
	thread := repos.createThread(t, ctx, board.ID, "SQLite の話")
	repos.createPost(t, ctx, thread.ID, 1, "1 つ目")
	second := repos.createPost(t, ctx, thread.ID, 2, "2 つ目")

	lastPostedAt := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	err := repos.thread.UpdateLastPost(ctx, thread.ID, repository.UpdateThreadLastPostInput{
		PostsCount:   2,
		LastPostID:   second.ID,
		LastPostedAt: lastPostedAt,
	})
	if err != nil {
		t.Fatalf("UpdateLastPost() error = %v", err)
	}

	updated, err := repos.thread.FindByID(ctx, thread.ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}
	if updated == nil {
		t.Fatal("FindByID() = nil, want thread")
	}
	if updated.PostsCount != 2 {
		t.Errorf("thread.PostsCount = %d, want %d", updated.PostsCount, 2)
	}
	if updated.LastPostID == nil {
		t.Fatal("thread.LastPostID = nil, want the latest post")
	}
	if *updated.LastPostID != second.ID {
		t.Errorf("*thread.LastPostID = %v, want %v", *updated.LastPostID, second.ID)
	}
	if !updated.LastPostedAt.Equal(lastPostedAt) {
		t.Errorf("thread.LastPostedAt = %v, want %v", updated.LastPostedAt, lastPostedAt)
	}
}
