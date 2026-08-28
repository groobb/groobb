package repository_test

import (
	"context"
	"testing"

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
