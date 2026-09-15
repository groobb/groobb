package usecase_test

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/sqlitetime"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/validator"
)

// createPostFixtureは、テスト対象のUseCaseと、返信を書く先のスレッド、その最初の
// 投稿、返信を書くアカウント、そして検証が保存された行を読み戻すためのリポジトリです。
type createPostFixture struct {
	uc            *usecase.CreatePostUsecase
	db            *database.DB
	board         *model.Board
	thread        *model.Thread
	firstPost     *model.Post
	starter       model.UserID
	author        model.UserID
	threadRepo    *repository.ThreadRepository
	postRepo      *repository.PostRepository
	referenceRepo *repository.PostReferenceRepository
}

// newCreatePostUsecaseは、掲示板が1つ、最初の投稿を持つスレッドが1つ、そして
// 返信するための2つ目のアカウントがあるデータベース上にUseCaseを構築します。
//
// 返信する側をスレッドを立てたアカウントと別にするのは、1人の投稿の間隔が、フィクスチャと
// テストがこれから行う返信の間に立たないようにするためです。
func newCreatePostUsecase(t *testing.T) (createPostFixture, context.Context) {
	t.Helper()

	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)
	return newCreatePostUsecaseOn(t, testutil.SetupDB(t)), ctx
}

// newCreatePostUsecaseOnは掲示板・スレッド・アカウントをdbに投入し、その上に
// UseCaseを構築します。データベースを開かずに受け取るのは、テストが自分で開いたプールを
// 渡せるようにするためです。
func newCreatePostUsecaseOn(t *testing.T, db *database.DB) createPostFixture {
	t.Helper()

	ctx := context.Background()
	categoryRepo := repository.NewCategoryRepository(db)
	boardRepo := repository.NewBoardRepository(db)

	category, err := categoryRepo.Create(ctx, repository.CreateCategoryInput{Slug: "music", Name: "音楽"})
	if err != nil {
		t.Fatalf("テスト用カテゴリーの作成に失敗: %v", err)
	}
	board, err := boardRepo.Create(ctx, repository.CreateBoardInput{CategoryID: &category.ID, Slug: "jazz", Name: "ジャズ"})
	if err != nil {
		t.Fatalf("テスト用掲示板の作成に失敗: %v", err)
	}

	starter := testutil.NewUserBuilder(t, db).Build()
	thread, firstPost := seedThread(t, db, board.ID, starter, "枯葉の名演")

	return createPostFixture{
		uc:            newCreatePostUsecaseOver(db),
		db:            db,
		board:         board,
		thread:        thread,
		firstPost:     firstPost,
		starter:       starter,
		author:        testutil.NewUserBuilder(t, db).Build(),
		threadRepo:    repository.NewThreadRepository(db),
		postRepo:      repository.NewPostRepository(db),
		referenceRepo: repository.NewPostReferenceRepository(db),
	}
}

// newCreatePostUsecaseOverはdbのプール上にUseCaseを構築します。同じデータベース
// ファイルへ自身のプール経由で届く2つ目のUseCaseを必要とするテストのためのものです。
func newCreatePostUsecaseOver(db *database.DB) *usecase.CreatePostUsecase {
	return usecase.NewCreatePostUsecase(
		db.Writer,
		validator.NewPostCreateValidator(),
		repository.NewThreadRepository(db),
		repository.NewPostRepository(db),
		repository.NewPostReferenceRepository(db),
		repository.NewUserRepository(db),
	)
}

// seedThreadは、スレッドとその最初の投稿、そしてスレッドが持つその投稿の姿を
// 書き込みます。これが、立てられた後のスレッドが存在する状態です。
func seedThread(t *testing.T, db *database.DB, boardID model.BoardID, author model.UserID, title string) (*model.Thread, *model.Post) {
	t.Helper()

	ctx := context.Background()
	threadRepo := repository.NewThreadRepository(db)
	postRepo := repository.NewPostRepository(db)

	thread, err := threadRepo.Create(ctx, repository.CreateThreadInput{
		BoardID:  boardID,
		UserID:   &author,
		Title:    title,
		Language: model.LocaleJa.ThreadLanguage(),
	})
	if err != nil {
		t.Fatalf("テスト用スレッドの作成に失敗: %v", err)
	}

	post, err := postRepo.Create(ctx, repository.CreatePostInput{
		ThreadID: thread.ID,
		UserID:   &author,
		Number:   1,
		Body:     "好きな演奏は?",
	})
	if err != nil {
		t.Fatalf("テスト用の最初の投稿の作成に失敗: %v", err)
	}

	if err := threadRepo.UpdateLastPost(ctx, thread.ID, repository.UpdateThreadLastPostInput{
		PostsCount:   1,
		LastPostID:   post.ID,
		LastPostedAt: post.CreatedAt,
	}); err != nil {
		t.Fatalf("テスト用スレッドの最終投稿の更新に失敗: %v", err)
	}

	return findThread(t, db, thread.ID), post
}

