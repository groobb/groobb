package repository_test

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
)

// createPost inserts a post with no author, failing the test on error. The
// author is left unset because no assertion that uses this helper depends on it.
//
// [Ja] createPost は作者を持たない投稿を挿入し、エラー時はテストを失敗させる。作者を
// 設定しないのは、このヘルパーを使う検証がどれもそれに依存しないためである。
func (r *contentRepos) createPost(t *testing.T, ctx context.Context, threadID model.ThreadID, number int, body string) *model.Post {
	t.Helper()

	post, err := r.post.Create(ctx, repository.CreatePostInput{
		ThreadID: threadID,
		Number:   number,
		Body:     body,
	})
	if err != nil {
		t.Fatalf("テスト用投稿の作成に失敗: %v", err)
	}

	return post
}

func TestPostRepository_Create(t *testing.T) {
	t.Parallel()

	repos, ctx := newContentRepos(t)
	board := repos.createBoardWithCategory(t, ctx, "tech")
	thread := repos.createThread(t, ctx, board.ID, "SQLite の話")
	userID := testutil.NewUserBuilder(t, repos.db).Build()

	post, err := repos.post.Create(ctx, repository.CreatePostInput{
		ThreadID: thread.ID,
		UserID:   &userID,
		Number:   1,
		Body:     ">>1 に返信する本文",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if post.ID == 0 {
		t.Error("Create() post.ID は DB 採番で空でないはず")
	}
	if post.ThreadID != thread.ID {
		t.Errorf("post.ThreadID = %v, want %v", post.ThreadID, thread.ID)
	}
	if post.UserID == nil {
		t.Fatal("post.UserID = nil, want the author")
	}
	if *post.UserID != userID {
		t.Errorf("*post.UserID = %v, want %v", *post.UserID, userID)
	}
	if post.Number != 1 {
		t.Errorf("post.Number = %d, want %d", post.Number, 1)
	}
	if post.Body != ">>1 に返信する本文" {
		t.Errorf("post.Body = %q, want %q", post.Body, ">>1 に返信する本文")
	}
	if post.CreatedAt.IsZero() {
		t.Error("post.CreatedAt は DB 既定値で設定されるはず")
	}
	if post.UpdatedAt.IsZero() {
		t.Error("post.UpdatedAt は DB 既定値で設定されるはず")
	}
}

func TestPostRepository_Create_LeavesAuthorUnsetForAWithdrawnUser(t *testing.T) {
	t.Parallel()

	repos, ctx := newContentRepos(t)
	board := repos.createBoardWithCategory(t, ctx, "tech")
	thread := repos.createThread(t, ctx, board.ID, "SQLite の話")

	post := repos.createPost(t, ctx, thread.ID, 1, "退会した人の投稿")

	if post.UserID != nil {
		t.Errorf("post.UserID = %v, want nil", *post.UserID)
	}
}

func TestPostRepository_ListByThreadID(t *testing.T) {
	t.Parallel()

	t.Run("そのスレッドの投稿だけをレス番号順で返す", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newContentRepos(t)
		board := repos.createBoardWithCategory(t, ctx, "tech")
		thread := repos.createThread(t, ctx, board.ID, "SQLite の話")
		other := repos.createThread(t, ctx, board.ID, "別のスレッド")

		// The insertion order is deliberately not the reply-number order, so a
		// result that merely echoes it cannot pass.
		//
		// [Ja] 挿入順はレス番号の順と異なるようにしてあり、挿入順をそのまま返すだけの
		// 結果では通らないようにしている。
		repos.createPost(t, ctx, thread.ID, 2, "2 つ目")
		repos.createPost(t, ctx, other.ID, 1, "別のスレッドの投稿")
		repos.createPost(t, ctx, thread.ID, 3, "3 つ目")
		repos.createPost(t, ctx, thread.ID, 1, "1 つ目")

		posts, err := repos.post.ListByThreadID(ctx, thread.ID)
		if err != nil {
			t.Fatalf("ListByThreadID() error = %v", err)
		}

		wantNumbers := []int{1, 2, 3}
		if len(posts) != len(wantNumbers) {
			t.Fatalf("len(ListByThreadID()) = %d, want %d", len(posts), len(wantNumbers))
		}
		for i, want := range wantNumbers {
			if posts[i].Number != want {
				t.Errorf("ListByThreadID()[%d].Number = %d, want %d", i, posts[i].Number, want)
			}
		}
	})

	t.Run("非公開の投稿もレス番号順で残り、本文と非公開の印を保持する", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newContentRepos(t)
		board := repos.createBoardWithCategory(t, ctx, "tech")
		thread := repos.createThread(t, ctx, board.ID, "非公開の投稿があるスレッド")
		second := repos.createPost(t, ctx, thread.ID, 2, "非公開にされる本文")
		third := repos.createPost(t, ctx, thread.ID, 3, "3つ目の本文")
		first := repos.createPost(t, ctx, thread.ID, 1, "1つ目の本文")

		if err := repos.post.Unpublish(ctx, second.ID); err != nil {
			t.Fatalf("Unpublish() error = %v", err)
		}

		posts, err := repos.post.ListByThreadID(ctx, thread.ID)
		if err != nil {
			t.Fatalf("ListByThreadID() error = %v", err)
		}
		wantPosts := []*model.Post{first, second, third}
		if len(posts) != len(wantPosts) {
			t.Fatalf("len(ListByThreadID()) = %d, want %d", len(posts), len(wantPosts))
		}
		for i, want := range wantPosts {
			got := posts[i]
			if got.ID != want.ID {
				t.Errorf("ListByThreadID()[%d].ID = %v, want %v", i, got.ID, want.ID)
			}
			if got.Number != want.Number {
				t.Errorf("ListByThreadID()[%d].Number = %d, want %d", i, got.Number, want.Number)
			}
			if got.Body != want.Body {
				t.Errorf("ListByThreadID()[%d].Body = %q, want %q", i, got.Body, want.Body)
			}
			if want.ID == second.ID {
				if got.UnpublishedAt == nil || got.UnpublishedAt.IsZero() {
					t.Errorf("ListByThreadID()[%d].UnpublishedAt = %v, want the stamped time", i, got.UnpublishedAt)
				}
			} else if got.UnpublishedAt != nil {
				t.Errorf("ListByThreadID()[%d].UnpublishedAt = %v, want nil", i, got.UnpublishedAt)
			}
		}
	})

	t.Run("投稿を持たないスレッドは空を返す", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newContentRepos(t)
		board := repos.createBoardWithCategory(t, ctx, "tech")
		thread := repos.createThread(t, ctx, board.ID, "SQLite の話")

		posts, err := repos.post.ListByThreadID(ctx, thread.ID)
		if err != nil {
			t.Fatalf("ListByThreadID() error = %v", err)
		}
		if len(posts) != 0 {
			t.Errorf("len(ListByThreadID()) = %d, want 0", len(posts))
		}
	})
}

