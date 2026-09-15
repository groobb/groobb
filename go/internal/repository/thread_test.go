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

// contentReposは、テストが所有する1つのデータベース上に、コミュニティの中身を
// 組み立てるリポジトリ群をまとめる。どれも単独では成り立たない。スレッドには掲示板が、
// 掲示板にはカテゴリーが、投稿にはスレッドが、参照には2つの投稿が要るため、どれを
// 検証するテストもその下にあるものをすべて作ることになる。
type contentRepos struct {
	db            *database.DB
	category      *repository.CategoryRepository
	board         *repository.BoardRepository
	thread        *repository.ThreadRepository
	post          *repository.PostRepository
	postReference *repository.PostReferenceRepository
}

// newContentReposは新しいデータベース上にリポジトリ群を作る。
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

// createBoardWithCategoryは掲示板を、それが属さなければならないカテゴリーと
// 一緒に作る。掲示板そのものではなく掲示板の中身を問うテストのためのものである。
func (r *contentRepos) createBoardWithCategory(t *testing.T, ctx context.Context, slug string) *model.Board {
	t.Helper()

	category := createCategory(t, ctx, r.category, slug+"-category", 0)
	return createBoard(t, ctx, r.board, &category.ID, slug, 0)
}

// createThreadは作者を持たないスレッドを挿入し、エラー時はテストを失敗させる。
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

// createThreadPostedAtは、実際にスレッドが存在する形 — 最初の投稿を伴い、非正規化
// 列がその投稿を表している状態 — でスレッドを作る。スレッドごとに異なる最終投稿時刻を
// 与えたいテストが必要とするものである。
func (r *contentRepos) createThreadPostedAt(t *testing.T, ctx context.Context, boardID model.BoardID, title string, lastPostedAt time.Time) *model.Thread {
	t.Helper()

	thread := r.createThread(t, ctx, boardID, title)
	post := r.createPost(t, ctx, thread.ID, 1, title+"の1つ目の投稿")

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
		Title:    "SQLiteの話",
		Language: model.LocaleJa.ThreadLanguage(),
	})
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}

	if thread.ID == 0 {
		t.Error("Create() thread.IDはDB採番で空でないはず")
	}
	if thread.BoardID != board.ID {
		t.Errorf("thread.BoardID = %v、期待値 = %v", thread.BoardID, board.ID)
	}
	if thread.UserID == nil {
		t.Fatal("thread.UserID = nil、期待値は作成者")
	}
	if *thread.UserID != userID {
		t.Errorf("*thread.UserID = %v、期待値 = %v", *thread.UserID, userID)
	}
	if thread.Title != "SQLiteの話" {
		t.Errorf("thread.Title = %q、期待値 = %q", thread.Title, "SQLiteの話")
	}
	if thread.Language != model.LocaleJa.ThreadLanguage() {
		t.Errorf("thread.Language = %q、期待値 = %q", thread.Language, model.LocaleJa.ThreadLanguage())
	}
	if thread.PostsCount != 0 {
		t.Errorf("thread.PostsCount = %d、期待値 = %d", thread.PostsCount, 0)
	}
	if thread.LastPostID != nil {
		t.Errorf("thread.LastPostID = %v、期待値 = nil", *thread.LastPostID)
	}
	if thread.LastPostedAt.IsZero() {
		t.Error("thread.LastPostedAtはDB既定値で設定されるはず")
	}
	if thread.CreatedAt.IsZero() {
		t.Error("thread.CreatedAtはDB既定値で設定されるはず")
	}
	if thread.UpdatedAt.IsZero() {
		t.Error("thread.UpdatedAtはDB既定値で設定されるはず")
	}
}

func TestThreadRepository_Create_LeavesAuthorUnsetForAWithdrawnUser(t *testing.T) {
	t.Parallel()

	repos, ctx := newContentRepos(t)
	board := repos.createBoardWithCategory(t, ctx, "tech")

	thread := repos.createThread(t, ctx, board.ID, "退会した人が立てたスレッド")

	if thread.UserID != nil {
		t.Errorf("thread.UserID = %v、期待値 = nil", *thread.UserID)
	}
}

