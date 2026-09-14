package repository_test

import (
	"context"
	"strings"
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
// author is left unset and the language fixed to Japanese because no assertion
// that uses this helper depends on either, which keeps a test's fixtures down to
// what it is actually about.
//
// [Ja] createThread は作者を持たないスレッドを挿入し、エラー時はテストを失敗させる。
// 作者を設定せず言語を日本語に固定するのは、このヘルパーを使う検証がどれもそれらに
// 依存しないためで、テストのフィクスチャをそのテストが実際に問うているものだけに保つ。
func (r *contentRepos) createThread(t *testing.T, ctx context.Context, boardID model.BoardID, title string) *model.Thread {
	t.Helper()

	thread, err := r.thread.Create(ctx, repository.CreateThreadInput{
		BoardID:  boardID,
		Title:    title,
		Language: model.LocaleJa.ThreadLanguage(),
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
		BoardID:  board.ID,
		UserID:   &userID,
		Title:    "SQLite の話",
		Language: model.LocaleJa.ThreadLanguage(),
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
	if thread.Language != model.LocaleJa.ThreadLanguage() {
		t.Errorf("thread.Language = %q, want %q", thread.Language, model.LocaleJa.ThreadLanguage())
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

// TestThreadRepository_Create_StoresEveryThreadLanguage verifies that each
// language a thread may be written in survives the round trip through the
// column, the one that resolves to no display language included. The value is
// read back through FindByID rather than taken from the inserted row alone, so
// the conversion every listing shares is covered as well.
//
// [Ja] TestThreadRepository_Create_StoresEveryThreadLanguage は、スレッドを書ける言語が
// どれも列を往復して保たれること — どの表示言語にも解決しない値を含む — を検証します。
// 値は挿入した行からだけでなく FindByID からも読み戻すため、各一覧が共有する変換も
// 併せて覆います。
func TestThreadRepository_Create_StoresEveryThreadLanguage(t *testing.T) {
	t.Parallel()

	repos, ctx := newContentRepos(t)
	board := repos.createBoardWithCategory(t, ctx, "tech")

	for _, language := range model.ThreadLanguages() {
		t.Run(string(language), func(t *testing.T) {
			created, err := repos.thread.Create(ctx, repository.CreateThreadInput{
				BoardID:  board.ID,
				Title:    "言語が " + string(language) + " のスレッド",
				Language: language,
			})
			if err != nil {
				t.Fatalf("Create() error = %v", err)
			}
			if created.Language != language {
				t.Errorf("Create() thread.Language = %q, want %q", created.Language, language)
			}

			found, err := repos.thread.FindByID(ctx, created.ID)
			if err != nil {
				t.Fatalf("FindByID() error = %v", err)
			}
			if found == nil {
				t.Fatal("FindByID() = nil, want thread")
			}
			if found.Language != language {
				t.Errorf("FindByID() thread.Language = %q, want %q", found.Language, language)
			}
		})
	}
}

// TestThreadRepository_Create_RejectsALanguageOutsideTheSet verifies that the
// check standing in for the CHECK the column does not carry refuses a value no
// thread may be written in, and that the refusal happens before the insert.
//
// The unset case is the one that would arise by accident: a caller that forgets
// the field passes the zero value, and without this check the board would hold a
// thread whose language names nothing.
//
// [Ja] TestThreadRepository_Create_RejectsALanguageOutsideTheSet は、列が持たない CHECK の
// 代わりを務める検査が、スレッドを書けない値を拒否すること、そしてその拒否が挿入より前に
// 起きることを検証します。
//
// 未設定の場合が、事故として起こりうるものです。フィールドを書き忘れた呼び出し側は
// ゼロ値を渡すことになり、この検査が無ければ掲示板は、言語が何も名指さないスレッドを
// 抱えることになります。
func TestThreadRepository_Create_RejectsALanguageOutsideTheSet(t *testing.T) {
	t.Parallel()

	repos, ctx := newContentRepos(t)
	board := repos.createBoardWithCategory(t, ctx, "tech")

	tests := []struct {
		name     string
		language model.ThreadLanguage
	}{
		{name: "アプリがロケールを持たない言語", language: "fr"},
		{name: "未設定", language: ""},
		{name: "地域のサブタグが付いたタグ", language: "ja-JP"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			thread, err := repos.thread.Create(ctx, repository.CreateThreadInput{
				BoardID:  board.ID,
				Title:    "受け付けられないスレッド",
				Language: tt.language,
			})
			if err == nil {
				t.Fatalf("Create() error = nil, want an error (language=%q)", tt.language)
			}
			if thread != nil {
				t.Errorf("Create() = %v, want nil", thread)
			}
		})
	}

	threads, err := repos.thread.ListByBoardID(ctx, board.ID)
	if err != nil {
		t.Fatalf("ListByBoardID() error = %v", err)
	}
	if len(threads) != 0 {
		t.Errorf("len(ListByBoardID()) = %d, want 0", len(threads))
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

// findThread reads the thread back through the repository, failing the test when
// it is gone. Assertions about a column a moderation write sets go through the
// lookup the application itself uses, so a write that lands in the database
// without reaching model.Thread does not pass.
//
// [Ja] findThreadはスレッドをリポジトリ経由で読み戻し、失われている場合はテストを
// 失敗させる。モデレーションの書き込みが立てる列についての検証を、アプリケーション自身が
// 使う引き当てを通して行うことで、データベースには届いてもmodel.Threadに現れない書き込みが
// 通らないようにする。
func (r *contentRepos) findThread(t *testing.T, ctx context.Context, id model.ThreadID) *model.Thread {
	t.Helper()

	thread, err := r.thread.FindByID(ctx, id)
	if err != nil {
		t.Fatalf("スレッドの取得に失敗: %v", err)
	}
	if thread == nil {
		t.Fatal("スレッドが見つからない")
	}

	return thread
}

func TestThreadRepository_Lock(t *testing.T) {
	t.Parallel()

	t.Run("ロックの時刻を立て、解除がそれを外す", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newContentRepos(t)
		board := repos.createBoardWithCategory(t, ctx, "tech")
		created := repos.createThread(t, ctx, board.ID, "ロックされるスレッド")

		if err := repos.thread.Lock(ctx, created.ID); err != nil {
			t.Fatalf("Lock() error = %v", err)
		}

		locked := repos.findThread(t, ctx, created.ID)
		if locked.LockedAt == nil {
			t.Fatal("thread.LockedAt = nil, want the stamped time")
		}
		if locked.LockedAt.Before(created.CreatedAt) {
			t.Errorf("thread.LockedAt = %v, want at or after the thread's creation (%v)", locked.LockedAt, created.CreatedAt)
		}

		if err := repos.thread.Unlock(ctx, created.ID); err != nil {
			t.Fatalf("Unlock() error = %v", err)
		}

		unlocked := repos.findThread(t, ctx, created.ID)
		if unlocked.LockedAt != nil {
			t.Errorf("thread.LockedAt = %v, want nil", unlocked.LockedAt)
		}
	})

	// The two marks are separate columns, so a thread carrying both is the case
	// that shows unlocking clears the one it is asked to and leaves the other
	// standing.
	//
	// [Ja] 2つの印は別々の列であるため、両方を持つスレッドこそが、解除が求められた
	// ほうだけを外し、もう一方をそのままにすることを示す場合になる。
	t.Run("解除が外すのはロックだけで、非公開の印は残る", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newContentRepos(t)
		board := repos.createBoardWithCategory(t, ctx, "tech")
		created := repos.createThread(t, ctx, board.ID, "ロックされて非公開にされたスレッド")

		if err := repos.thread.Lock(ctx, created.ID); err != nil {
			t.Fatalf("Lock() error = %v", err)
		}
		if err := repos.thread.Unpublish(ctx, created.ID); err != nil {
			t.Fatalf("Unpublish() error = %v", err)
		}
		if err := repos.thread.Unlock(ctx, created.ID); err != nil {
			t.Fatalf("Unlock() error = %v", err)
		}

		thread := repos.findThread(t, ctx, created.ID)
		if thread.LockedAt != nil {
			t.Errorf("thread.LockedAt = %v, want nil", thread.LockedAt)
		}
		if thread.UnpublishedAt == nil {
			t.Error("thread.UnpublishedAt = nil, want the stamped time")
		}
	})

	// The three writes name their row in a WHERE and read no affected count, so
	// an id matching nothing is a statement that changes nothing. A caller is
	// meant to have read the thread inside the transaction it writes in, and
	// this is what it gets when it did not: silence rather than an error. The
	// thread standing beside the missing id is checked too, so a WHERE that
	// stopped narrowing cannot pass.
	//
	// [Ja] 3つの書き込みはいずれもWHEREで行を名指し、更新した行数を読まないため、何にも
	// 一致しないidは何も変えない文になる。呼び出し元は書き込むトランザクションの中で
	// スレッドを読んでいるはずであり、読んでいなかったときに得るのがこれである。
	// エラーではなく沈黙である。存在しないidの隣にあるスレッドも確認することで、
	// WHEREが絞り込みをやめた場合に通らないようにする。
	t.Run("存在しない id への書き込みは何も変えずに成功する", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newContentRepos(t)
		board := repos.createBoardWithCategory(t, ctx, "tech")
		created := repos.createThread(t, ctx, board.ID, "無関係なスレッド")
		missing := created.ID + 100000

		if err := repos.thread.Lock(ctx, missing); err != nil {
			t.Errorf("Lock() error = %v, want nil", err)
		}
		if err := repos.thread.Unlock(ctx, missing); err != nil {
			t.Errorf("Unlock() error = %v, want nil", err)
		}
		if err := repos.thread.Unpublish(ctx, missing); err != nil {
			t.Errorf("Unpublish() error = %v, want nil", err)
		}

		thread := repos.findThread(t, ctx, created.ID)
		if thread.LockedAt != nil {
			t.Errorf("thread.LockedAt = %v, want nil", thread.LockedAt)
		}
		if thread.UnpublishedAt != nil {
			t.Errorf("thread.UnpublishedAt = %v, want nil", thread.UnpublishedAt)
		}
	})

	// Unlocking a thread that was never locked writes NULL over NULL, which is
	// the same silence: the caller learns nothing about what the column held.
	//
	// [Ja] ロックされていないスレッドの解除はNULLにNULLを書くものであり、同じ沈黙で
	// ある。呼び出し元は列が何を持っていたかを知らされない。
	t.Run("ロックされていないスレッドの解除も成功する", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newContentRepos(t)
		board := repos.createBoardWithCategory(t, ctx, "tech")
		created := repos.createThread(t, ctx, board.ID, "ロックされていないスレッド")

		if err := repos.thread.Unlock(ctx, created.ID); err != nil {
			t.Fatalf("Unlock() error = %v", err)
		}

		thread := repos.findThread(t, ctx, created.ID)
		if thread.LockedAt != nil {
			t.Errorf("thread.LockedAt = %v, want nil", thread.LockedAt)
		}
	})
}

func TestThreadRepository_Unpublish(t *testing.T) {
	t.Parallel()

	t.Run("非公開のスレッドも FindByID は返す", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newContentRepos(t)
		board := repos.createBoardWithCategory(t, ctx, "tech")
		created := repos.createThread(t, ctx, board.ID, "非公開にされるスレッド")

		if err := repos.thread.Unpublish(ctx, created.ID); err != nil {
			t.Fatalf("Unpublish() error = %v", err)
		}

		thread := repos.findThread(t, ctx, created.ID)
		if thread.UnpublishedAt == nil {
			t.Fatal("thread.UnpublishedAt = nil, want the stamped time")
		}
		if thread.Title != created.Title {
			t.Errorf("thread.Title = %q, want %q", thread.Title, created.Title)
		}
	})

	t.Run("非公開のスレッドは掲示板の一覧と各板の最新から消える", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newContentRepos(t)
		board := repos.createBoardWithCategory(t, ctx, "tech")
		noon := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
		hidden := repos.createThreadPostedAt(t, ctx, board.ID, "非公開にされるスレッド", noon.Add(time.Hour))
		repos.createThreadPostedAt(t, ctx, board.ID, "残るスレッド", noon)

		if err := repos.thread.Unpublish(ctx, hidden.ID); err != nil {
			t.Fatalf("Unpublish() error = %v", err)
		}

		listed, err := repos.thread.ListByBoardID(ctx, board.ID)
		if err != nil {
			t.Fatalf("ListByBoardID() error = %v", err)
		}
		if len(listed) != 1 {
			t.Fatalf("len(ListByBoardID()) = %d, want 1", len(listed))
		}
		if listed[0].Title != "残るスレッド" {
			t.Errorf("ListByBoardID()[0].Title = %q, want %q", listed[0].Title, "残るスレッド")
		}

		recent, err := repos.thread.ListRecentPerBoard(ctx, 5)
		if err != nil {
			t.Fatalf("ListRecentPerBoard() error = %v", err)
		}
		if len(recent) != 1 {
			t.Fatalf("len(ListRecentPerBoard()) = %d, want 1", len(recent))
		}
		if recent[0].Title != "残るスレッド" {
			t.Errorf("ListRecentPerBoard()[0].Title = %q, want %q", recent[0].Title, "残るスレッド")
		}
	})

	// perBoard is asked for below the number of candidates, which is what tells
	// apart leaving the unpublished thread out before the cut from leaving it
	// out after. Asked for with room to spare, both orders return the published
	// thread alone; asked for one, only the first returns anything at all, and
	// the other hands the board a slot spent on a thread nobody can see.
	//
	// [Ja] perBoardを候補の件数より小さく求める。これが、非公開のスレッドを切り出しの
	// 前に落とすことと後に落とすことを区別する。余裕を持って求めればどちらの順序でも
	// 公開のスレッド1件が返るが、1件だけを求めると前者しか何も返さず、後者は誰にも
	// 見えないスレッドに枠を使った掲示板を返す。
	t.Run("非公開のスレッドは各板の最新の枠を消費しない", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newContentRepos(t)
		board := repos.createBoardWithCategory(t, ctx, "tech")
		noon := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
		hidden := repos.createThreadPostedAt(t, ctx, board.ID, "最後に投稿された非公開のスレッド", noon.Add(time.Hour))
		repos.createThreadPostedAt(t, ctx, board.ID, "その1つ前の公開スレッド", noon)

		if err := repos.thread.Unpublish(ctx, hidden.ID); err != nil {
			t.Fatalf("Unpublish() error = %v", err)
		}

		recent, err := repos.thread.ListRecentPerBoard(ctx, 1)
		if err != nil {
			t.Fatalf("ListRecentPerBoard() error = %v", err)
		}
		if len(recent) != 1 {
			t.Fatalf("len(ListRecentPerBoard()) = %d, want 1", len(recent))
		}
		if recent[0].Title != "その1つ前の公開スレッド" {
			t.Errorf("ListRecentPerBoard()[0].Title = %q, want %q", recent[0].Title, "その1つ前の公開スレッド")
		}
	})
}