// createPostBy inserts a post written by the given user, failing the test on
// error. It sits beside createPost because a lookup by author is about the very
// thing that helper leaves unset.
//
// [Ja] createPostBy は指定したユーザーが書いた投稿を挿入し、エラー時はテストを失敗させる。
// createPost と並べて置くのは、作者で引き当てるテストが問うのが、そのヘルパーが設定しない
// ものそのものであるためである。
func (r *contentRepos) createPostBy(t *testing.T, ctx context.Context, threadID model.ThreadID, userID model.UserID, number int, body string) *model.Post {
	t.Helper()

	post, err := r.post.Create(ctx, repository.CreatePostInput{
		ThreadID: threadID,
		UserID:   &userID,
		Number:   number,
		Body:     body,
	})
	if err != nil {
		t.Fatalf("テスト用投稿の作成に失敗: %v", err)
	}

	return post
}

func TestPostRepository_FindLatestByUserID(t *testing.T) {
	t.Parallel()

	t.Run("その利用者が最後に書いた投稿を掲示板やスレッドをまたいで返す", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newContentRepos(t)
		tech := repos.createBoardWithCategory(t, ctx, "tech")
		chat := repos.createBoardWithCategory(t, ctx, "chat")
		techThread := repos.createThread(t, ctx, tech.ID, "SQLite の話")
		chatThread := repos.createThread(t, ctx, chat.ID, "雑談")
		author := testutil.NewUserBuilder(t, repos.db).Build()
		other := testutil.NewUserBuilder(t, repos.db).Build()

		base := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
		earlier := repos.createPostBy(t, ctx, techThread.ID, author, 1, "先に書いた投稿")
		latest := repos.createPostBy(t, ctx, chatThread.ID, author, 1, "別の掲示板に後から書いた投稿")
		// Another person's post is the newest row in the table, so a lookup that
		// ignored the author would return it instead.
		//
		// [Ja] 他人の投稿をテーブルの中で最も新しい行にしてあり、作者を見ない引き当てなら
		// こちらを返すことになる。
		newest := repos.createPostBy(t, ctx, techThread.ID, other, 2, "他の人の投稿")
		testutil.BackdatePost(t, repos.db, earlier.ID, base)
		testutil.BackdatePost(t, repos.db, latest.ID, base.Add(time.Minute))
		testutil.BackdatePost(t, repos.db, newest.ID, base.Add(time.Hour))

		got, err := repos.post.FindLatestByUserID(ctx, author)
		if err != nil {
			t.Fatalf("FindLatestByUserID() error = %v", err)
		}
		if got == nil {
			t.Fatal("FindLatestByUserID() = nil, want the author's latest post")
		}
		if got.ID != latest.ID {
			t.Errorf("FindLatestByUserID().ID = %v, want %v", got.ID, latest.ID)
		}
	})

	t.Run("同じ時刻の投稿は後から書かれたものを返す", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newContentRepos(t)
		board := repos.createBoardWithCategory(t, ctx, "tech")
		thread := repos.createThread(t, ctx, board.ID, "SQLite の話")
		author := testutil.NewUserBuilder(t, repos.db).Build()

		sameMoment := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
		earlier := repos.createPostBy(t, ctx, thread.ID, author, 1, "先に書いた投稿")
		later := repos.createPostBy(t, ctx, thread.ID, author, 2, "同じ時刻に後から書いた投稿")
		testutil.BackdatePost(t, repos.db, earlier.ID, sameMoment)
		testutil.BackdatePost(t, repos.db, later.ID, sameMoment)

		got, err := repos.post.FindLatestByUserID(ctx, author)
		if err != nil {
			t.Fatalf("FindLatestByUserID() error = %v", err)
		}
		if got == nil {
			t.Fatal("FindLatestByUserID() = nil, want the later of the two posts")
		}
		if got.ID != later.ID {
			t.Errorf("FindLatestByUserID().ID = %v, want %v", got.ID, later.ID)
		}
	})

	t.Run("まだ投稿していない利用者には nil を返す", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newContentRepos(t)
		board := repos.createBoardWithCategory(t, ctx, "tech")
		thread := repos.createThread(t, ctx, board.ID, "SQLite の話")
		author := testutil.NewUserBuilder(t, repos.db).Build()
		newcomer := testutil.NewUserBuilder(t, repos.db).Build()
		repos.createPostBy(t, ctx, thread.ID, author, 1, "他の人の投稿")

		got, err := repos.post.FindLatestByUserID(ctx, newcomer)
		if err != nil {
			t.Fatalf("FindLatestByUserID() error = %v", err)
		}
		if got != nil {
			t.Errorf("FindLatestByUserID() = %v, want nil", got.ID)
		}
	})
}