// TestThreadRepository_Create_StoresEveryThreadLanguageは、スレッドを書ける言語が
// どれも列を往復して保たれること — どの表示言語にも解決しない値を含む — を検証します。
// 値は挿入した行からだけでなくFindByIDからも読み戻すため、各一覧が共有する変換も
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
				t.Fatalf("Create()のエラー = %v", err)
			}
			if created.Language != language {
				t.Errorf("Create() thread.Language = %q、期待値 = %q", created.Language, language)
			}

			found, err := repos.thread.FindByID(ctx, created.ID)
			if err != nil {
				t.Fatalf("FindByID()のエラー = %v", err)
			}
			if found == nil {
				t.Fatal("FindByID() = nil、期待値はスレッド")
			}
			if found.Language != language {
				t.Errorf("FindByID() thread.Language = %q、期待値 = %q", found.Language, language)
			}
		})
	}
}

// TestThreadRepository_Create_RejectsALanguageOutsideTheSetは、列が持たないCHECKの
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
				t.Fatalf("Create()のエラー = nil、期待値はエラー (language=%q)", tt.language)
			}
			if thread != nil {
				t.Errorf("Create() = %v、期待値 = nil", thread)
			}
		})
	}

	threads, err := repos.thread.ListByBoardID(ctx, board.ID)
	if err != nil {
		t.Fatalf("ListByBoardID()のエラー = %v", err)
	}
	if len(threads) != 0 {
		t.Errorf("len(ListByBoardID()) = %d、期待値 = 0", len(threads))
	}
}

func TestThreadRepository_FindByID(t *testing.T) {
	t.Parallel()

	repos, ctx := newContentRepos(t)
	board := repos.createBoardWithCategory(t, ctx, "tech")
	created := repos.createThread(t, ctx, board.ID, "SQLiteの話")

	t.Run("idでスレッドを取得できる", func(t *testing.T) {
		thread, err := repos.thread.FindByID(ctx, created.ID)
		if err != nil {
			t.Fatalf("FindByID()のエラー = %v", err)
		}
		if thread == nil {
			t.Fatal("FindByID() = nil、期待値はスレッド")
		}
		if thread.ID != created.ID {
			t.Errorf("thread.ID = %v、期待値 = %v", thread.ID, created.ID)
		}
		if thread.Title != created.Title {
			t.Errorf("thread.Title = %q、期待値 = %q", thread.Title, created.Title)
		}
	})

	t.Run("存在しないidは (nil, nil) を返す", func(t *testing.T) {
		thread, err := repos.thread.FindByID(ctx, created.ID+1)
		if err != nil {
			t.Fatalf("FindByID()のエラー = %v、期待値 = nil", err)
		}
		if thread != nil {
			t.Errorf("FindByID() = %v、期待値 = nil", thread)
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

		// 挿入順は期待する並びともその逆とも異なるようにしてあり、挿入順をそのまま
		// 返すだけの結果では通らないようにしている。
		noon := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
		repos.createThreadPostedAt(t, ctx, board.ID, "昼のスレッド", noon)
		repos.createThreadPostedAt(t, ctx, board.ID, "朝のスレッド", noon.Add(-3*time.Hour))
		repos.createThreadPostedAt(t, ctx, other.ID, "別の板のスレッド", noon.Add(time.Hour))
		repos.createThreadPostedAt(t, ctx, board.ID, "夜のスレッド", noon.Add(6*time.Hour))
		repos.createThreadPostedAt(t, ctx, board.ID, "昼のもう1つのスレッド", noon)

		threads, err := repos.thread.ListByBoardID(ctx, board.ID)
		if err != nil {
			t.Fatalf("ListByBoardID()のエラー = %v", err)
		}

		wantTitles := []string{"夜のスレッド", "昼のもう1つのスレッド", "昼のスレッド", "朝のスレッド"}
		if len(threads) != len(wantTitles) {
			t.Fatalf("len(ListByBoardID()) = %d、期待値 = %d", len(threads), len(wantTitles))
		}
		for i, want := range wantTitles {
			if threads[i].Title != want {
				t.Errorf("ListByBoardID()[%d].Title = %q、期待値 = %q", i, threads[i].Title, want)
			}
		}
	})

	t.Run("スレッドを持たない掲示板は空を返す", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newContentRepos(t)
		board := repos.createBoardWithCategory(t, ctx, "tech")

		threads, err := repos.thread.ListByBoardID(ctx, board.ID)
		if err != nil {
			t.Fatalf("ListByBoardID()のエラー = %v", err)
		}
		if len(threads) != 0 {
			t.Errorf("len(ListByBoardID()) = %d、期待値 = 0", len(threads))
		}
	})
}