func TestThreadRepository_ListByIDs(t *testing.T) {
	t.Parallel()

	t.Run("id 順に返し、非公開のスレッドも含む", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newContentRepos(t)
		board := repos.createBoardWithCategory(t, ctx, "tech")
		first := repos.createThread(t, ctx, board.ID, "公開されたスレッド")
		second := repos.createThread(t, ctx, board.ID, "非公開にされるスレッド")

		if err := repos.thread.Unpublish(ctx, second.ID); err != nil {
			t.Fatalf("Unpublish() error = %v", err)
		}

		// The ids are asked for out of order and with a repeat, the shape a log
		// page hands over after collecting them from its rows.
		//
		// [Ja] idは順不同かつ重複を含めて渡す。履歴のページが自身の行から集めたときの
		// 形である。
		threads, err := repos.thread.ListByIDs(ctx, []model.ThreadID{second.ID, first.ID, second.ID})
		if err != nil {
			t.Fatalf("ListByIDs() error = %v", err)
		}
		if len(threads) != 2 {
			t.Fatalf("len(ListByIDs()) = %d, want 2", len(threads))
		}
		if threads[0].ID != first.ID {
			t.Errorf("ListByIDs()[0].ID = %v, want %v", threads[0].ID, first.ID)
		}
		if threads[1].ID != second.ID {
			t.Errorf("ListByIDs()[1].ID = %v, want %v", threads[1].ID, second.ID)
		}
		if threads[1].UnpublishedAt == nil {
			t.Error("ListByIDs()[1].UnpublishedAt = nil, want the stamped time")
		}
	})

	t.Run("存在しない id は結果に現れない", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newContentRepos(t)
		board := repos.createBoardWithCategory(t, ctx, "tech")
		created := repos.createThread(t, ctx, board.ID, "唯一のスレッド")

		threads, err := repos.thread.ListByIDs(ctx, []model.ThreadID{created.ID, created.ID + 100000})
		if err != nil {
			t.Fatalf("ListByIDs() error = %v", err)
		}
		if len(threads) != 1 {
			t.Fatalf("len(ListByIDs()) = %d, want 1", len(threads))
		}
		if threads[0].ID != created.ID {
			t.Errorf("ListByIDs()[0].ID = %v, want %v", threads[0].ID, created.ID)
		}
	})

	t.Run("空の id は空を返す", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newContentRepos(t)

		threads, err := repos.thread.ListByIDs(ctx, nil)
		if err != nil {
			t.Fatalf("ListByIDs() error = %v", err)
		}
		if len(threads) != 0 {
			t.Errorf("len(ListByIDs()) = %d, want 0", len(threads))
		}
	})
}

