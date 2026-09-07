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

// createPostFixture is the UseCase under test together with the thread replies
// are written to, its first post, the account writing them, and the
// repositories the assertions read the saved rows back through.
//
// [Ja] createPostFixture は、テスト対象の UseCase と、返信を書く先のスレッド、その最初の
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

// newCreatePostUsecase builds the UseCase over a database holding one board,
// one thread with its first post, and a second account to reply with.
//
// The replier is not the account that started the thread, so the interval
// between one person's posts does not stand between the fixture and the reply a
// test is about to make.
//
// [Ja] newCreatePostUsecase は、掲示板が 1 つ、最初の投稿を持つスレッドが 1 つ、そして
// 返信するための 2 つ目のアカウントがあるデータベース上に UseCase を構築します。
//
// 返信する側をスレッドを立てたアカウントと別にするのは、1 人の投稿の間隔が、フィクスチャと
// テストがこれから行う返信の間に立たないようにするためです。
func newCreatePostUsecase(t *testing.T) (createPostFixture, context.Context) {
	t.Helper()

	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)
	return newCreatePostUsecaseOn(t, testutil.SetupDB(t)), ctx
}

// newCreatePostUsecaseOn seeds the board, thread and accounts into db and builds
// the UseCase over it. It takes the database rather than opening one so that a
// test can hand it a pool it opened itself.
//
// [Ja] newCreatePostUsecaseOn は掲示板・スレッド・アカウントを db に投入し、その上に
// UseCase を構築します。データベースを開かずに受け取るのは、テストが自分で開いたプールを
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

// newCreatePostUsecaseOver builds the UseCase over db's pools, for a test that
// needs a second one reaching the same database file through a pool of its own.
//
// [Ja] newCreatePostUsecaseOver は db のプール上に UseCase を構築します。同じデータベース
// ファイルへ自身のプール経由で届く 2 つ目の UseCase を必要とするテストのためのものです。
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

// seedThread writes a thread together with its first post and the thread's view
// of that post, which is the state a thread exists in once it has been started.
//
// [Ja] seedThread は、スレッドとその最初の投稿、そしてスレッドが持つその投稿の姿を
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

// growThread fills the thread up to count posts, writing the rows and the
// thread's view of them directly rather than through the UseCase: a test about
// what happens near the cap would otherwise spend a thousand write transactions
// getting there, each waiting out the interval between the author's posts.
//
// The posts are dated an hour ago so that the account they are attributed to is
// not left inside its own interval by the arrangement.
//
// [Ja] growThread はスレッドを count 件まで満たします。行とスレッドが持つその姿を UseCase
// ではなく直接書くのは、そうしなければ上限の近くで何が起きるかを問うテストが、そこへ至る
// までに 1000 回の書き込みトランザクションを費やし、そのたびに投稿者の間隔を待つことに
// なるためです。
//
// 投稿を 1 時間前の日時にするのは、これらが帰属するアカウントを、この用意によって自身の
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

// findThread reads the thread back, for an assertion about the state it was
// left in.
//
// [Ja] findThread はスレッドを読み戻します。どの状態で残されたかを問う検証のためのもの
// です。
func findThread(t *testing.T, db *database.DB, id model.ThreadID) *model.Thread {
	t.Helper()

	thread, err := repository.NewThreadRepository(db).FindByID(context.Background(), id)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}
	if thread == nil {
		t.Fatalf("スレッドを id で引けない: id=%v", id)
	}
	return thread
}

// countPostReferences reads how many references the whole database holds, so a
// test can say that a refused reply recorded none rather than that a particular
// row is missing.
//
// [Ja] countPostReferences はデータベース全体が持つ参照の件数を読みます。拒否された返信が
// 特定の行を欠いていることではなく、1 件も記録していないことをテストが述べられるように
// するためです。
func countPostReferences(t *testing.T, db *database.DB) int {
	t.Helper()

	var count int
	if err := db.Reader.QueryRowContext(context.Background(), "SELECT count(*) FROM post_references").Scan(&count); err != nil {
		t.Fatalf("投稿参照の件数の取得に失敗: %v", err)
	}
	return count
}