func TestThreadRepository_ListRecentPerBoard(t *testing.T) {
	t.Parallel()

	t.Run("掲示板ごとに指定件数までを、板はposition順・スレッドは最終投稿が新しい順で返す", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newContentRepos(t)
		category := createCategory(t, ctx, repos.category, "general", 0)
		second := createBoard(t, ctx, repos.board, &category.ID, "chat", 2)
		first := createBoard(t, ctx, repos.board, &category.ID, "tech", 1)

		// 挿入順は期待する並びともその逆とも異なるようにしてあり、挿入順をそのまま
		// 返すだけの結果では通らないようにしている。
		noon := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
		repos.createThreadPostedAt(t, ctx, first.ID, "techの2番目", noon.Add(-time.Hour))
		repos.createThreadPostedAt(t, ctx, second.ID, "chatの最新", noon.Add(3*time.Hour))
		repos.createThreadPostedAt(t, ctx, first.ID, "techの最新", noon)
		repos.createThreadPostedAt(t, ctx, first.ID, "techの3番目", noon.Add(-2*time.Hour))

		threads, err := repos.thread.ListRecentPerBoard(ctx, 2)
		if err != nil {
			t.Fatalf("ListRecentPerBoard()のエラー = %v", err)
		}

		wantTitles := []string{"techの最新", "techの2番目", "chatの最新"}
		if len(threads) != len(wantTitles) {
			t.Fatalf("len(ListRecentPerBoard()) = %d、期待値 = %d", len(threads), len(wantTitles))
		}
		for i, want := range wantTitles {
			if threads[i].Title != want {
				t.Errorf("ListRecentPerBoard()[%d].Title = %q、期待値 = %q", i, threads[i].Title, want)
			}
		}
	})

	t.Run("スレッドを持たない掲示板は1行も持たない", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newContentRepos(t)
		category := createCategory(t, ctx, repos.category, "general", 0)
		posted := createBoard(t, ctx, repos.board, &category.ID, "tech", 1)
		createBoard(t, ctx, repos.board, &category.ID, "quiet", 2)

		repos.createThreadPostedAt(t, ctx, posted.ID, "唯一のスレッド", time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC))

		threads, err := repos.thread.ListRecentPerBoard(ctx, 5)
		if err != nil {
			t.Fatalf("ListRecentPerBoard()のエラー = %v", err)
		}
		if len(threads) != 1 {
			t.Fatalf("len(ListRecentPerBoard()) = %d、期待値 = %d", len(threads), 1)
		}
		if threads[0].BoardID != posted.ID {
			t.Errorf("ListRecentPerBoard()[0].BoardID = %v、期待値 = %v", threads[0].BoardID, posted.ID)
		}
	})

	t.Run("同じpositionの板はid順、同じ最終投稿時刻のスレッドはidの降順で上限まで返す", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newContentRepos(t)
		category := createCategory(t, ctx, repos.category, "general", 0)
		first := createBoard(t, ctx, repos.board, &category.ID, "first", 1)
		second := createBoard(t, ctx, repos.board, &category.ID, "second", 1)

		// 最初の掲示板にある3スレッドはすべて同じ時刻なので、掲示板ごとの上限が
		// 同順位の途中に入り、最大の2 IDを残す必要があります。掲示板同士もpositionが
		// 同じなので、それぞれのグループはID順に並ぶ必要があります。
		sameTime := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
		repos.createThreadPostedAt(t, ctx, first.ID, "firstの小さいid", sameTime)
		repos.createThreadPostedAt(t, ctx, first.ID, "firstの中央のid", sameTime)
		repos.createThreadPostedAt(t, ctx, first.ID, "firstの大きいid", sameTime)
		repos.createThreadPostedAt(t, ctx, second.ID, "secondのスレッド", sameTime)

		threads, err := repos.thread.ListRecentPerBoard(ctx, 2)
		if err != nil {
			t.Fatalf("ListRecentPerBoard()のエラー = %v", err)
		}

		want := []struct {
			boardID model.BoardID
			title   string
		}{
			{boardID: first.ID, title: "firstの大きいid"},
			{boardID: first.ID, title: "firstの中央のid"},
			{boardID: second.ID, title: "secondのスレッド"},
		}
		if len(threads) != len(want) {
			t.Fatalf("len(ListRecentPerBoard()) = %d、期待値 = %d", len(threads), len(want))
		}
		for i, wantThread := range want {
			if threads[i].BoardID != wantThread.boardID {
				t.Errorf("ListRecentPerBoard()[%d].BoardID = %v、期待値 = %v", i, threads[i].BoardID, wantThread.boardID)
			}
			if threads[i].Title != wantThread.title {
				t.Errorf("ListRecentPerBoard()[%d].Title = %q、期待値 = %q", i, threads[i].Title, wantThread.title)
			}
		}
	})
}