// TestPostRepository_FindLatestByUserID_ReadsOneRowThroughTheIndex verifies that
// the lookup the interval between a person's posts is measured with is answered
// by the index: the author's rows are found through it and already in the order
// asked for, so reading the latest one costs the same whether that person has
// written once or a thousand times.
//
// [Ja] TestPostRepository_FindLatestByUserID_ReadsOneRowThroughTheIndex は、人の投稿の
// 間隔を測る引き当てが索引で答えられることを検証します。その作者の行は索引を通して見つかり、
// しかも求めた順に並んでいるため、最新の 1 件を読む費用は、その人が 1 度書いたか 1000 度
// 書いたかによらず変わりません。
func TestPostRepository_FindLatestByUserID_ReadsOneRowThroughTheIndex(t *testing.T) {
	t.Parallel()

	repos, ctx := newContentRepos(t)

	plan := queryPlan(t, ctx, repos.db, queryStatement(t, "posts.sql", "GetLatestPostByUserID"), int64(1))

	const index = "index_posts_on_user_id_and_created_at_and_id"
	if !strings.Contains(plan, index) {
		t.Errorf("最新投稿のクエリの実行計画 = %q, %q をたどるはず", plan, index)
	}
	// SQLite names a sort it performs itself a temporary B-tree, which is what
	// appears once the index no longer carries the order the query asks for.
	//
	// [Ja] SQLite は自ら行う並べ替えを一時 B-tree と呼ぶ。これが現れるのは、索引が
	// クエリの求める順序を持たなくなったときである。
	if strings.Contains(plan, "TEMP B-TREE") {
		t.Errorf("最新投稿のクエリの実行計画 = %q, 並べ替えを伴わないはず", plan)
	}
}