// replySubmission is one reply about to be sent: which UseCase sends it, which
// thread it is written to, and which account it is attributed to.
//
// [Ja] replySubmission は、これから送られる 1 つの返信です。どの UseCase が送り、どの
// スレッドへ書かれ、どのアカウントに帰属するかを持ちます。
type replySubmission struct {
	uc       *usecase.CreatePostUsecase
	threadID model.ThreadID
	author   model.UserID
}

// submitAtOnce sends every submission at the same time and returns what each one
// answered, in the order they were given.
//
// The goroutines wait on a barrier before sending, so that the submissions are
// in flight together rather than one of them possibly finishing before the next
// is even started. What these tests ask of the outcome is decided by the write
// lock, and the lock is only put to the test while more than one submission is
// holding a transaction open.
//
// [Ja] submitAtOnce はすべての送信を同時に行い、それぞれが返したものを、渡された順で
// 返します。
//
// goroutine は送信の前に待ち合わせます。そうしなければ、1 つが次の起動を待たずに完走して
// しまいうるためです。これらのテストが結果に問うことを決めるのは書き込みロックであり、
// ロックが試されるのは 2 つ以上の送信が同時にトランザクションを開いている間だけです。
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

// TestCreatePostUsecase_Execute_Success verifies that a valid reply is saved
// with the next reply number, attributed to its author, and that the thread's
// view of its posts describes it afterwards.
//
// [Ja] TestCreatePostUsecase_Execute_Success は、妥当な返信が次のレス番号を付けて作者に
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
		t.Fatalf("Execute() error = %v", err)
	}
	if out == nil {
		t.Fatal("Execute() output = nil")
	}
	if out.Number != 2 {
		t.Errorf("out.Number = %d, want %d", out.Number, 2)
	}

	posts, err := f.postRepo.ListByThreadID(ctx, f.thread.ID)
	if err != nil {
		t.Fatalf("ListByThreadID() error = %v", err)
	}
	if len(posts) != 2 {
		t.Fatalf("len(posts) = %d, want %d", len(posts), 2)
	}
	reply := posts[1]
	if reply.Number != 2 {
		t.Errorf("reply.Number = %d, want %d", reply.Number, 2)
	}
	// The body is stored as the validator normalized it: the whitespace around it
	// is part of how it reads and is kept, while the line break is stored as the
	// one form the application holds.
	//
	// [Ja] 本文は validator が正規化した形で保存される。前後の空白は本文の読まれ方の一部と
	// して保たれ、改行はアプリケーションが保持する 1 つの形で保存される。
	if want := "  バラード集がいいです\n特に2曲目  "; reply.Body != want {
		t.Errorf("reply.Body = %q, want %q", reply.Body, want)
	}
	if reply.UserID == nil || *reply.UserID != f.author {
		t.Errorf("reply.UserID = %v, want %v", reply.UserID, f.author)
	}

	thread := findThread(t, f.db, f.thread.ID)
	if thread.PostsCount != 2 {
		t.Errorf("thread.PostsCount = %d, want %d", thread.PostsCount, 2)
	}
	if thread.LastPostID == nil || *thread.LastPostID != reply.ID {
		t.Errorf("thread.LastPostID = %v, want %v", thread.LastPostID, reply.ID)
	}
	if !thread.LastPostedAt.Equal(reply.CreatedAt) {
		t.Errorf("thread.LastPostedAt = %v, want %v", thread.LastPostedAt, reply.CreatedAt)
	}
}

