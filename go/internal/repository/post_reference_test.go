package repository_test

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// createPostReferenceはある投稿が別の投稿を参照していることを記録し、エラー時は
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
	thread := repos.createThread(t, ctx, board.ID, "SQLiteの話")
	first := repos.createPost(t, ctx, thread.ID, 1, "1つ目")
	second := repos.createPost(t, ctx, thread.ID, 2, ">>1")

	reference := repos.createPostReference(t, ctx, second.ID, first.ID)

	if reference.ID == 0 {
		t.Error("Create() reference.IDはDB採番で空でないはず")
	}
	if reference.PostID != second.ID {
		t.Errorf("reference.PostID = %v、期待値 = %v", reference.PostID, second.ID)
	}
	if reference.ReferencedPostID != first.ID {
		t.Errorf("reference.ReferencedPostID = %v、期待値 = %v", reference.ReferencedPostID, first.ID)
	}
	if reference.CreatedAt.IsZero() {
		t.Error("reference.CreatedAtはDB既定値で設定されるはず")
	}
	if reference.UpdatedAt.IsZero() {
		t.Error("reference.UpdatedAtはDB既定値で設定されるはず")
	}
}

func TestPostReferenceRepository_Create_RejectsTheSameReferenceTwice(t *testing.T) {
	t.Parallel()

	repos, ctx := newContentRepos(t)
	board := repos.createBoardWithCategory(t, ctx, "tech")
	thread := repos.createThread(t, ctx, board.ID, "SQLiteの話")
	first := repos.createPost(t, ctx, thread.ID, 1, "1つ目")
	second := repos.createPost(t, ctx, thread.ID, 2, ">>1と >>1")

	repos.createPostReference(t, ctx, second.ID, first.ID)

	_, err := repos.postReference.Create(ctx, repository.CreatePostReferenceInput{
		PostID:           second.ID,
		ReferencedPostID: first.ID,
	})
	if err == nil {
		t.Fatal("Create()のエラー = nil、期待値は一意制約違反")
	}
	if !repository.IsUniqueViolation(err) {
		t.Errorf("Create()のエラー = %v、期待値は一意制約違反", err)
	}
}

func TestPostReferenceRepository_ListByReferencedPostIDs(t *testing.T) {
	t.Parallel()

	t.Run("渡した投稿を指す参照だけを指し先ごと・参照した順に返す", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newContentRepos(t)
		board := repos.createBoardWithCategory(t, ctx, "tech")
		thread := repos.createThread(t, ctx, board.ID, "SQLiteの話")
		first := repos.createPost(t, ctx, thread.ID, 1, "1つ目")
		second := repos.createPost(t, ctx, thread.ID, 2, ">>1")
		third := repos.createPost(t, ctx, thread.ID, 3, ">>1 >>2")
		fourth := repos.createPost(t, ctx, thread.ID, 4, ">>3")

		// 挿入順は期待する並びともその逆とも異なるようにしてあり、挿入順をそのまま
		// 返すだけの結果では通らないようにしている。
		repos.createPostReference(t, ctx, third.ID, second.ID)
		repos.createPostReference(t, ctx, fourth.ID, third.ID)
		repos.createPostReference(t, ctx, second.ID, first.ID)
		repos.createPostReference(t, ctx, third.ID, first.ID)

		references, err := repos.postReference.ListByReferencedPostIDs(ctx, []model.PostID{first.ID, second.ID})
		if err != nil {
			t.Fatalf("ListByReferencedPostIDs()のエラー = %v", err)
		}

		want := []model.PostReference{
			{ReferencedPostID: first.ID, PostID: second.ID},
			{ReferencedPostID: first.ID, PostID: third.ID},
			{ReferencedPostID: second.ID, PostID: third.ID},
		}
		if len(references) != len(want) {
			t.Fatalf("len(ListByReferencedPostIDs()) = %d、期待値 = %d", len(references), len(want))
		}
		for i, w := range want {
			if references[i].ReferencedPostID != w.ReferencedPostID || references[i].PostID != w.PostID {
				t.Errorf("ListByReferencedPostIDs()[%d] = (投稿 %v -> %v)、期待値 = (投稿 %v -> %v)",
					i, references[i].PostID, references[i].ReferencedPostID, w.PostID, w.ReferencedPostID)
			}
		}
	})

	t.Run("idを1つも渡さなければクエリせず空を返す", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newContentRepos(t)
		board := repos.createBoardWithCategory(t, ctx, "tech")
		thread := repos.createThread(t, ctx, board.ID, "SQLiteの話")
		first := repos.createPost(t, ctx, thread.ID, 1, "1つ目")
		second := repos.createPost(t, ctx, thread.ID, 2, ">>1")
		repos.createPostReference(t, ctx, second.ID, first.ID)

		references, err := repos.postReference.ListByReferencedPostIDs(ctx, nil)
		if err != nil {
			t.Fatalf("ListByReferencedPostIDs()のエラー = %v", err)
		}
		if len(references) != 0 {
			t.Errorf("len(ListByReferencedPostIDs()) = %d、期待値 = 0", len(references))
		}
	})

	t.Run("返信されていない投稿は空を返す", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newContentRepos(t)
		board := repos.createBoardWithCategory(t, ctx, "tech")
		thread := repos.createThread(t, ctx, board.ID, "SQLiteの話")
		only := repos.createPost(t, ctx, thread.ID, 1, "1つ目")

		references, err := repos.postReference.ListByReferencedPostIDs(ctx, []model.PostID{only.ID})
		if err != nil {
			t.Fatalf("ListByReferencedPostIDs()のエラー = %v", err)
		}
		if len(references) != 0 {
			t.Errorf("len(ListByReferencedPostIDs()) = %d、期待値 = 0", len(references))
		}
	})
}