// queryStatement returns the statement the named query runs, read out of the
// file sqlc compiles it from. It is read rather than written out again in a test
// so that the plan checked against it belongs to the query that actually runs; a
// copy would go on passing after the original changed.
//
// [Ja] queryStatement は、名前で指したクエリが実行する文を、sqlc がそれをコンパイルする
// 元のファイルから読み出します。テストの中に書き直さず読むのは、それに対して確かめる実行
// 計画を実際に走るクエリのものにするためです。書き写したものは、元が変わった後も通り
// 続けます。
func queryStatement(t *testing.T, file, name string) string {
	t.Helper()

	marker := "-- name: " + name

	raw, err := os.ReadFile(filepath.Join("..", "..", "db", "queries", file))
	if err != nil {
		t.Fatalf("クエリファイルの読み込みに失敗: %v", err)
	}

	_, after, found := strings.Cut(string(raw), marker)
	if !found {
		t.Fatalf("クエリファイルに %q が見つからない", marker)
	}
	// The name is followed by the result annotation (:one, :many, ...), which
	// is not part of the statement.
	//
	// [Ja] 名前の後ろには結果の注釈 (:one・:many など) が続く。これは文の一部では
	// ないため落とす。
	_, statement, _ := strings.Cut(after, "\n")
	statement, _, _ = strings.Cut(statement, "-- name:")

	return strings.TrimSpace(sqlcArg.ReplaceAllString(statement, "?"))
}

// sqlcArg matches the named-parameter form sqlc accepts, which the generator
// turns into a placeholder the driver understands. A statement read from the
// query file still carries the form as written, and SQLite cannot parse it, so
// the same substitution is made here.
//
// [Ja] sqlcArg は sqlc が受け付ける名前付きパラメータの記法に一致する。生成器はこれを
// ドライバの解する差し込み位置に変える。クエリファイルから読んだ文は書かれたままの記法を
// 持ち、SQLite はそれを解釈できないため、ここで同じ置き換えを行う。
var sqlcArg = regexp.MustCompile(`sqlc\.arg\([^)]*\)`)

// queryPlan returns how SQLite says it will answer the statement, as the lines
// of EXPLAIN QUERY PLAN joined into one.
//
// [Ja] queryPlan は、SQLite がその文にどう答えるつもりかを、EXPLAIN QUERY PLAN の各行を
// 1 つに繋いだ形で返します。
func queryPlan(t *testing.T, ctx context.Context, db *database.DB, statement string, args ...any) string {
	t.Helper()

	rows, err := db.Reader.QueryContext(ctx, "EXPLAIN QUERY PLAN "+statement, args...)
	if err != nil {
		t.Fatalf("実行計画の取得に失敗: %v", err)
	}
	defer func() { _ = rows.Close() }()

	var details []string
	for rows.Next() {
		var id, parent, notUsed int64
		var detail string
		if err := rows.Scan(&id, &parent, &notUsed, &detail); err != nil {
			t.Fatalf("実行計画の読み取りに失敗: %v", err)
		}
		details = append(details, detail)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("実行計画の読み取りに失敗: %v", err)
	}

	return strings.Join(details, "\n")
}

