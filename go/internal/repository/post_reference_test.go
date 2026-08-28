package repository_test

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// createPostReference records that one post refers to another, failing the test
// on error.
//
// [Ja] createPostReference はある投稿が別の投稿を参照していることを記録し、エラー時は
// テストを失敗させる。
func (r *contentRepos) createPostReference(t *testing.T, ctx context.Context, postID, referencedPostID model.PostID) *model.PostReference {
	t.Helper()

	reference, err := r.postReference.Create(ctx, repository.CreatePostReferenceInput{
		PostID:           postID,
		ReferencedPostID: referencedPostID,
	})
	if err != nil {
		t.Fatalf("テスト用レス参照の作成に失敗: %v", err)
	}

	return reference
}

func TestPostReferenceRepository_Create(t *testing.T) {
	t.Parallel()

	repos, ctx := newContentRepos(t)
	board := repos.createBoardWithCategory(t, ctx, "tech")
	thread := repos.createThread(t, ctx, board.ID, "SQLite の話")
	first := repos.createPost(t, ctx, thread.ID, 1, "1 つ目")
	second := repos.createPost(t, ctx, thread.ID, 2, ">>1")

	reference := repos.createPostReference(t, ctx, second.ID, first.ID)

	if reference.ID == 0 {
		t.Error("Create() reference.ID は DB 採番で空でないはず")
	}
	if reference.PostID != second.ID {
		t.Errorf("reference.PostID = %v, want %v", reference.PostID, second.ID)
	}
	if reference.ReferencedPostID != first.ID {
		t.Errorf("reference.ReferencedPostID = %v, want %v", reference.ReferencedPostID, first.ID)
	}
	if reference.CreatedAt.IsZero() {
		t.Error("reference.CreatedAt は DB 既定値で設定されるはず")
	}
	if reference.UpdatedAt.IsZero() {
		t.Error("reference.UpdatedAt は DB 既定値で設定されるはず")
	}
}

func TestPostReferenceRepository_Create_RejectsTheSameReferenceTwice(t *testing.T) {
	t.Parallel()

	repos, ctx := newContentRepos(t)
	board := repos.createBoardWithCategory(t, ctx, "tech")
	thread := repos.createThread(t, ctx, board.ID, "SQLite の話")
	first := repos.createPost(t, ctx, thread.ID, 1, "1 つ目")
	second := repos.createPost(t, ctx, thread.ID, 2, ">>1 と >>1")

	repos.createPostReference(t, ctx, second.ID, first.ID)

	_, err := repos.postReference.Create(ctx, repository.CreatePostReferenceInput{
		PostID:           second.ID,
		ReferencedPostID: first.ID,
	})
	if err == nil {
		t.Fatal("Create() error = nil, want a unique violation")
	}
	if !repository.IsUniqueViolation(err) {
		t.Errorf("Create() error = %v, want a unique violation", err)
	}
}

func TestPostReferenceRepository_ListByReferencedPostIDs(t *testing.T) {
	t.Parallel()

	t.Run("渡した投稿を指す参照だけを指し先ごと・参照した順に返す", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newContentRepos(t)
		board := repos.createBoardWithCategory(t, ctx, "tech")
		thread := repos.createThread(t, ctx, board.ID, "SQLite の話")
		first := repos.createPost(t, ctx, thread.ID, 1, "1 つ目")
		second := repos.createPost(t, ctx, thread.ID, 2, ">>1")
		third := repos.createPost(t, ctx, thread.ID, 3, ">>1 >>2")
		fourth := repos.createPost(t, ctx, thread.ID, 4, ">>3")

		// The insertion order is deliberately neither the expected order nor its
		// reverse, so a result that merely echoes it cannot pass.
		//
		// [Ja] 挿入順は期待する並びともその逆とも異なるようにしてあり、挿入順をそのまま
		// 返すだけの結果では通らないようにしている。
		repos.createPostReference(t, ctx, third.ID, second.ID)
		repos.createPostReference(t, ctx, fourth.ID, third.ID)
		repos.createPostReference(t, ctx, second.ID, first.ID)
		repos.createPostReference(t, ctx, third.ID, first.ID)

		references, err := repos.postReference.ListByReferencedPostIDs(ctx, []model.PostID{first.ID, second.ID})
		if err != nil {
			t.Fatalf("ListByReferencedPostIDs() error = %v", err)
		}

		want := []model.PostReference{
			{ReferencedPostID: first.ID, PostID: second.ID},
			{ReferencedPostID: first.ID, PostID: third.ID},
			{ReferencedPostID: second.ID, PostID: third.ID},
		}
		if len(references) != len(want) {
			t.Fatalf("len(ListByReferencedPostIDs()) = %d, want %d", len(references), len(want))
		}
		for i, w := range want {
			if references[i].ReferencedPostID != w.ReferencedPostID || references[i].PostID != w.PostID {
				t.Errorf("ListByReferencedPostIDs()[%d] = (post %v -> %v), want (post %v -> %v)",
					i, references[i].PostID, references[i].ReferencedPostID, w.PostID, w.ReferencedPostID)
			}
		}
	})

	t.Run("id を 1 つも渡さなければクエリせず空を返す", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newContentRepos(t)
		board := repos.createBoardWithCategory(t, ctx, "tech")
		thread := repos.createThread(t, ctx, board.ID, "SQLite の話")
		first := repos.createPost(t, ctx, thread.ID, 1, "1 つ目")
		second := repos.createPost(t, ctx, thread.ID, 2, ">>1")
		repos.createPostReference(t, ctx, second.ID, first.ID)

		references, err := repos.postReference.ListByReferencedPostIDs(ctx, nil)
		if err != nil {
			t.Fatalf("ListByReferencedPostIDs() error = %v", err)
		}
		if len(references) != 0 {
			t.Errorf("len(ListByReferencedPostIDs()) = %d, want 0", len(references))
		}
	})

	t.Run("返信されていない投稿は空を返す", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newContentRepos(t)
		board := repos.createBoardWithCategory(t, ctx, "tech")
		thread := repos.createThread(t, ctx, board.ID, "SQLite の話")
		only := repos.createPost(t, ctx, thread.ID, 1, "1 つ目")

		references, err := repos.postReference.ListByReferencedPostIDs(ctx, []model.PostID{only.ID})
		if err != nil {
			t.Fatalf("ListByReferencedPostIDs() error = %v", err)
		}
		if len(references) != 0 {
			t.Errorf("len(ListByReferencedPostIDs()) = %d, want 0", len(references))
		}
	})
}