// listReferencesToは、いずれかの投稿を指す参照を返す。書き込みが記録したものすべてに
// ついて検証し、見つかると期待した行だけを見るのではないテストのためのものである。
func (r *contentRepos) listReferencesTo(t *testing.T, ctx context.Context, postIDs ...model.PostID) []*model.PostReference {
	t.Helper()

	references, err := r.postReference.ListByReferencedPostIDs(ctx, postIDs)
	if err != nil {
		t.Fatalf("テスト用のレス参照の取得に失敗: %v", err)
	}

	return references
}

func TestPostReferenceRepository_CreateAllByReferencedNumbers(t *testing.T) {
	t.Parallel()

	t.Run("同じスレッドの自分より小さい番号だけを記録する", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newContentRepos(t)
		board := repos.createBoardWithCategory(t, ctx, "tech")
		thread := repos.createThread(t, ctx, board.ID, "SQLiteの話")
		other := repos.createThread(t, ctx, board.ID, "別のスレッド")
		first := repos.createPost(t, ctx, thread.ID, 1, "1つ目")
		second := repos.createPost(t, ctx, thread.ID, 2, "2つ目")
		third := repos.createPost(t, ctx, thread.ID, 3, "3つ目")
		otherFirst := repos.createPost(t, ctx, other.ID, 1, "別のスレッドの1つ目")
		otherSecond := repos.createPost(t, ctx, other.ID, 2, "別のスレッドの2つ目")

		// 本文は、届く2つの番号のほかに、自身の番号・どの投稿も持たない番号・
		// 自身より先の番号を名指しており、別のスレッドも1と2を持っている。
		fourth := repos.createPost(t, ctx, thread.ID, 4, ">>1 >>2 >>4 >>5 >>99")

		err := repos.postReference.CreateAllByReferencedNumbers(ctx, repository.CreatePostReferencesInput{
			PostID:            fourth.ID,
			ThreadID:          thread.ID,
			Number:            fourth.Number,
			ReferencedNumbers: []int{1, 2, 4, 5, 99},
		})
		if err != nil {
			t.Fatalf("CreateAllByReferencedNumbers()のエラー = %v", err)
		}

		references := repos.listReferencesTo(t, ctx, first.ID, second.ID, third.ID, fourth.ID, otherFirst.ID, otherSecond.ID)
		want := []model.PostReference{
			{ReferencedPostID: first.ID, PostID: fourth.ID},
			{ReferencedPostID: second.ID, PostID: fourth.ID},
		}
		if len(references) != len(want) {
			t.Fatalf("記録された参照の数 = %d、期待値 = %d", len(references), len(want))
		}
		for i, w := range want {
			if references[i].ReferencedPostID != w.ReferencedPostID || references[i].PostID != w.PostID {
				t.Errorf("記録された参照[%d] = (投稿 %v -> %v)、期待値 = (投稿 %v -> %v)",
					i, references[i].PostID, references[i].ReferencedPostID, w.PostID, w.ReferencedPostID)
			}
		}
	})

	t.Run("同じ番号を2度渡しても関係を1つだけ記録する", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newContentRepos(t)
		board := repos.createBoardWithCategory(t, ctx, "tech")
		thread := repos.createThread(t, ctx, board.ID, "SQLiteの話")
		first := repos.createPost(t, ctx, thread.ID, 1, "1つ目")
		second := repos.createPost(t, ctx, thread.ID, 2, ">>1と >>1")

		err := repos.postReference.CreateAllByReferencedNumbers(ctx, repository.CreatePostReferencesInput{
			PostID:            second.ID,
			ThreadID:          thread.ID,
			Number:            second.Number,
			ReferencedNumbers: []int{1, 1},
		})
		if err != nil {
			t.Fatalf("CreateAllByReferencedNumbers()のエラー = %v", err)
		}

		references := repos.listReferencesTo(t, ctx, first.ID, second.ID)
		if len(references) != 1 {
			t.Fatalf("記録された参照の数 = %d、期待値 = 1", len(references))
		}
		if references[0].PostID != second.ID || references[0].ReferencedPostID != first.ID {
			t.Errorf("記録された参照 = (投稿 %v -> %v)、期待値 = (投稿 %v -> %v)",
				references[0].PostID, references[0].ReferencedPostID, second.ID, first.ID)
		}
	})

	t.Run("番号を1つも渡さなければ何も記録しない", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newContentRepos(t)
		board := repos.createBoardWithCategory(t, ctx, "tech")
		thread := repos.createThread(t, ctx, board.ID, "SQLiteの話")
		first := repos.createPost(t, ctx, thread.ID, 1, "1つ目")
		second := repos.createPost(t, ctx, thread.ID, 2, "参照を書いていない本文")

		err := repos.postReference.CreateAllByReferencedNumbers(ctx, repository.CreatePostReferencesInput{
			PostID:            second.ID,
			ThreadID:          thread.ID,
			Number:            second.Number,
			ReferencedNumbers: nil,
		})
		if err != nil {
			t.Fatalf("CreateAllByReferencedNumbers()のエラー = %v", err)
		}

		references := repos.listReferencesTo(t, ctx, first.ID, second.ID)
		if len(references) != 0 {
			t.Errorf("記録された参照の数 = %d、期待値 = 0", len(references))
		}
	})
}