// TestCreatePostUsecase_Execute_SavesReferences verifies which of the reply
// numbers a body names become references: a number a post below it carries does,
// and the same number written twice still records the one relationship. A number
// nothing carries, the reply's own number, and a number ahead of it record
// nothing, so the reverse links a post shows are the ones that exist.
//
// [Ja] TestCreatePostUsecase_Execute_SavesReferences は、本文が名指すレス番号のうち何が
// 参照になるかを検証します。下の投稿が持つ番号は参照になり、同じ番号を 2 度書いても記録
// されるのは 1 つの関係です。どの投稿も持たない番号・その返信自身の番号・それより先の番号は
// 何も記録しないため、投稿が見せる逆リンクは実在するものになります。
func TestCreatePostUsecase_Execute_SavesReferences(t *testing.T) {
	t.Parallel()

	f, ctx := newCreatePostUsecase(t)

	out, err := f.uc.Execute(ctx, usecase.CreatePostInput{
		ThreadID: f.thread.ID,
		UserID:   f.author,
		Body:     ">>1 に同意。>>1 の2曲目が好き (>>2 は自分、>>99 はまだ無い)",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	references, err := f.referenceRepo.ListByReferencedPostIDs(ctx, []model.PostID{f.firstPost.ID})
	if err != nil {
		t.Fatalf("ListByReferencedPostIDs() error = %v", err)
	}
	if len(references) != 1 {
		t.Fatalf("len(references) = %d, want %d", len(references), 1)
	}

	posts, err := f.postRepo.ListByThreadID(ctx, f.thread.ID)
	if err != nil {
		t.Fatalf("ListByThreadID() error = %v", err)
	}
	reply := posts[out.Number-1]
	if references[0].PostID != reply.ID {
		t.Errorf("references[0].PostID = %v, want %v", references[0].PostID, reply.ID)
	}
	if references[0].ReferencedPostID != f.firstPost.ID {
		t.Errorf("references[0].ReferencedPostID = %v, want %v", references[0].ReferencedPostID, f.firstPost.ID)
	}

	if got := countPostReferences(t, f.db); got != 1 {
		t.Errorf("投稿参照の件数 = %d, want %d", got, 1)
	}
}

// TestCreatePostUsecase_Execute_InvalidInput verifies that a body with something
// to fix comes back as a *model.ValidationError naming the field, and that
// nothing is written for it.
//
// [Ja] TestCreatePostUsecase_Execute_InvalidInput は、直すところのある本文がフィールドを
// 名指す *model.ValidationError として戻り、そのために何も書かれないことを検証します。
func TestCreatePostUsecase_Execute_InvalidInput(t *testing.T) {
	t.Parallel()

	f, ctx := newCreatePostUsecase(t)

	out, err := f.uc.Execute(ctx, usecase.CreatePostInput{
		ThreadID: f.thread.ID,
		UserID:   f.author,
		Body:     "   ",
	})
	if out != nil {
		t.Error("Execute() output は nil のはず")
	}
	ve := model.AsValidationError(err)
	if ve == nil {
		t.Fatalf("Execute() error = %v, want *model.ValidationError", err)
	}
	if !ve.HasFieldError("body") {
		t.Error("ve に body のエラーが無い")
	}

	if got := countPosts(t, f.db); got != 1 {
		t.Errorf("投稿の件数 = %d, want %d", got, 1)
	}
	if thread := findThread(t, f.db, f.thread.ID); thread.PostsCount != 1 {
		t.Errorf("thread.PostsCount = %d, want %d", thread.PostsCount, 1)
	}
}

// TestCreatePostUsecase_Execute_UnknownThread verifies that an id naming no
// thread is reported as a missing resource, which is what lets the handler
// answer 404 instead of saving a reply nothing displays.
//
// [Ja] TestCreatePostUsecase_Execute_UnknownThread は、どのスレッドも指さない id が
// リソース未存在として報告されることを検証します。これにより、どこにも表示されない返信を
// 保存する代わりに、ハンドラーは 404 を返せます。
func TestCreatePostUsecase_Execute_UnknownThread(t *testing.T) {
	t.Parallel()

	f, ctx := newCreatePostUsecase(t)

	out, err := f.uc.Execute(ctx, usecase.CreatePostInput{
		ThreadID: f.thread.ID + 1000,
		UserID:   f.author,
		Body:     "こんにちは",
	})
	if out != nil {
		t.Error("Execute() output は nil のはず")
	}
	ae := model.AsAppError(err)
	if ae == nil {
		t.Fatalf("Execute() error = %v, want *model.AppError", err)
	}
	if ae.Code != model.AppErrCodeResourceNotFound {
		t.Errorf("ae.Code = %d, want %d", ae.Code, model.AppErrCodeResourceNotFound)
	}
	if got := countPosts(t, f.db); got != 1 {
		t.Errorf("投稿の件数 = %d, want %d", got, 1)
	}
}

// TestCreatePostUsecase_Execute_WithdrawnUser verifies that an account that has
// withdrawn cannot reply, even though its session was issued while it was still
// there.
//
// [Ja] TestCreatePostUsecase_Execute_WithdrawnUser は、退会したアカウントが返信できない
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
		t.Error("Execute() output は nil のはず")
	}
	ae := model.AsAppError(err)
	if ae == nil {
		t.Fatalf("Execute() error = %v, want *model.AppError", err)
	}
	if ae.Code != model.AppErrCodeForbidden {
		t.Errorf("ae.Code = %d, want %d", ae.Code, model.AppErrCodeForbidden)
	}
	if got := countPosts(t, f.db); got != 1 {
		t.Errorf("投稿の件数 = %d, want %d", got, 1)
	}
}

