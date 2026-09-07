package usecase_test

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
)

// TestGetThreadSummaryUsecase_Execute verifies that the thread is read back as
// the row says it, and that the posts written in it are not: a page naming a
// thread without showing the conversation is what this read exists for.
//
// The count the thread carries comes back with it, since that is what the lock
// standing at the end of the page is derived from.
//
// [Ja] TestGetThreadSummaryUsecase_Execute は、スレッドが行の述べるとおりに読み戻され、
// そこに書かれた投稿は読み戻されないことを検証します。会話を見せずにスレッドを名指す
// ページのために、この読み取りはあります。
//
// スレッドが持つ件数も併せて返ります。ページの末尾に立つロックがそこから導かれるため
// です。
func TestGetThreadSummaryUsecase_Execute(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testutil.SetupDB(t)

	boardRepo := repository.NewBoardRepository(db)
	threadRepo := repository.NewThreadRepository(db)
	postRepo := repository.NewPostRepository(db)

	jazz, err := boardRepo.Create(ctx, repository.CreateBoardInput{Slug: "jazz", Name: "ジャズ"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	thread, err := threadRepo.Create(ctx, repository.CreateThreadInput{
		BoardID:  jazz.ID,
		Title:    "枯葉の名演",
		Language: model.LocaleJa.ThreadLanguage(),
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	post, err := postRepo.Create(ctx, repository.CreatePostInput{ThreadID: thread.ID, Number: 1, Body: "好きな演奏は?"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := threadRepo.UpdateLastPost(ctx, thread.ID, repository.UpdateThreadLastPostInput{
		PostsCount:   1,
		LastPostID:   post.ID,
		LastPostedAt: post.CreatedAt,
	}); err != nil {
		t.Fatalf("UpdateLastPost() error = %v", err)
	}

	uc := usecase.NewGetThreadSummaryUsecase(threadRepo)

	output, err := uc.Execute(ctx, usecase.GetThreadSummaryInput{ID: thread.ID})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if output.Thread.ID != thread.ID {
		t.Errorf("読み戻したスレッド = %v, want %v", output.Thread.ID, thread.ID)
	}
	if output.Thread.Title != "枯葉の名演" {
		t.Errorf("読み戻したタイトル = %q, want %q", output.Thread.Title, "枯葉の名演")
	}
	if want := model.LocaleJa.ThreadLanguage(); output.Thread.Language != want {
		t.Errorf("読み戻した主言語 = %q, want %q", output.Thread.Language, want)
	}
	if output.Thread.PostsCount != 1 {
		t.Errorf("読み戻した投稿数 = %d, want 1", output.Thread.PostsCount)
	}
}

// TestGetThreadSummaryUsecase_Execute_NotFound verifies that an id naming no
// thread comes back as the application error a handler answers 404 with, rather
// than as an empty result the caller would have to notice.
//
// [Ja] TestGetThreadSummaryUsecase_Execute_NotFound は、どのスレッドも名指さない id が、
// 呼び出し側が気付かなければならない空の結果ではなく、ハンドラーが 404 で応答する
// アプリケーションエラーとして返ることを検証します。
func TestGetThreadSummaryUsecase_Execute_NotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testutil.SetupDB(t)

	uc := usecase.NewGetThreadSummaryUsecase(repository.NewThreadRepository(db))

	output, err := uc.Execute(ctx, usecase.GetThreadSummaryInput{ID: model.ThreadID(999)})
	if output != nil {
		t.Errorf("出力 = %v, want nil", output)
	}

	ae := model.AsAppError(err)
	if ae == nil {
		t.Fatalf("Execute() error = %v, want *model.AppError", err)
	}
	if ae.Code != model.AppErrCodeResourceNotFound {
		t.Errorf("エラーコード = %v, want %v", ae.Code, model.AppErrCodeResourceNotFound)
	}
}