func TestThreadRepository_UpdateLastPost(t *testing.T) {
	t.Parallel()

	repos, ctx := newContentRepos(t)
	board := repos.createBoardWithCategory(t, ctx, "tech")
	thread := repos.createThread(t, ctx, board.ID, "SQLiteの話")
	repos.createPost(t, ctx, thread.ID, 1, "1つ目")
	second := repos.createPost(t, ctx, thread.ID, 2, "2つ目")

	lastPostedAt := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	err := repos.thread.UpdateLastPost(ctx, thread.ID, repository.UpdateThreadLastPostInput{
		PostsCount:   2,
		LastPostID:   second.ID,
		LastPostedAt: lastPostedAt,
	})
	if err != nil {
		t.Fatalf("UpdateLastPost()のエラー = %v", err)
	}

	updated, err := repos.thread.FindByID(ctx, thread.ID)
	if err != nil {
		t.Fatalf("FindByID()のエラー = %v", err)
	}
	if updated == nil {
		t.Fatal("FindByID() = nil、期待値はスレッド")
	}
	if updated.PostsCount != 2 {
		t.Errorf("thread.PostsCount = %d、期待値 = %d", updated.PostsCount, 2)
	}
	if updated.LastPostID == nil {
		t.Fatal("thread.LastPostID = nil、期待値は最新の投稿")
	}
	if *updated.LastPostID != second.ID {
		t.Errorf("*thread.LastPostID = %v、期待値 = %v", *updated.LastPostID, second.ID)
	}
	if !updated.LastPostedAt.Equal(lastPostedAt) {
		t.Errorf("thread.LastPostedAt = %v、期待値 = %v", updated.LastPostedAt, lastPostedAt)
	}
}