// TestCreatePostUsecase_Execute_WithdrawnUserOnLockedThread verifies that an
// account that has withdrawn hears about itself rather than about the thread,
// even when the thread it wrote to takes no further post. Neither refusal can be
// waited out, and the one naming the account is the one that holds wherever it
// writes next: the guidance a locked thread carries offers the next thread,
// which this account could not start either.
//
// [Ja] TestCreatePostUsecase_Execute_WithdrawnUserOnLockedThread は、退会したアカウントが、
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
		t.Error("Execute() output は nil のはず")
	}
	ae := model.AsAppError(err)
	if ae == nil {
		t.Fatalf("Execute() error = %v, want *model.AppError", err)
	}
	if ae.Code != model.AppErrCodeForbidden {
		t.Errorf("ae.Code = %d, want %d", ae.Code, model.AppErrCodeForbidden)
	}
	if len(ae.LockReasons) != 0 {
		t.Errorf("ae.LockReasons = %v, want 空", ae.LockReasons)
	}
}

// TestCreatePostUsecase_Execute_WithinPostInterval verifies that a second reply
// sent right after the first is refused with the wait it is refused for, and
// that the refusal writes nothing.
//
// [Ja] TestCreatePostUsecase_Execute_WithinPostInterval は、1 つ目のすぐ後に送った 2 つ目の
// 返信が、拒否の理由である待ち時間とともに拒否され、その拒否が何も書かないことを検証します。
func TestCreatePostUsecase_Execute_WithinPostInterval(t *testing.T) {
	t.Parallel()

	f, ctx := newCreatePostUsecase(t)
	input := usecase.CreatePostInput{ThreadID: f.thread.ID, UserID: f.author, Body: "こんにちは"}

	if _, err := f.uc.Execute(ctx, input); err != nil {
		t.Fatalf("1 つ目の Execute() error = %v", err)
	}

	out, err := f.uc.Execute(ctx, input)
	if out != nil {
		t.Error("Execute() output は nil のはず")
	}
	ae := model.AsAppError(err)
	if ae == nil {
		t.Fatalf("Execute() error = %v, want *model.AppError", err)
	}
	if ae.Code != model.AppErrCodeRateLimited {
		t.Errorf("ae.Code = %d, want %d", ae.Code, model.AppErrCodeRateLimited)
	}
	if ae.RetryAfter <= 0 || ae.RetryAfter > model.PostInterval {
		t.Errorf("ae.RetryAfter = %s, want (0, %s]", ae.RetryAfter, model.PostInterval)
	}

	if got := countPosts(t, f.db); got != 2 {
		t.Errorf("投稿の件数 = %d, want %d", got, 2)
	}
	if thread := findThread(t, f.db, f.thread.ID); thread.PostsCount != 2 {
		t.Errorf("thread.PostsCount = %d, want %d", thread.PostsCount, 2)
	}
}