func TestPostRepository_FindByThreadIDAndNumber(t *testing.T) {
	t.Parallel()

	t.Run("スレッドとレス番号で投稿を取得できる", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newContentRepos(t)
		board := repos.createBoardWithCategory(t, ctx, "tech")
		thread := repos.createThread(t, ctx, board.ID, "SQLite の話")
		first := repos.createPost(t, ctx, thread.ID, 1, "1 つ目の投稿")

		post, err := repos.post.FindByThreadIDAndNumber(ctx, thread.ID, 1)
		if err != nil {
			t.Fatalf("FindByThreadIDAndNumber() error = %v", err)
		}
		if post == nil {
			t.Fatal("FindByThreadIDAndNumber() = nil, want post")
		}
		if post.ID != first.ID {
			t.Errorf("post.ID = %v, want %v", post.ID, first.ID)
		}
		if post.Body != first.Body {
			t.Errorf("post.Body = %q, want %q", post.Body, first.Body)
		}
	})

	t.Run("同じ番号でも別のスレッドの投稿は返さない", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newContentRepos(t)
		board := repos.createBoardWithCategory(t, ctx, "tech")
		thread := repos.createThread(t, ctx, board.ID, "SQLite の話")
		other := repos.createThread(t, ctx, board.ID, "別のスレッド")
		first := repos.createPost(t, ctx, thread.ID, 1, "1 つ目の投稿")
		otherFirst := repos.createPost(t, ctx, other.ID, 1, "別のスレッドの 1 つ目")

		post, err := repos.post.FindByThreadIDAndNumber(ctx, other.ID, 1)
		if err != nil {
			t.Fatalf("FindByThreadIDAndNumber() error = %v", err)
		}
		if post == nil {
			t.Fatal("FindByThreadIDAndNumber() = nil, want post")
		}
		if post.ID != otherFirst.ID {
			t.Errorf("post.ID = %v, want %v (not %v)", post.ID, otherFirst.ID, first.ID)
		}
	})

	t.Run("そのスレッドに無い番号は (nil, nil) を返す", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newContentRepos(t)
		board := repos.createBoardWithCategory(t, ctx, "tech")
		thread := repos.createThread(t, ctx, board.ID, "SQLite の話")
		repos.createPost(t, ctx, thread.ID, 1, "1 つ目の投稿")

		post, err := repos.post.FindByThreadIDAndNumber(ctx, thread.ID, 3)
		if err != nil {
			t.Fatalf("FindByThreadIDAndNumber() error = %v, want nil", err)
		}
		if post != nil {
			t.Errorf("FindByThreadIDAndNumber() = %v, want nil", post)
		}
	})

	t.Run("非公開の投稿も返す", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newContentRepos(t)
		board := repos.createBoardWithCategory(t, ctx, "tech")
		thread := repos.createThread(t, ctx, board.ID, "SQLite の話")
		repos.createPost(t, ctx, thread.ID, 1, "1 つ目の投稿")
		second := repos.createPost(t, ctx, thread.ID, 2, "2 つ目の投稿")

		if err := repos.post.Unpublish(ctx, second.ID); err != nil {
			t.Fatalf("Unpublish() error = %v", err)
		}

		post, err := repos.post.FindByThreadIDAndNumber(ctx, thread.ID, 2)
		if err != nil {
			t.Fatalf("FindByThreadIDAndNumber() error = %v", err)
		}
		if post == nil {
			t.Fatal("FindByThreadIDAndNumber() = nil, want post")
		}
		if post.UnpublishedAt == nil {
			t.Error("post.UnpublishedAt = nil, want the stamped time")
		}
		if post.Body != second.Body {
			t.Errorf("post.Body = %q, want %q", post.Body, second.Body)
		}
	})
}