// findThreadはスレッドをリポジトリ経由で読み戻し、失われている場合はテストを
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
			t.Fatalf("Lock()のエラー = %v", err)
		}

		locked := repos.findThread(t, ctx, created.ID)
		if locked.LockedAt == nil {
			t.Fatal("thread.LockedAt = nil、期待値は打刻された時刻")
		}
		if locked.LockedAt.Before(created.CreatedAt) {
			t.Errorf("thread.LockedAt = %v、期待値はスレッドの作成時刻 (%v) 以降", locked.LockedAt, created.CreatedAt)
		}

		if err := repos.thread.Unlock(ctx, created.ID); err != nil {
			t.Fatalf("Unlock()のエラー = %v", err)
		}

		unlocked := repos.findThread(t, ctx, created.ID)
		if unlocked.LockedAt != nil {
			t.Errorf("thread.LockedAt = %v、期待値 = nil", unlocked.LockedAt)
		}
	})

	// 2つの印は別々の列であるため、両方を持つスレッドこそが、解除が求められた
	// ほうだけを外し、もう一方をそのままにすることを示す場合になる。
	t.Run("解除が外すのはロックだけで、非公開の印は残る", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newContentRepos(t)
		board := repos.createBoardWithCategory(t, ctx, "tech")
		created := repos.createThread(t, ctx, board.ID, "ロックされて非公開にされたスレッド")

		if err := repos.thread.Lock(ctx, created.ID); err != nil {
			t.Fatalf("Lock()のエラー = %v", err)
		}
		if err := repos.thread.Unpublish(ctx, created.ID); err != nil {
			t.Fatalf("Unpublish()のエラー = %v", err)
		}
		if err := repos.thread.Unlock(ctx, created.ID); err != nil {
			t.Fatalf("Unlock()のエラー = %v", err)
		}

		thread := repos.findThread(t, ctx, created.ID)
		if thread.LockedAt != nil {
			t.Errorf("thread.LockedAt = %v、期待値 = nil", thread.LockedAt)
		}
		if thread.UnpublishedAt == nil {
			t.Error("thread.UnpublishedAt = nil、期待値は打刻された時刻")
		}
	})

	// 3つの書き込みはいずれもWHEREで行を名指し、更新した行数を読まないため、何にも
	// 一致しないidは何も変えない文になる。呼び出し元は書き込むトランザクションの中で
	// スレッドを読んでいるはずであり、読んでいなかったときに得るのがこれである。
	// エラーではなく沈黙である。存在しないidの隣にあるスレッドも確認することで、
	// WHEREが絞り込みをやめた場合に通らないようにする。
	t.Run("存在しないidへの書き込みは何も変えずに成功する", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newContentRepos(t)
		board := repos.createBoardWithCategory(t, ctx, "tech")
		created := repos.createThread(t, ctx, board.ID, "無関係なスレッド")
		missing := created.ID + 100000

		if err := repos.thread.Lock(ctx, missing); err != nil {
			t.Errorf("Lock()のエラー = %v、期待値 = nil", err)
		}
		if err := repos.thread.Unlock(ctx, missing); err != nil {
			t.Errorf("Unlock()のエラー = %v、期待値 = nil", err)
		}
		if err := repos.thread.Unpublish(ctx, missing); err != nil {
			t.Errorf("Unpublish()のエラー = %v、期待値 = nil", err)
		}

		thread := repos.findThread(t, ctx, created.ID)
		if thread.LockedAt != nil {
			t.Errorf("thread.LockedAt = %v、期待値 = nil", thread.LockedAt)
		}
		if thread.UnpublishedAt != nil {
			t.Errorf("thread.UnpublishedAt = %v、期待値 = nil", thread.UnpublishedAt)
		}
	})

	// ロックされていないスレッドの解除はNULLにNULLを書くものであり、同じ沈黙で
	// ある。呼び出し元は列が何を持っていたかを知らされない。
	t.Run("ロックされていないスレッドの解除も成功する", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newContentRepos(t)
		board := repos.createBoardWithCategory(t, ctx, "tech")
		created := repos.createThread(t, ctx, board.ID, "ロックされていないスレッド")

		if err := repos.thread.Unlock(ctx, created.ID); err != nil {
			t.Fatalf("Unlock()のエラー = %v", err)
		}

		thread := repos.findThread(t, ctx, created.ID)
		if thread.LockedAt != nil {
			t.Errorf("thread.LockedAt = %v、期待値 = nil", thread.LockedAt)
		}
	})
}

func TestThreadRepository_Unpublish(t *testing.T) {
	t.Parallel()

	t.Run("非公開のスレッドもFindByIDは返す", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newContentRepos(t)
		board := repos.createBoardWithCategory(t, ctx, "tech")
		created := repos.createThread(t, ctx, board.ID, "非公開にされるスレッド")

		if err := repos.thread.Unpublish(ctx, created.ID); err != nil {
			t.Fatalf("Unpublish()のエラー = %v", err)
		}

		thread := repos.findThread(t, ctx, created.ID)
		if thread.UnpublishedAt == nil {
			t.Fatal("thread.UnpublishedAt = nil、期待値は打刻された時刻")
		}
		if thread.Title != created.Title {
			t.Errorf("thread.Title = %q、期待値 = %q", thread.Title, created.Title)
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
			t.Fatalf("Unpublish()のエラー = %v", err)
		}

		listed, err := repos.thread.ListByBoardID(ctx, board.ID)
		if err != nil {
			t.Fatalf("ListByBoardID()のエラー = %v", err)
		}
		if len(listed) != 1 {
			t.Fatalf("len(ListByBoardID()) = %d、期待値 = 1", len(listed))
		}
		if listed[0].Title != "残るスレッド" {
			t.Errorf("ListByBoardID()[0].Title = %q、期待値 = %q", listed[0].Title, "残るスレッド")
		}

		recent, err := repos.thread.ListRecentPerBoard(ctx, 5)
		if err != nil {
			t.Fatalf("ListRecentPerBoard()のエラー = %v", err)
		}
		if len(recent) != 1 {
			t.Fatalf("len(ListRecentPerBoard()) = %d、期待値 = 1", len(recent))
		}
		if recent[0].Title != "残るスレッド" {
			t.Errorf("ListRecentPerBoard()[0].Title = %q、期待値 = %q", recent[0].Title, "残るスレッド")
		}
	})

	// perBoardを候補の件数より小さく求める。これが、非公開のスレッドを切り出しの
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
			t.Fatalf("Unpublish()のエラー = %v", err)
		}

		recent, err := repos.thread.ListRecentPerBoard(ctx, 1)
		if err != nil {
			t.Fatalf("ListRecentPerBoard()のエラー = %v", err)
		}
		if len(recent) != 1 {
			t.Fatalf("len(ListRecentPerBoard()) = %d、期待値 = 1", len(recent))
		}
		if recent[0].Title != "その1つ前の公開スレッド" {
			t.Errorf("ListRecentPerBoard()[0].Title = %q、期待値 = %q", recent[0].Title, "その1つ前の公開スレッド")
		}
	})
}