// TestThreadRepository_Listings_KeepTheirSearchWithoutTheUnpublishedThreads
// verifies that leaving unpublished threads out of the two listings does not
// change how SQLite finds their rows. The condition is on no index, so it can
// only be applied to rows something else already found; the check is that the
// something else is unchanged, by comparing each statement's plan against the
// same statement with the condition removed.
//
// The one difference the comparison forgives is the index no longer covering
// the per-board subquery, since unpublished_at is not in it: that subquery now
// reads the row for each candidate it walks, including unpublished ones.
// LIMIT bounds the returned rows; finding enough published rows can require
// reading many unpublished candidates, or the entire board's range when too
// few published rows remain. This test checks the search plan, not a bound on
// the number of rows read.
//
// [Ja] TestThreadRepository_Listings_KeepTheirSearchWithoutTheUnpublishedThreadsは、
// 2つの一覧から非公開のスレッドを落とすことが、SQLiteのそれらの行の見つけ方を変えない
// ことを検証します。この条件はどの索引にも乗らないため、ほかの何かが既に見つけた行に
// 対してしか適用できません。検査するのは、そのほかの何かが変わっていないことであり、
// 各文の実行計画を、条件を取り除いた同じ文の実行計画と比べます。
//
// 比較が見逃す唯一の違いは、掲示板ごとの副問い合わせを索引が覆わなくなることです。
// unpublished_atが索引に含まれないためで、この副問い合わせは非公開を含む候補ごとに行を
// 読みます。LIMITが制限するのは返却件数であり、公開行が必要な件数に達するまでに多数の
// 非公開行を読むことも、公開行が不足して掲示板の範囲全体を読むこともあります。
// このテストは検索の形を確認するもので、読み取る行数の上限を検証するものではありません。
func TestThreadRepository_Listings_KeepTheirSearchWithoutTheUnpublishedThreads(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		query     string
		condition string
		arg       int64
	}{
		{
			name:      "掲示板のスレッド一覧",
			query:     "ListThreadsByBoardID",
			condition: " AND unpublished_at IS NULL",
			arg:       1,
		},
		{
			name:      "各掲示板の最新スレッド",
			query:     "ListRecentThreadsPerBoard",
			condition: " AND recent.unpublished_at IS NULL",
			arg:       5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repos, ctx := newContentRepos(t)

			statement := queryStatement(t, "threads.sql", tt.query)
			if !strings.Contains(statement, tt.condition) {
				t.Fatalf("%s に %q が見つからない", tt.query, tt.condition)
			}
			published := strings.Replace(statement, tt.condition, "", 1)

			got := searchPlan(t, ctx, repos.db, statement, tt.arg)
			want := searchPlan(t, ctx, repos.db, published, tt.arg)
			if got != want {
				t.Errorf("%s の実行計画 = %q, 条件を除いた %q と同じはず", tt.query, got, want)
			}
			if !strings.Contains(got, "index_threads_on_board_id_and_last_posted_at") {
				t.Errorf("%s の実行計画 = %q, 掲示板と最終投稿の索引をたどるはず", tt.query, got)
			}
		})
	}
}

// searchPlan returns the query plan with the distinction between an index and a
// covering one dropped, so that a comparison of two plans is about which index
// answers the search rather than whether the rows it found still have to be
// read.
//
// [Ja] searchPlanは、索引とそれが覆う索引の区別を落とした実行計画を返し、2つの実行計画の
// 比較が、見つけた行をさらに読む必要があるかどうかではなく、どの索引が検索に答えるかに
// ついてのものになるようにします。
func searchPlan(t *testing.T, ctx context.Context, db *database.DB, statement string, args ...any) string {
	t.Helper()

	return strings.ReplaceAll(queryPlan(t, ctx, db, statement, args...), "USING COVERING INDEX", "USING INDEX")
}