// TestCreatePostUsecase_Execute_IntervalIncludesStartingAThread verifies that
// the post a thread is started with counts towards the same interval: a person
// who has just started a thread waits before replying, here in a thread they
// did not start.
//
// [Ja] TestCreatePostUsecase_Execute_IntervalIncludesStartingAThread は、スレッドを立てた
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
		t.Fatalf("スレッド作成の Execute() error = %v", err)
	}

	out, err := f.uc.Execute(ctx, usecase.CreatePostInput{
		ThreadID: f.thread.ID,
		UserID:   f.author,
		Body:     "こちらにも一言",
	})
	if out != nil {
		t.Error("Execute() output は nil のはず")
	}
	ae := model.AsAppError(err)
	if ae == nil {
		t.Fatalf("Execute() error = %v, want *model.AppError", err)
	}
	if ae.Code != model.AppErrCodeRateLimited {
		t.Errorf("ae.Code = %d, want %d", ae.Code, model.AppErrCodeRateLimited)
	}
	if thread := findThread(t, f.db, f.thread.ID); thread.PostsCount != 1 {
		t.Errorf("thread.PostsCount = %d, want %d", thread.PostsCount, 1)
	}
}

// TestCreatePostUsecase_Execute_LockedThread verifies that a thread holding
// every post it can hold takes no more: the reply is refused with the reason
// carried as a value, and it leaves the thread's count, the references and the
// author's interval as they were.
//
// The interval is asserted through a reply to another thread going through
// immediately afterwards, because what the refusal must not do is start the
// author's next wait: a submission nothing was saved for cannot be what spaces
// out the following one.
//
// [Ja] TestCreatePostUsecase_Execute_LockedThread は、持てる投稿をすべて持っているスレッド
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
		Body:     ">>1 まだ間に合いますか?",
	})
	if out != nil {
		t.Error("Execute() output は nil のはず")
	}
	ae := model.AsAppError(err)
	if ae == nil {
		t.Fatalf("Execute() error = %v, want *model.AppError", err)
	}
	if ae.Code != model.AppErrCodeThreadLocked {
		t.Errorf("ae.Code = %d, want %d", ae.Code, model.AppErrCodeThreadLocked)
	}
	// The reasons travel as the values the model produced, so the handler chooses
	// what to show without reading them back out of a message.
	//
	// [Ja] 理由はモデルが生んだ値のまま運ばれる。ハンドラーが何を見せるかを、メッセージから
	// 読み取り直さずに選べるようにするためである。
	want := []model.ThreadLockReason{model.ThreadLockReasonPostLimitReached}
	if !slices.Equal(ae.LockReasons, want) {
		t.Errorf("ae.LockReasons = %v, want %v", ae.LockReasons, want)
	}

	if got := countPosts(t, f.db); got != model.ThreadPostLimit {
		t.Errorf("投稿の件数 = %d, want %d", got, model.ThreadPostLimit)
	}
	if thread := findThread(t, f.db, f.thread.ID); thread.PostsCount != model.ThreadPostLimit {
		t.Errorf("thread.PostsCount = %d, want %d", thread.PostsCount, model.ThreadPostLimit)
	}
	if got := countPostReferences(t, f.db); got != 0 {
		t.Errorf("投稿参照の件数 = %d, want %d", got, 0)
	}

	other, _ := seedThread(t, f.db, f.board.ID, f.starter, "次のスレッド")
	if _, err := f.uc.Execute(ctx, usecase.CreatePostInput{
		ThreadID: other.ID,
		UserID:   f.author,
		Body:     "こちらに書きます",
	}); err != nil {
		t.Fatalf("ロックによる拒否の直後の Execute() error = %v", err)
	}
}