// growThreadはスレッドをcount件まで満たします。行とスレッドが持つその姿をUseCase
// ではなく直接書くのは、そうしなければ上限の近くで何が起きるかを問うテストが、そこへ至る
// までに1000回の書き込みトランザクションを費やし、そのたびに投稿者の間隔を待つことに
// なるためです。
//
// 投稿を1時間前の日時にするのは、これらが帰属するアカウントを、この用意によって自身の
// 間隔の内側に置いたままにしないためです。
func growThread(t *testing.T, db *database.DB, thread *model.Thread, author model.UserID, count int) *model.Thread {
	t.Helper()

	if count <= thread.PostsCount {
		t.Fatalf("growThread(count=%d) はスレッドの現在の件数 %d より大きい必要がある", count, thread.PostsCount)
	}

	ctx := context.Background()
	postedAt := sqlitetime.Time(time.Now().Add(-time.Hour))
	authorID := int64(author)

	if _, err := db.Writer.ExecContext(ctx, `
		WITH RECURSIVE numbers(number) AS (
			SELECT ?
			UNION ALL
			SELECT number + 1 FROM numbers WHERE number < ?
		)
		INSERT INTO posts (thread_id, user_id, number, body, created_at, updated_at)
		SELECT ?, ?, number, '演奏の話', ?, ? FROM numbers;
	`, thread.PostsCount+1, count, int64(thread.ID), authorID, postedAt, postedAt); err != nil {
		t.Fatalf("テスト用投稿の一括作成に失敗: %v", err)
	}

	if _, err := db.Writer.ExecContext(ctx, `
		UPDATE threads
		SET posts_count = ?,
		    last_post_id = (SELECT id FROM posts WHERE thread_id = ? ORDER BY number DESC LIMIT 1),
		    last_posted_at = ?
		WHERE id = ?;
	`, count, int64(thread.ID), postedAt, int64(thread.ID)); err != nil {
		t.Fatalf("テスト用スレッドの集計の更新に失敗: %v", err)
	}

	return findThread(t, db, thread.ID)
}

// findThreadはスレッドを読み戻します。どの状態で残されたかを問う検証のためのもの
// です。
func findThread(t *testing.T, db *database.DB, id model.ThreadID) *model.Thread {
	t.Helper()

	thread, err := repository.NewThreadRepository(db).FindByID(context.Background(), id)
	if err != nil {
		t.Fatalf("FindByID()のエラー = %v", err)
	}
	if thread == nil {
		t.Fatalf("スレッドをidで引けない: id=%v", id)
	}
	return thread
}

// countPostReferencesはデータベース全体が持つ参照の件数を読みます。拒否された返信が
// 特定の行を欠いていることではなく、1件も記録していないことをテストが述べられるように
// するためです。
func countPostReferences(t *testing.T, db *database.DB) int {
	t.Helper()

	var count int
	if err := db.Reader.QueryRowContext(context.Background(), "SELECT count(*) FROM post_references").Scan(&count); err != nil {
		t.Fatalf("投稿参照の件数の取得に失敗: %v", err)
	}
	return count
}

// replySubmissionは、これから送られる1つの返信です。どのUseCaseが送り、どの
// スレッドへ書かれ、どのアカウントに帰属するかを持ちます。
type replySubmission struct {
	uc       *usecase.CreatePostUsecase
	threadID model.ThreadID
	author   model.UserID
}

// submitAtOnceはすべての送信を同時に行い、それぞれが返したものを、渡された順で
// 返します。
//
// goroutineは送信の前に待ち合わせます。そうしなければ、1つが次の起動を待たずに完走して
// しまいうるためです。これらのテストが結果に問うことを決めるのは書き込みロックであり、
// ロックが試されるのは2つ以上の送信が同時にトランザクションを開いている間だけです。
func submitAtOnce(ctx context.Context, submissions []replySubmission) ([]*usecase.CreatePostOutput, []error) {
	outputs := make([]*usecase.CreatePostOutput, len(submissions))
	errs := make([]error, len(submissions))
	start := make(chan struct{})

	var wg sync.WaitGroup
	for i, submission := range submissions {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			outputs[i], errs[i] = submission.uc.Execute(ctx, usecase.CreatePostInput{
				ThreadID: submission.threadID,
				UserID:   submission.author,
				Body:     fmt.Sprintf("%v からの返信", submission.author),
			})
		}()
	}
	close(start)
	wg.Wait()

	return outputs, errs
}

