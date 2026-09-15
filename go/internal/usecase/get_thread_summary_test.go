package usecase_test

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
)

// TestGetThreadSummaryUsecase_Executeは、スレッドが行の述べるとおりに読み戻され、
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
		t.Fatalf("Create()のエラー = %v", err)
	}
	thread, err := threadRepo.Create(ctx, repository.CreateThreadInput{
		BoardID:  jazz.ID,
		Title:    "枯葉の名演",
		Language: model.LocaleJa.ThreadLanguage(),
	})
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}
	post, err := postRepo.Create(ctx, repository.CreatePostInput{ThreadID: thread.ID, Number: 1, Body: "好きな演奏は?"})
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}
	if err := threadRepo.UpdateLastPost(ctx, thread.ID, repository.UpdateThreadLastPostInput{
		PostsCount:   1,
		LastPostID:   post.ID,
		LastPostedAt: post.CreatedAt,
	}); err != nil {
		t.Fatalf("UpdateLastPost()のエラー = %v", err)
	}

	uc := usecase.NewGetThreadSummaryUsecase(threadRepo)

	output, err := uc.Execute(ctx, usecase.GetThreadSummaryInput{ID: thread.ID})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	if output.Thread.ID != thread.ID {
		t.Errorf("読み戻したスレッド = %v、期待値 = %v", output.Thread.ID, thread.ID)
	}
	if output.Thread.Title != "枯葉の名演" {
		t.Errorf("読み戻したタイトル = %q、期待値 = %q", output.Thread.Title, "枯葉の名演")
	}
	if want := model.LocaleJa.ThreadLanguage(); output.Thread.Language != want {
		t.Errorf("読み戻した主言語 = %q、期待値 = %q", output.Thread.Language, want)
	}
	if output.Thread.PostsCount != 1 {
		t.Errorf("読み戻した投稿数 = %d、期待値 = 1", output.Thread.PostsCount)
	}
}

// TestGetThreadSummaryUsecase_Execute_NotFoundは、どのスレッドも名指さないidが、
// 呼び出し側が気付かなければならない空の結果ではなく、ハンドラーが404で応答する
// アプリケーションエラーとして返ることを検証します。
func TestGetThreadSummaryUsecase_Execute_NotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testutil.SetupDB(t)

	uc := usecase.NewGetThreadSummaryUsecase(repository.NewThreadRepository(db))

	output, err := uc.Execute(ctx, usecase.GetThreadSummaryInput{ID: model.ThreadID(999)})
	if output != nil {
		t.Errorf("出力 = %v、期待値 = nil", output)
	}

	ae := model.AsAppError(err)
	if ae == nil {
		t.Fatalf("Execute()のエラー = %v、期待値 = *model.AppError", err)
	}
	if ae.Code != model.AppErrCodeResourceNotFound {
		t.Errorf("エラーコード = %v、期待値 = %v", ae.Code, model.AppErrCodeResourceNotFound)
	}
}

// TestGetThreadSummaryUsecase_Execute_Unpublishedは、管理者が見えない場所へ移した
// スレッドが、GetThreadUsecaseが報告するのと同じく非公開のAppErrorとして返ることを検証
// します。この読み取りが配信するのは拒否された返信が戻ってくるページであり、何も示さなく
// なったスレッドは書き込む先のページではありません。
func TestGetThreadSummaryUsecase_Execute_Unpublished(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testutil.SetupDB(t)

	board, err := repository.NewBoardRepository(db).Create(ctx, repository.CreateBoardInput{Slug: "jazz", Name: "ジャズ"})
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}
	threadRepo := repository.NewThreadRepository(db)
	thread, err := threadRepo.Create(ctx, repository.CreateThreadInput{
		BoardID:  board.ID,
		Title:    "枯葉の名演",
		Language: model.LocaleJa.ThreadLanguage(),
	})
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}
	if err := threadRepo.Unpublish(ctx, thread.ID); err != nil {
		t.Fatalf("Unpublish()のエラー = %v", err)
	}

	output, err := usecase.NewGetThreadSummaryUsecase(threadRepo).Execute(ctx, usecase.GetThreadSummaryInput{ID: thread.ID})
	if output != nil {
		t.Errorf("出力 = %v、期待値 = nil", output)
	}
	assertAppErrCode(t, err, model.AppErrCodeResourceUnpublished)
}