// TestCreatePostUsecase_Execute_LocksTheThreadAtTheCap verifies that the post
// that fills a thread is saved, and that the thread is locked from the moment
// that post is committed: the next reply is refused, without anything having run
// in between to mark the thread as full.
//
// [Ja] TestCreatePostUsecase_Execute_LocksTheThreadAtTheCap は、スレッドを満たす投稿が
// 保存されること、そしてその投稿がコミットされた時点からスレッドがロック中であることを
// 検証します。次の返信は拒否されます。その間に、スレッドを満杯と印付ける何かが走ることは
// ありません。
func TestCreatePostUsecase_Execute_LocksTheThreadAtTheCap(t *testing.T) {
	t.Parallel()

	f, ctx := newCreatePostUsecase(t)
	thread := growThread(t, f.db, f.thread, f.starter, model.ThreadPostLimit-1)
	if len(thread.LockReasons()) != 0 {
		t.Fatalf("上限の 1 件手前のスレッドの LockReasons() = %v, want 空", thread.LockReasons())
	}

	out, err := f.uc.Execute(ctx, usecase.CreatePostInput{
		ThreadID: f.thread.ID,
		UserID:   f.author,
		Body:     "最後の 1 件です",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if out == nil {
		t.Fatal("Execute() output = nil")
	}
	if out.Number != model.ThreadPostLimit {
		t.Errorf("out.Number = %d, want %d", out.Number, model.ThreadPostLimit)
	}

	filled := findThread(t, f.db, f.thread.ID)
	if filled.PostsCount != model.ThreadPostLimit {
		t.Errorf("thread.PostsCount = %d, want %d", filled.PostsCount, model.ThreadPostLimit)
	}
	want := []model.ThreadLockReason{model.ThreadLockReasonPostLimitReached}
	if !slices.Equal(filled.LockReasons(), want) {
		t.Errorf("thread.LockReasons() = %v, want %v", filled.LockReasons(), want)
	}

	// Another account replies next, so what refuses the submission is the thread's
	// state rather than the interval between one person's posts.
	//
	// [Ja] 次に返信するのを別のアカウントにするのは、送信を拒否するものを、1 人の投稿の
	// 間隔ではなくスレッドの状態にするためである。
	_, err = f.uc.Execute(ctx, usecase.CreatePostInput{
		ThreadID: f.thread.ID,
		UserID:   testutil.NewUserBuilder(t, f.db).Build(),
		Body:     "まだ書けますか?",
	})
	ae := model.AsAppError(err)
	if ae == nil || ae.Code != model.AppErrCodeThreadLocked {
		t.Fatalf("Execute() error = %v, want *model.AppError (%d)", err, model.AppErrCodeThreadLocked)
	}
	if got := countPosts(t, f.db); got != model.ThreadPostLimit {
		t.Errorf("投稿の件数 = %d, want %d", got, model.ThreadPostLimit)
	}
}

// TestCreatePostUsecase_Execute_ConcurrentAtTheCap verifies that two accounts
// replying at once to a thread one post short of the cap produce one post and
// one refusal: the count is read under the write lock, so the second submission
// sees the post the first committed rather than the state before it, and the cap
// is not passed by a reply that was numbered from a stale count.
//
// [Ja] TestCreatePostUsecase_Execute_ConcurrentAtTheCap は、上限の 1 件手前のスレッドへ
// 2 つのアカウントが同時に返信すると、1 つの投稿と 1 つの拒否になることを検証します。件数は
// 書き込みロックの下で読むため、2 つ目の送信が見るのは 1 つ目がコミットした投稿であって
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

// TestCreatePostUsecase_Execute_ConcurrentAcrossConnectionPools verifies that
// SQLite's write lock enforces the post cap even when submissions use separate
// connection pools opened on the same database file.
//
// A single pool caps the writer at one connection, so submissions through it are
// serialized before SQLite is asked anything. Opening a second pool takes that
// away and leaves the outcome to the write lock the database file itself holds.
//
// [Ja] TestCreatePostUsecase_Execute_ConcurrentAcrossConnectionPools は、同じデータベース
// ファイルに対して開かれた別々の接続プールから投稿しても、SQLiteの書き込みロックが
// 投稿数の上限を守ることを検証します。
//
// 1 つのプールは書き込み用コネクションを 1 本に制限するため、そこを通る送信は SQLite に
// 何かを尋ねる前に直列化されます。2 つ目のプールを開くとそれが無くなり、結果はデータベース
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

// TestCreatePostUsecase_Execute_ConcurrentByTheSameUserAcrossThreads verifies
// that one account's post interval also holds across threads and connection
// pools. Only one reply may be saved, since changing the destination does not
// give the author another posting interval.
//
// [Ja] TestCreatePostUsecase_Execute_ConcurrentByTheSameUserAcrossThreads は、同じ
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
				t.Fatalf("submissions[%d] の output = nil", i)
			}
			if outputs[i].Number != 2 {
				t.Errorf("submissions[%d] の Number = %d, want 2", i, outputs[i].Number)
			}
		} else {
			if outputs[i] != nil {
				t.Errorf("submissions[%d] の output は nil のはず", i)
			}
			ae := model.AsAppError(err)
			if ae == nil || ae.Code != model.AppErrCodeRateLimited {
				t.Fatalf("submissions[%d] の error = %v, want *model.AppError (%d)", i, err, model.AppErrCodeRateLimited)
			}
			if ae.RetryAfter <= 0 || ae.RetryAfter > model.PostInterval {
				t.Errorf("submissions[%d] の RetryAfter = %s, want (0, %s]", i, ae.RetryAfter, model.PostInterval)
			}
		}

		thread := findThread(t, f.db, submissions[i].threadID)
		if thread.PostsCount != wantCount {
			t.Errorf("submissions[%d] の thread.PostsCount = %d, want %d", i, thread.PostsCount, wantCount)
		}
	}
	if succeeded != 1 {
		t.Errorf("成功した送信 = %d件, want 1件", succeeded)
	}
	if got := countPosts(t, f.db); got != before+1 {
		t.Errorf("投稿の件数 = %d, want %d", got, before+1)
	}
}