// TestCreatePostUsecase_Execute_Successは、妥当な返信が次のレス番号を付けて作者に
// 帰属して保存され、その後スレッドが持つ投稿の姿がそれを表すことを検証します。
func TestCreatePostUsecase_Execute_Success(t *testing.T) {
	t.Parallel()

	f, ctx := newCreatePostUsecase(t)

	out, err := f.uc.Execute(ctx, usecase.CreatePostInput{
		ThreadID: f.thread.ID,
		UserID:   f.author,
		Body:     "  バラード集がいいです\r\n特に2曲目  ",
	})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	if out == nil {
		t.Fatal("Execute()のoutput = nil")
	}
	if out.Number != 2 {
		t.Errorf("out.Number = %d、期待値 = %d", out.Number, 2)
	}

	posts, err := f.postRepo.ListByThreadID(ctx, f.thread.ID)
	if err != nil {
		t.Fatalf("ListByThreadID()のエラー = %v", err)
	}
	if len(posts) != 2 {
		t.Fatalf("len(posts) = %d、期待値 = %d", len(posts), 2)
	}
	reply := posts[1]
	if reply.Number != 2 {
		t.Errorf("reply.Number = %d、期待値 = %d", reply.Number, 2)
	}
	// 本文はvalidatorが正規化した形で保存される。前後の空白は本文の読まれ方の一部と
	// して保たれ、改行はアプリケーションが保持する1つの形で保存される。
	if want := "  バラード集がいいです\n特に2曲目  "; reply.Body != want {
		t.Errorf("reply.Body = %q、期待値 = %q", reply.Body, want)
	}
	if reply.UserID == nil || *reply.UserID != f.author {
		t.Errorf("reply.UserID = %v、期待値 = %v", reply.UserID, f.author)
	}

	thread := findThread(t, f.db, f.thread.ID)
	if thread.PostsCount != 2 {
		t.Errorf("thread.PostsCount = %d、期待値 = %d", thread.PostsCount, 2)
	}
	if thread.LastPostID == nil || *thread.LastPostID != reply.ID {
		t.Errorf("thread.LastPostID = %v、期待値 = %v", thread.LastPostID, reply.ID)
	}
	if !thread.LastPostedAt.Equal(reply.CreatedAt) {
		t.Errorf("thread.LastPostedAt = %v、期待値 = %v", thread.LastPostedAt, reply.CreatedAt)
	}
}

// TestCreatePostUsecase_Execute_SavesReferencesは、本文が名指すレス番号のうち何が
// 参照になるかを検証します。下の投稿が持つ番号は参照になり、同じ番号を2度書いても記録
// されるのは1つの関係です。どの投稿も持たない番号・その返信自身の番号・それより先の番号は
// 何も記録しないため、投稿が見せる逆リンクは実在するものになります。
func TestCreatePostUsecase_Execute_SavesReferences(t *testing.T) {
	t.Parallel()

	f, ctx := newCreatePostUsecase(t)

	out, err := f.uc.Execute(ctx, usecase.CreatePostInput{
		ThreadID: f.thread.ID,
		UserID:   f.author,
		Body:     ">>1に同意。>>1の2曲目が好き (>>2は自分、>>99はまだ無い)",
	})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}

	references, err := f.referenceRepo.ListByReferencedPostIDs(ctx, []model.PostID{f.firstPost.ID})
	if err != nil {
		t.Fatalf("ListByReferencedPostIDs()のエラー = %v", err)
	}
	if len(references) != 1 {
		t.Fatalf("len(references) = %d、期待値 = %d", len(references), 1)
	}

	posts, err := f.postRepo.ListByThreadID(ctx, f.thread.ID)
	if err != nil {
		t.Fatalf("ListByThreadID()のエラー = %v", err)
	}
	reply := posts[out.Number-1]
	if references[0].PostID != reply.ID {
		t.Errorf("references[0].PostID = %v、期待値 = %v", references[0].PostID, reply.ID)
	}
	if references[0].ReferencedPostID != f.firstPost.ID {
		t.Errorf("references[0].ReferencedPostID = %v、期待値 = %v", references[0].ReferencedPostID, f.firstPost.ID)
	}

	if got := countPostReferences(t, f.db); got != 1 {
		t.Errorf("投稿参照の件数 = %d、期待値 = %d", got, 1)
	}
}

// TestCreatePostUsecase_Execute_InvalidInputは、直すところのある本文がフィールドを
// 名指す *model.ValidationErrorとして戻り、そのために何も書かれないことを検証します。
func TestCreatePostUsecase_Execute_InvalidInput(t *testing.T) {
	t.Parallel()

	f, ctx := newCreatePostUsecase(t)

	out, err := f.uc.Execute(ctx, usecase.CreatePostInput{
		ThreadID: f.thread.ID,
		UserID:   f.author,
		Body:     "   ",
	})
	if out != nil {
		t.Error("Execute()のoutputはnilのはず")
	}
	ve := model.AsValidationError(err)
	if ve == nil {
		t.Fatalf("Execute()のエラー = %v、期待値 = *model.ValidationError", err)
	}
	if !ve.HasFieldError("body") {
		t.Error("veにbodyのエラーが無い")
	}

	if got := countPosts(t, f.db); got != 1 {
		t.Errorf("投稿の件数 = %d、期待値 = %d", got, 1)
	}
	if thread := findThread(t, f.db, f.thread.ID); thread.PostsCount != 1 {
		t.Errorf("thread.PostsCount = %d、期待値 = %d", thread.PostsCount, 1)
	}
}