func TestPostRepository_ListByIDs(t *testing.T) {
	t.Parallel()

	t.Run("id 順に返し、非公開の投稿も含む", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newContentRepos(t)
		board := repos.createBoardWithCategory(t, ctx, "tech")
		thread := repos.createThread(t, ctx, board.ID, "SQLite の話")
		first := repos.createPost(t, ctx, thread.ID, 1, "1 つ目の投稿")
		second := repos.createPost(t, ctx, thread.ID, 2, "非公開にされる投稿")

		if err := repos.post.Unpublish(ctx, second.ID); err != nil {
			t.Fatalf("Unpublish() error = %v", err)
		}

		posts, err := repos.post.ListByIDs(ctx, []model.PostID{second.ID, first.ID, second.ID})
		if err != nil {
			t.Fatalf("ListByIDs() error = %v", err)
		}
		if len(posts) != 2 {
			t.Fatalf("len(ListByIDs()) = %d, want 2", len(posts))
		}
		if posts[0].ID != first.ID {
			t.Errorf("ListByIDs()[0].ID = %v, want %v", posts[0].ID, first.ID)
		}
		if posts[1].ID != second.ID {
			t.Errorf("ListByIDs()[1].ID = %v, want %v", posts[1].ID, second.ID)
		}
		if posts[1].UnpublishedAt == nil {
			t.Error("ListByIDs()[1].UnpublishedAt = nil, want the stamped time")
		}
		if posts[1].Body != second.Body {
			t.Errorf("ListByIDs()[1].Body = %q, want %q", posts[1].Body, second.Body)
		}
	})

	t.Run("存在しない id は結果に現れない", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newContentRepos(t)
		board := repos.createBoardWithCategory(t, ctx, "tech")
		thread := repos.createThread(t, ctx, board.ID, "SQLite の話")
		created := repos.createPost(t, ctx, thread.ID, 1, "唯一の投稿")

		posts, err := repos.post.ListByIDs(ctx, []model.PostID{created.ID, created.ID + 100000})
		if err != nil {
			t.Fatalf("ListByIDs() error = %v", err)
		}
		if len(posts) != 1 {
			t.Fatalf("len(ListByIDs()) = %d, want 1", len(posts))
		}
		if posts[0].ID != created.ID {
			t.Errorf("ListByIDs()[0].ID = %v, want %v", posts[0].ID, created.ID)
		}
	})

	t.Run("空の id は空を返す", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newContentRepos(t)

		posts, err := repos.post.ListByIDs(ctx, nil)
		if err != nil {
			t.Fatalf("ListByIDs() error = %v", err)
		}
		if len(posts) != 0 {
			t.Errorf("len(ListByIDs()) = %d, want 0", len(posts))
		}
	})
}

// TestPostRepository_Unpublish_KeepsTheThreadsCountAndLatestPost verifies that
// unpublishing a post leaves the thread's denormalized view of its posts alone.
// The count is the number of reply numbers issued rather than the number of
// bodies on display, so a thread at the cap stays closed and the next post keeps
// the number it would have had.
//
// [Ja] TestPostRepository_Unpublish_KeepsTheThreadsCountAndLatestPostは、投稿を非公開に
// してもスレッドが持つ投稿の非正規化された姿がそのままであることを検証します。件数は表示
// されている本文の数ではなく発行したレス番号の数であるため、上限に達したスレッドは閉じた
// ままであり、次の投稿は本来の番号を保ちます。
func TestPostRepository_Unpublish_KeepsTheThreadsCountAndLatestPost(t *testing.T) {
	t.Parallel()

	repos, ctx := newContentRepos(t)
	board := repos.createBoardWithCategory(t, ctx, "tech")
	lastPostedAt := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
	thread := repos.createThreadPostedAt(t, ctx, board.ID, "1 件だけのスレッド", lastPostedAt)

	post, err := repos.post.FindByThreadIDAndNumber(ctx, thread.ID, 1)
	if err != nil {
		t.Fatalf("FindByThreadIDAndNumber() error = %v", err)
	}
	if post == nil {
		t.Fatal("FindByThreadIDAndNumber() = nil, want post")
	}

	if err := repos.post.Unpublish(ctx, post.ID); err != nil {
		t.Fatalf("Unpublish() error = %v", err)
	}

	found, err := repos.thread.FindByID(ctx, thread.ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}
	if found == nil {
		t.Fatal("FindByID() = nil, want thread")
	}
	if found.PostsCount != 1 {
		t.Errorf("thread.PostsCount = %d, want 1", found.PostsCount)
	}
	if found.LastPostID == nil {
		t.Fatal("thread.LastPostID = nil, want the post's id")
	}
	if *found.LastPostID != post.ID {
		t.Errorf("thread.LastPostID = %v, want %v", *found.LastPostID, post.ID)
	}
	if !found.LastPostedAt.Equal(lastPostedAt) {
		t.Errorf("thread.LastPostedAt = %v, want %v", found.LastPostedAt, lastPostedAt)
	}
}