// openDB opens another pool onto the database file at path and closes it when
// the test ends.
//
// [Ja] openDB は path のデータベースファイルに対してもう 1 つプールを開き、テストの終了時に
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

// assertOneFilledTheCap states what two submissions racing for the last place in
// a thread must come to: one of them takes it with the cap's own number, the
// other is told the thread is locked, and the thread ends up holding exactly the
// cap.
//
// [Ja] assertOneFilledTheCap は、スレッドの最後の 1 枠を奪い合う 2 つの送信が至るべき結果を
// 述べます。一方が上限そのものの番号でそこを取り、他方はスレッドがロック中であると告げられ、
// スレッドはちょうど上限の件数を持って終わります。
func assertOneFilledTheCap(t *testing.T, f createPostFixture, outputs []*usecase.CreatePostOutput, errs []error) {
	t.Helper()

	succeeded := 0
	for i, err := range errs {
		if err == nil {
			succeeded++
			if outputs[i] == nil {
				t.Fatalf("submissions[%d] の output = nil", i)
			}
			if outputs[i].Number != model.ThreadPostLimit {
				t.Errorf("submissions[%d] の Number = %d, want %d", i, outputs[i].Number, model.ThreadPostLimit)
			}
			continue
		}
		ae := model.AsAppError(err)
		if ae == nil || ae.Code != model.AppErrCodeThreadLocked {
			t.Errorf("submissions[%d] の error = %v, want *model.AppError (%d)", i, err, model.AppErrCodeThreadLocked)
		}
	}
	if succeeded != 1 {
		t.Errorf("成功した送信 = %d 件, want %d 件", succeeded, 1)
	}

	if got := countPosts(t, f.db); got != model.ThreadPostLimit {
		t.Errorf("投稿の件数 = %d, want %d", got, model.ThreadPostLimit)
	}
	if thread := findThread(t, f.db, f.thread.ID); thread.PostsCount != model.ThreadPostLimit {
		t.Errorf("thread.PostsCount = %d, want %d", thread.PostsCount, model.ThreadPostLimit)
	}
}