// TestCreatePostUsecase_Execute_UnknownThreadは、どのスレッドも指さないidが
// リソース未存在として報告されることを検証します。これにより、どこにも表示されない返信を
// 保存する代わりに、ハンドラーは404を返せます。
func TestCreatePostUsecase_Execute_UnknownThread(t *testing.T) {
	t.Parallel()

	f, ctx := newCreatePostUsecase(t)

	out, err := f.uc.Execute(ctx, usecase.CreatePostInput{
		ThreadID: f.thread.ID + 1000,
		UserID:   f.author,
		Body:     "こんにちは",
	})
	if out != nil {
		t.Error("Execute()のoutputはnilのはず")
	}
	ae := model.AsAppError(err)
	if ae == nil {
		t.Fatalf("Execute()のエラー = %v、期待値 = *model.AppError", err)
	}
	if ae.Code != model.AppErrCodeResourceNotFound {
		t.Errorf("ae.Code = %d、期待値 = %d", ae.Code, model.AppErrCodeResourceNotFound)
	}
	if got := countPosts(t, f.db); got != 1 {
		t.Errorf("投稿の件数 = %d、期待値 = %d", got, 1)
	}
}

// TestCreatePostUsecase_Execute_WithdrawnUserは、退会したアカウントが返信できない
// ことを検証します。そのセッションが、まだ存在していたときに発行されたものであってもです。
func TestCreatePostUsecase_Execute_WithdrawnUser(t *testing.T) {
	t.Parallel()

	f, ctx := newCreatePostUsecase(t)

	out, err := f.uc.Execute(ctx, usecase.CreatePostInput{
		ThreadID: f.thread.ID,
		UserID:   testutil.NewUserBuilder(t, f.db).WithDeletedAt(time.Now().Add(-24 * time.Hour)).Build(),
		Body:     "こんにちは",
	})
	if out != nil {
		t.Error("Execute()のoutputはnilのはず")
	}
	ae := model.AsAppError(err)
	if ae == nil {
		t.Fatalf("Execute()のエラー = %v、期待値 = *model.AppError", err)
	}
	if ae.Code != model.AppErrCodeForbidden {
		t.Errorf("ae.Code = %d、期待値 = %d", ae.Code, model.AppErrCodeForbidden)
	}
	if got := countPosts(t, f.db); got != 1 {
		t.Errorf("投稿の件数 = %d、期待値 = %d", got, 1)
	}
}

// TestCreatePostUsecase_Execute_WithdrawnUserOnLockedThreadは、退会したアカウントが、
// 書き込み先のスレッドがそれ以上投稿を受け付けない場合でも、スレッドではなく自身について
// 告げられることを検証します。どちらの拒否も待って解けるものではなく、アカウントを名指す
// ほうが、そのアカウントが次にどこへ書いても成立し続ける理由です。ロック中のスレッドが運ぶ
// 案内は次のスレッドを差し出しますが、このアカウントはそれを立てることもできません。
func TestCreatePostUsecase_Execute_WithdrawnUserOnLockedThread(t *testing.T) {
	t.Parallel()

	f, ctx := newCreatePostUsecase(t)
	growThread(t, f.db, f.thread, f.starter, model.ThreadPostLimit)

	out, err := f.uc.Execute(ctx, usecase.CreatePostInput{
		ThreadID: f.thread.ID,
		UserID:   testutil.NewUserBuilder(t, f.db).WithDeletedAt(time.Now().Add(-24 * time.Hour)).Build(),
		Body:     "こんにちは",
	})
	if out != nil {
		t.Error("Execute()のoutputはnilのはず")
	}
	ae := model.AsAppError(err)
	if ae == nil {
		t.Fatalf("Execute()のエラー = %v、期待値 = *model.AppError", err)
	}
	if ae.Code != model.AppErrCodeForbidden {
		t.Errorf("ae.Code = %d、期待値 = %d", ae.Code, model.AppErrCodeForbidden)
	}
	if len(ae.LockReasons) != 0 {
		t.Errorf("ae.LockReasons = %v、期待値 = 空", ae.LockReasons)
	}
}

// TestCreatePostUsecase_Execute_WithinPostIntervalは、1つ目のすぐ後に送った2つ目の
// 返信が、拒否の理由である待ち時間とともに拒否され、その拒否が何も書かないことを検証します。
func TestCreatePostUsecase_Execute_WithinPostInterval(t *testing.T) {
	t.Parallel()

	f, ctx := newCreatePostUsecase(t)
	input := usecase.CreatePostInput{ThreadID: f.thread.ID, UserID: f.author, Body: "こんにちは"}

	if _, err := f.uc.Execute(ctx, input); err != nil {
		t.Fatalf("1つ目のExecute()のエラー = %v", err)
	}

	out, err := f.uc.Execute(ctx, input)
	if out != nil {
		t.Error("Execute()のoutputはnilのはず")
	}
	ae := model.AsAppError(err)
	if ae == nil {
		t.Fatalf("Execute()のエラー = %v、期待値 = *model.AppError", err)
	}
	if ae.Code != model.AppErrCodeRateLimited {
		t.Errorf("ae.Code = %d、期待値 = %d", ae.Code, model.AppErrCodeRateLimited)
	}
	if ae.RetryAfter <= 0 || ae.RetryAfter > model.PostInterval {
		t.Errorf("ae.RetryAfter = %s、期待値 = (0, %s]", ae.RetryAfter, model.PostInterval)
	}

	if got := countPosts(t, f.db); got != 2 {
		t.Errorf("投稿の件数 = %d、期待値 = %d", got, 2)
	}
	if thread := findThread(t, f.db, f.thread.ID); thread.PostsCount != 2 {
		t.Errorf("thread.PostsCount = %d、期待値 = %d", thread.PostsCount, 2)
	}
}