func TestThreadRepository_ListByIDs(t *testing.T) {
	t.Parallel()

	t.Run("id順に返し、非公開のスレッドも含む", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newContentRepos(t)
		board := repos.createBoardWithCategory(t, ctx, "tech")
		first := repos.createThread(t, ctx, board.ID, "公開されたスレッド")
		second := repos.createThread(t, ctx, board.ID, "非公開にされるスレッド")

		if err := repos.thread.Unpublish(ctx, second.ID); err != nil {
			t.Fatalf("Unpublish()のエラー = %v", err)
		}

		// idは順不同かつ重複を含めて渡す。履歴のページが自身の行から集めたときの
		// 形である。
		threads, err := repos.thread.ListByIDs(ctx, []model.ThreadID{second.ID, first.ID, second.ID})
		if err != nil {
			t.Fatalf("ListByIDs()のエラー = %v", err)
		}
		if len(threads) != 2 {
			t.Fatalf("len(ListByIDs()) = %d、期待値 = 2", len(threads))
		}
		if threads[0].ID != first.ID {
			t.Errorf("ListByIDs()[0].ID = %v、期待値 = %v", threads[0].ID, first.ID)
		}
		if threads[1].ID != second.ID {
			t.Errorf("ListByIDs()[1].ID = %v、期待値 = %v", threads[1].ID, second.ID)
		}
		if threads[1].UnpublishedAt == nil {
			t.Error("ListByIDs()[1].UnpublishedAt = nil、期待値は打刻された時刻")
		}
	})

	t.Run("存在しないidは結果に現れない", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newContentRepos(t)
		board := repos.createBoardWithCategory(t, ctx, "tech")
		created := repos.createThread(t, ctx, board.ID, "唯一のスレッド")

		threads, err := repos.thread.ListByIDs(ctx, []model.ThreadID{created.ID, created.ID + 100000})
		if err != nil {
			t.Fatalf("ListByIDs()のエラー = %v", err)
		}
		if len(threads) != 1 {
			t.Fatalf("len(ListByIDs()) = %d、期待値 = 1", len(threads))
		}
		if threads[0].ID != created.ID {
			t.Errorf("ListByIDs()[0].ID = %v、期待値 = %v", threads[0].ID, created.ID)
		}
	})

	t.Run("空のidは空を返す", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newContentRepos(t)

		threads, err := repos.thread.ListByIDs(ctx, nil)
		if err != nil {
			t.Fatalf("ListByIDs()のエラー = %v", err)
		}
		if len(threads) != 0 {
			t.Errorf("len(ListByIDs()) = %d、期待値 = 0", len(threads))
		}
	})
}

// TestThreadRepository_Listings_KeepTheirSearchWithoutTheUnpublishedThreadsは、
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
				t.Errorf("%s の実行計画 = %q、条件を除いた %q と同じはず", tt.query, got, want)
			}
			if !strings.Contains(got, "index_threads_on_board_id_and_last_posted_at") {
				t.Errorf("%s の実行計画 = %q、掲示板と最終投稿の索引をたどるはず", tt.query, got)
			}
		})
	}
}

// searchPlanは、索引とそれが覆う索引の区別を落とした実行計画を返し、2つの実行計画の
// 比較が、見つけた行をさらに読む必要があるかどうかではなく、どの索引が検索に答えるかに
// ついてのものになるようにします。
func searchPlan(t *testing.T, ctx context.Context, db *database.DB, statement string, args ...any) string {
	t.Helper()

	return strings.ReplaceAll(queryPlan(t, ctx, db, statement, args...), "USING COVERING INDEX", "USING INDEX")
}