// TestCreatePostUsecase_Execute_ConcurrentByDifferentUsers verifies that replies
// sent at once by different accounts are each given a reply number of their own
// and are all kept, and that the thread ends up describing the last of them.
//
// [Ja] TestCreatePostUsecase_Execute_ConcurrentByDifferentUsers は、別々のアカウントから
// 同時に送られた返信が、それぞれ自分のレス番号を与えられてすべて残ること、そしてスレッドが
// その最後の 1 件を表して終わることを検証します。
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
			t.Fatalf("submissions[%d] の Execute() error = %v", i, err)
		}
	}

	numbers := make([]int, len(outputs))
	for i, out := range outputs {
		numbers[i] = out.Number
	}
	slices.Sort(numbers)
	if want := []int{2, 3, 4}; !slices.Equal(numbers, want) {
		t.Errorf("採番されたレス番号 = %v, want %v", numbers, want)
	}

	posts, err := f.postRepo.ListByThreadID(ctx, f.thread.ID)
	if err != nil {
		t.Fatalf("ListByThreadID() error = %v", err)
	}
	if len(posts) != 4 {
		t.Fatalf("len(posts) = %d, want %d", len(posts), 4)
	}

	thread := findThread(t, f.db, f.thread.ID)
	if thread.PostsCount != 4 {
		t.Errorf("thread.PostsCount = %d, want %d", thread.PostsCount, 4)
	}
	last := posts[3]
	if thread.LastPostID == nil || *thread.LastPostID != last.ID {
		t.Errorf("thread.LastPostID = %v, want %v", thread.LastPostID, last.ID)
	}
}

// TestCreatePostUsecase_Execute_RollsBackOnFailure verifies that a failure after
// the post and the thread's count are written leaves neither behind: the thread
// that was one post short of the cap is still one short, so a submission that
// did not go through cannot be what closed it.
//
// The failure is provoked by a trigger that refuses to record a reference,
// because the UseCase holds concrete repositories that a test cannot swap for
// failing ones. The refusal lands on the last write of the transaction, which is
// the point where the post and the count are already written to roll back.
//
// [Ja] TestCreatePostUsecase_Execute_RollsBackOnFailure は、投稿とスレッドの件数を書いた
// 後の失敗がどちらも残さないことを検証します。上限の 1 件手前だったスレッドは 1 件手前の
// ままであり、通らなかった送信がそれを閉じたことにはなりません。
//
// 失敗を起こすのに参照の記録を拒否するトリガーを使うのは、UseCase が具象のリポジトリを
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
		Body:     ">>1 最後の 1 件です",
	})
	if out != nil {
		t.Error("Execute() output は nil のはず")
	}
	if err == nil {
		t.Fatal("Execute() error = nil, want error")
	}
	// The failure is not one the user can act on, so it is neither a form error
	// nor a known business outcome: the handler answers 500.
	//
	// [Ja] この失敗は利用者が対処できるものではないため、フォームのエラーでも業務上の
	// 既知の結果でもない。ハンドラーは 500 を返す。
	if ve := model.AsValidationError(err); ve != nil {
		t.Errorf("Execute() error = %v, want 素の error", ve)
	}
	if ae := model.AsAppError(err); ae != nil {
		t.Errorf("Execute() error = %v, want 素の error", ae)
	}

	if got := countPosts(t, f.db); got != model.ThreadPostLimit-1 {
		t.Errorf("投稿の件数 = %d, want %d", got, model.ThreadPostLimit-1)
	}
	if got := countPostReferences(t, f.db); got != 0 {
		t.Errorf("投稿参照の件数 = %d, want %d", got, 0)
	}

	thread := findThread(t, f.db, f.thread.ID)
	if thread.PostsCount != model.ThreadPostLimit-1 {
		t.Errorf("thread.PostsCount = %d, want %d", thread.PostsCount, model.ThreadPostLimit-1)
	}
	if reasons := thread.LockReasons(); len(reasons) != 0 {
		t.Errorf("thread.LockReasons() = %v, want 空", reasons)
	}
}