// TestCreatePostUsecase_Execute_IntervalIncludesStartingAThreadは、スレッドを立てた
// ときの投稿が同じ間隔に数えられることを検証します。スレッドを立てたばかりの人は、返信する
// 前に待つことになります。ここでは自分が立てたのではないスレッドへの返信です。
func TestCreatePostUsecase_Execute_IntervalIncludesStartingAThread(t *testing.T) {
	t.Parallel()

	f, ctx := newCreatePostUsecase(t)

	createThreadUC := usecase.NewCreateThreadUsecase(
		f.db.Writer,
		validator.NewThreadCreateValidator(),
		repository.NewBoardRepository(f.db),
		repository.NewThreadRepository(f.db),
		repository.NewPostRepository(f.db),
		repository.NewUserRepository(f.db),
	)
	if _, err := createThreadUC.Execute(ctx, usecase.CreateThreadInput{
		BoardSlug: f.board.Slug,
		UserID:    f.author,
		Title:     "モーダルジャズの入口",
		Language:  string(model.LocaleJa.ThreadLanguage()),
		Body:      "何から聴きましたか?",
	}); err != nil {
		t.Fatalf("スレッド作成のExecute()のエラー = %v", err)
	}

	out, err := f.uc.Execute(ctx, usecase.CreatePostInput{
		ThreadID: f.thread.ID,
		UserID:   f.author,
		Body:     "こちらにも一言",
	})
	if out != nil {
		t.Error("Execute()のoutputはnilのはず")
	}
	ae := model.AsAppError(err)
	if ae == nil {
		t.Fatalf("Execute()のエラー = %v、期待値 = *model.AppError", err)
	}
	if ae.Code != model.AppErrCodeRateLimited {
		t.Errorf("ae.Code = %d、期待値 = %d", ae.Code, model.AppErrCodeRateLimited)
	}
	if thread := findThread(t, f.db, f.thread.ID); thread.PostsCount != 1 {
		t.Errorf("thread.PostsCount = %d、期待値 = %d", thread.PostsCount, 1)
	}
}

// TestCreatePostUsecase_Execute_LockedThreadは、持てる投稿をすべて持っているスレッド
// がそれ以上受け付けないことを検証します。返信は理由を値として伴って拒否され、スレッドの
// 件数・参照・投稿者の間隔をそのままにします。
//
// 間隔は、直後に別スレッドへの返信が通ることで確かめます。この拒否がしてはならないのは、
// 投稿者の次の待ち時間を始めることであるためです。何も保存されなかった送信が、次の送信の
// 間隔を空けるものになることはありません。
func TestCreatePostUsecase_Execute_LockedThread(t *testing.T) {
	t.Parallel()

	f, ctx := newCreatePostUsecase(t)
	growThread(t, f.db, f.thread, f.starter, model.ThreadPostLimit)

	out, err := f.uc.Execute(ctx, usecase.CreatePostInput{
		ThreadID: f.thread.ID,
		UserID:   f.author,
		Body:     ">>1まだ間に合いますか?",
	})
	if out != nil {
		t.Error("Execute()のoutputはnilのはず")
	}
	ae := model.AsAppError(err)
	if ae == nil {
		t.Fatalf("Execute()のエラー = %v、期待値 = *model.AppError", err)
	}
	if ae.Code != model.AppErrCodeThreadLocked {
		t.Errorf("ae.Code = %d、期待値 = %d", ae.Code, model.AppErrCodeThreadLocked)
	}
	// 理由はモデルが生んだ値のまま運ばれる。ハンドラーが何を見せるかを、メッセージから
	// 読み取り直さずに選べるようにするためである。
	want := []model.ThreadLockReason{model.ThreadLockReasonPostLimitReached}
	if !slices.Equal(ae.LockReasons, want) {
		t.Errorf("ae.LockReasons = %v、期待値 = %v", ae.LockReasons, want)
	}

	if got := countPosts(t, f.db); got != model.ThreadPostLimit {
		t.Errorf("投稿の件数 = %d、期待値 = %d", got, model.ThreadPostLimit)
	}
	if thread := findThread(t, f.db, f.thread.ID); thread.PostsCount != model.ThreadPostLimit {
		t.Errorf("thread.PostsCount = %d、期待値 = %d", thread.PostsCount, model.ThreadPostLimit)
	}
	if got := countPostReferences(t, f.db); got != 0 {
		t.Errorf("投稿参照の件数 = %d、期待値 = %d", got, 0)
	}

	other, _ := seedThread(t, f.db, f.board.ID, f.starter, "次のスレッド")
	if _, err := f.uc.Execute(ctx, usecase.CreatePostInput{
		ThreadID: other.ID,
		UserID:   f.author,
		Body:     "こちらに書きます",
	}); err != nil {
		t.Fatalf("ロックによる拒否の直後のExecute()のエラー = %v", err)
	}
}

// TestCreatePostUsecase_Execute_LocksTheThreadAtTheCapは、スレッドを満たす投稿が
// 保存されること、そしてその投稿がコミットされた時点からスレッドがロック中であることを
// 検証します。次の返信は拒否されます。その間に、スレッドを満杯と印付ける何かが走ることは
// ありません。
func TestCreatePostUsecase_Execute_LocksTheThreadAtTheCap(t *testing.T) {
	t.Parallel()

	f, ctx := newCreatePostUsecase(t)
	thread := growThread(t, f.db, f.thread, f.starter, model.ThreadPostLimit-1)
	if len(thread.LockReasons()) != 0 {
		t.Fatalf("上限の1件手前のスレッドのLockReasons() = %v、期待値 = 空", thread.LockReasons())
	}

	out, err := f.uc.Execute(ctx, usecase.CreatePostInput{
		ThreadID: f.thread.ID,
		UserID:   f.author,
		Body:     "最後の1件です",
	})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	if out == nil {
		t.Fatal("Execute()のoutput = nil")
	}
	if out.Number != model.ThreadPostLimit {
		t.Errorf("out.Number = %d、期待値 = %d", out.Number, model.ThreadPostLimit)
	}

	filled := findThread(t, f.db, f.thread.ID)
	if filled.PostsCount != model.ThreadPostLimit {
		t.Errorf("thread.PostsCount = %d、期待値 = %d", filled.PostsCount, model.ThreadPostLimit)
	}
	want := []model.ThreadLockReason{model.ThreadLockReasonPostLimitReached}
	if !slices.Equal(filled.LockReasons(), want) {
		t.Errorf("thread.LockReasons() = %v、期待値 = %v", filled.LockReasons(), want)
	}

	// 次に返信するのを別のアカウントにするのは、送信を拒否するものを、1人の投稿の
	// 間隔ではなくスレッドの状態にするためである。
	_, err = f.uc.Execute(ctx, usecase.CreatePostInput{
		ThreadID: f.thread.ID,
		UserID:   testutil.NewUserBuilder(t, f.db).Build(),
		Body:     "まだ書けますか?",
	})
	ae := model.AsAppError(err)
	if ae == nil || ae.Code != model.AppErrCodeThreadLocked {
		t.Fatalf("Execute()のエラー = %v、期待値 = *model.AppError (%d)", err, model.AppErrCodeThreadLocked)
	}
	if got := countPosts(t, f.db); got != model.ThreadPostLimit {
		t.Errorf("投稿の件数 = %d、期待値 = %d", got, model.ThreadPostLimit)
	}
}

// TestCreatePostUsecase_Execute_ConcurrentAtTheCapは、上限の1件手前のスレッドへ
// 2つのアカウントが同時に返信すると、1つの投稿と1つの拒否になることを検証します。件数は
// 書き込みロックの下で読むため、2つ目の送信が見るのは1つ目がコミットした投稿であって
// その前の状態ではなく、古い件数から番号を採った返信が上限を越えることもありません。
func TestCreatePostUsecase_Execute_ConcurrentAtTheCap(t *testing.T) {
	t.Parallel()

	f, ctx := newCreatePostUsecase(t)
	growThread(t, f.db, f.thread, f.starter, model.ThreadPostLimit-1)

	outputs, errs := submitAtOnce(ctx, []replySubmission{
		{uc: f.uc, threadID: f.thread.ID, author: f.author},
		{uc: f.uc, threadID: f.thread.ID, author: testutil.NewUserBuilder(t, f.db).Build()},
	})

	assertOneFilledTheCap(t, f, outputs, errs)
}

// TestCreatePostUsecase_Execute_ConcurrentAcrossConnectionPoolsは、同じデータベース
// ファイルに対して開かれた別々の接続プールから投稿しても、SQLiteの書き込みロックが
// 投稿数の上限を守ることを検証します。
//
// 1つのプールは書き込み用コネクションを1本に制限するため、そこを通る送信はSQLiteに
// 何かを尋ねる前に直列化されます。2つ目のプールを開くとそれが無くなり、結果はデータベース
// ファイル自身が持つ書き込みロックに委ねられます。
func TestCreatePostUsecase_Execute_ConcurrentAcrossConnectionPools(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)
	path := testutil.SetupDBPath(t)
	f := newCreatePostUsecaseOn(t, openDB(t, path))
	growThread(t, f.db, f.thread, f.starter, model.ThreadPostLimit-1)

	outputs, errs := submitAtOnce(ctx, []replySubmission{
		{uc: f.uc, threadID: f.thread.ID, author: f.author},
		{uc: newCreatePostUsecaseOver(openDB(t, path)), threadID: f.thread.ID, author: testutil.NewUserBuilder(t, f.db).Build()},
	})

	assertOneFilledTheCap(t, f, outputs, errs)
}

// TestCreatePostUsecase_Execute_ConcurrentByTheSameUserAcrossThreadsは、同じ
// アカウントの連投間隔がスレッドと接続プールをまたいでも守られることを検証します。
// 投稿先を変えても連投間隔は共有されるため、保存される返信は1件だけです。
func TestCreatePostUsecase_Execute_ConcurrentByTheSameUserAcrossThreads(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)
	path := testutil.SetupDBPath(t)
	f := newCreatePostUsecaseOn(t, openDB(t, path))
	other, _ := seedThread(t, f.db, f.board.ID, f.starter, "別のスレッド")
	before := countPosts(t, f.db)

	submissions := []replySubmission{
		{uc: f.uc, threadID: f.thread.ID, author: f.author},
		{uc: newCreatePostUsecaseOver(openDB(t, path)), threadID: other.ID, author: f.author},
	}
	outputs, errs := submitAtOnce(ctx, submissions)

	succeeded := 0
	for i, err := range errs {
		wantCount := 1
		if err == nil {
			succeeded++
			wantCount++
			if outputs[i] == nil {
				t.Fatalf("submissions[%d] のoutput = nil", i)
			}
			if outputs[i].Number != 2 {
				t.Errorf("submissions[%d] のNumber = %d、期待値 = 2", i, outputs[i].Number)
			}
		} else {
			if outputs[i] != nil {
				t.Errorf("submissions[%d] のoutputはnilのはず", i)
			}
			ae := model.AsAppError(err)
			if ae == nil || ae.Code != model.AppErrCodeRateLimited {
				t.Fatalf("submissions[%d] のエラー = %v、期待値 = *model.AppError (%d)", i, err, model.AppErrCodeRateLimited)
			}
			if ae.RetryAfter <= 0 || ae.RetryAfter > model.PostInterval {
				t.Errorf("submissions[%d] のRetryAfter = %s、期待値 = (0, %s]", i, ae.RetryAfter, model.PostInterval)
			}
		}

		thread := findThread(t, f.db, submissions[i].threadID)
		if thread.PostsCount != wantCount {
			t.Errorf("submissions[%d] のthread.PostsCount = %d、期待値 = %d", i, thread.PostsCount, wantCount)
		}
	}
	if succeeded != 1 {
		t.Errorf("成功した送信 = %d件、期待値 = 1件", succeeded)
	}
	if got := countPosts(t, f.db); got != before+1 {
		t.Errorf("投稿の件数 = %d、期待値 = %d", got, before+1)
	}
}

// openDBはpathのデータベースファイルに対してもう1つプールを開き、テストの終了時に
// クローズします。
func openDB(t *testing.T, path string) *database.DB {
	t.Helper()

	db, err := database.Open(context.Background(), path)
	if err != nil {
		t.Fatalf("テスト用データベースのオープンに失敗: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("テスト用データベースのクローズに失敗: %v", err)
		}
	})
	return db
}

// assertOneFilledTheCapは、スレッドの最後の1枠を奪い合う2つの送信が至るべき結果を
// 述べます。一方が上限そのものの番号でそこを取り、他方はスレッドがロック中であると告げられ、
// スレッドはちょうど上限の件数を持って終わります。
func assertOneFilledTheCap(t *testing.T, f createPostFixture, outputs []*usecase.CreatePostOutput, errs []error) {
	t.Helper()

	succeeded := 0
	for i, err := range errs {
		if err == nil {
			succeeded++
			if outputs[i] == nil {
				t.Fatalf("submissions[%d] のoutput = nil", i)
			}
			if outputs[i].Number != model.ThreadPostLimit {
				t.Errorf("submissions[%d] のNumber = %d、期待値 = %d", i, outputs[i].Number, model.ThreadPostLimit)
			}
			continue
		}
		ae := model.AsAppError(err)
		if ae == nil || ae.Code != model.AppErrCodeThreadLocked {
			t.Errorf("submissions[%d] のエラー = %v、期待値 = *model.AppError (%d)", i, err, model.AppErrCodeThreadLocked)
		}
	}
	if succeeded != 1 {
		t.Errorf("成功した送信 = %d 件、期待値 = %d 件", succeeded, 1)
	}

	if got := countPosts(t, f.db); got != model.ThreadPostLimit {
		t.Errorf("投稿の件数 = %d、期待値 = %d", got, model.ThreadPostLimit)
	}
	if thread := findThread(t, f.db, f.thread.ID); thread.PostsCount != model.ThreadPostLimit {
		t.Errorf("thread.PostsCount = %d、期待値 = %d", thread.PostsCount, model.ThreadPostLimit)
	}
}

// TestCreatePostUsecase_Execute_ConcurrentByDifferentUsersは、別々のアカウントから
// 同時に送られた返信が、それぞれ自分のレス番号を与えられてすべて残ること、そしてスレッドが
// その最後の1件を表して終わることを検証します。
func TestCreatePostUsecase_Execute_ConcurrentByDifferentUsers(t *testing.T) {
	t.Parallel()

	f, ctx := newCreatePostUsecase(t)

	submissions := []replySubmission{{uc: f.uc, threadID: f.thread.ID, author: f.author}}
	for range 2 {
		submissions = append(submissions, replySubmission{uc: f.uc, threadID: f.thread.ID, author: testutil.NewUserBuilder(t, f.db).Build()})
	}

	outputs, errs := submitAtOnce(ctx, submissions)
	for i, err := range errs {
		if err != nil {
			t.Fatalf("submissions[%d] のExecute()のエラー = %v", i, err)
		}
	}

	numbers := make([]int, len(outputs))
	for i, out := range outputs {
		numbers[i] = out.Number
	}
	slices.Sort(numbers)
	if want := []int{2, 3, 4}; !slices.Equal(numbers, want) {
		t.Errorf("採番されたレス番号 = %v、期待値 = %v", numbers, want)
	}

	posts, err := f.postRepo.ListByThreadID(ctx, f.thread.ID)
	if err != nil {
		t.Fatalf("ListByThreadID()のエラー = %v", err)
	}
	if len(posts) != 4 {
		t.Fatalf("len(posts) = %d、期待値 = %d", len(posts), 4)
	}

	thread := findThread(t, f.db, f.thread.ID)
	if thread.PostsCount != 4 {
		t.Errorf("thread.PostsCount = %d、期待値 = %d", thread.PostsCount, 4)
	}
	last := posts[3]
	if thread.LastPostID == nil || *thread.LastPostID != last.ID {
		t.Errorf("thread.LastPostID = %v、期待値 = %v", thread.LastPostID, last.ID)
	}
}

// TestCreatePostUsecase_Execute_RollsBackOnFailureは、投稿とスレッドの件数を書いた
// 後の失敗がどちらも残さないことを検証します。上限の1件手前だったスレッドは1件手前の
// ままであり、通らなかった送信がそれを閉じたことにはなりません。
//
// 失敗を起こすのに参照の記録を拒否するトリガーを使うのは、UseCaseが具象のリポジトリを
// 保持していて、テストが失敗するものへ差し替えられないためです。拒否が当たるのは
// トランザクションの最後の書き込みであり、そこは投稿と件数が既に書かれた状態からの
// ロールバックを確かめられる地点です。
func TestCreatePostUsecase_Execute_RollsBackOnFailure(t *testing.T) {
	t.Parallel()

	f, ctx := newCreatePostUsecase(t)
	growThread(t, f.db, f.thread, f.starter, model.ThreadPostLimit-1)

	if _, err := f.db.Writer.ExecContext(ctx, `
		CREATE TRIGGER refuse_post_reference_insert BEFORE INSERT ON post_references
		BEGIN
			SELECT RAISE(ABORT, 'テストによる投稿参照の作成の拒否');
		END;
	`); err != nil {
		t.Fatalf("テスト用トリガーの作成に失敗: %v", err)
	}

	out, err := f.uc.Execute(ctx, usecase.CreatePostInput{
		ThreadID: f.thread.ID,
		UserID:   f.author,
		Body:     ">>1最後の1件です",
	})
	if out != nil {
		t.Error("Execute()のoutputはnilのはず")
	}
	if err == nil {
		t.Fatal("Execute()のエラー = nil、エラーを期待")
	}
	// この失敗は利用者が対処できるものではないため、フォームのエラーでも業務上の
	// 既知の結果でもない。ハンドラーは500を返す。
	if ve := model.AsValidationError(err); ve != nil {
		t.Errorf("Execute()のエラー = %v、期待値 = 素のerror", ve)
	}
	if ae := model.AsAppError(err); ae != nil {
		t.Errorf("Execute()のエラー = %v、期待値 = 素のerror", ae)
	}

	if got := countPosts(t, f.db); got != model.ThreadPostLimit-1 {
		t.Errorf("投稿の件数 = %d、期待値 = %d", got, model.ThreadPostLimit-1)
	}
	if got := countPostReferences(t, f.db); got != 0 {
		t.Errorf("投稿参照の件数 = %d、期待値 = %d", got, 0)
	}

	thread := findThread(t, f.db, f.thread.ID)
	if thread.PostsCount != model.ThreadPostLimit-1 {
		t.Errorf("thread.PostsCount = %d、期待値 = %d", thread.PostsCount, model.ThreadPostLimit-1)
	}
	if reasons := thread.LockReasons(); len(reasons) != 0 {
		t.Errorf("thread.LockReasons() = %v、期待値 = 空", reasons)
	}
}

// TestCreatePostUsecase_Execute_UnpublishedThreadは、管理者が見えない場所へ移した
// スレッドへの返信が非公開として拒否され、どこにも保存されないことを検証します。確認は
// 書き込みトランザクションの中で行うため、フォームが開かれている間に非公開になったスレッドは、
// その後に届いた送信を拒否します。それを隠す印の下に投稿を足したりはしません。
//
// この拒否を不在のスレッドや退会したアカウントと区別するのは、書き手にも本文にも落ち度が
// 無いためです。ハンドラーはスレッドの取り下げを述べるページで応答します。
func TestCreatePostUsecase_Execute_UnpublishedThread(t *testing.T) {
	t.Parallel()

	f, ctx := newCreatePostUsecase(t)

	if err := f.threadRepo.Unpublish(ctx, f.thread.ID); err != nil {
		t.Fatalf("Unpublish()のエラー = %v", err)
	}

	out, err := f.uc.Execute(ctx, usecase.CreatePostInput{
		ThreadID: f.thread.ID,
		UserID:   f.author,
		Body:     "こんにちは",
	})
	if out != nil {
		t.Error("Execute()のoutputはnilのはず")
	}
	assertAppErrCode(t, err, model.AppErrCodeResourceUnpublished)
	if got := countPosts(t, f.db); got != 1 {
		t.Errorf("投稿の件数 = %d、期待値 = %d", got, 1)
	}
}
