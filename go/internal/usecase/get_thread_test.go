package usecase_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
)

// threadFixture is the thread a test reads back, together with the rows around
// it the assertions are written against.
//
// [Ja] threadFixture はテストが読み戻すスレッドと、その周りの、検証が対象とする行です。
type threadFixture struct {
	uc        *usecase.GetThreadUsecase
	thread    *model.Thread
	other     *model.Thread
	author    model.UserID
	withdrawn model.UserID
}

// newGetThreadUsecase builds the UseCase over a database holding one community
// whose "music" category lists a "jazz" board with two threads in it.
//
// The thread the tests read carries three posts: one by an account that is still
// there, one by an account that has since withdrawn, and one replying to both of
// them. The second thread exists so the assertions show that a thread's posts are
// its own rather than every post in the board.
//
// [Ja] newGetThreadUsecase は、1 つのコミュニティを持つデータベース上に UseCase を
// 構築します。その "music" カテゴリーは "jazz" 掲示板を並べ、その中に 2 つのスレッドが
// 立っています。
//
// テストが読むスレッドは 3 つの投稿を持ちます。まだ存在するアカウントによるもの、その後
// 退会したアカウントによるもの、そしてその両方に返信したものです。2 つ目のスレッドがある
// のは、スレッドの投稿が掲示板のすべての投稿ではなくそのスレッド自身のものであることを
// 検証が示せるようにするためです。
func newGetThreadUsecase(t *testing.T) threadFixture {
	t.Helper()

	ctx := context.Background()
	db := testutil.SetupDB(t)

	categoryRepo := repository.NewCategoryRepository(db)
	boardRepo := repository.NewBoardRepository(db)
	threadRepo := repository.NewThreadRepository(db)
	postRepo := repository.NewPostRepository(db)
	postReferenceRepo := repository.NewPostReferenceRepository(db)

	music, err := categoryRepo.Create(ctx, repository.CreateCategoryInput{Slug: "music", Name: "音楽"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	jazz, err := boardRepo.Create(ctx, repository.CreateBoardInput{CategoryID: &music.ID, Slug: "jazz", Name: "ジャズ"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	author := testutil.NewUserBuilder(t, db).WithAtname("alice").WithEmail("alice@example.com").Build()
	withdrawn := testutil.NewUserBuilder(t, db).
		WithAtname("bob").
		WithEmail("bob@example.com").
		WithDeletedAt(time.Now().Add(-24 * time.Hour)).
		Build()

	thread, err := threadRepo.Create(ctx, repository.CreateThreadInput{BoardID: jazz.ID, UserID: &author, Title: "枯葉の名演"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	first, err := postRepo.Create(ctx, repository.CreatePostInput{ThreadID: thread.ID, UserID: &author, Number: 1, Body: "好きな演奏は?"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	second, err := postRepo.Create(ctx, repository.CreatePostInput{ThreadID: thread.ID, UserID: &withdrawn, Number: 2, Body: ">>1 Bill Evans"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	third, err := postRepo.Create(ctx, repository.CreatePostInput{ThreadID: thread.ID, Number: 3, Body: ">>1 >>2 同意"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	for _, reference := range []repository.CreatePostReferenceInput{
		{PostID: second.ID, ReferencedPostID: first.ID},
		{PostID: third.ID, ReferencedPostID: first.ID},
		{PostID: third.ID, ReferencedPostID: second.ID},
	} {
		if _, err := postReferenceRepo.Create(ctx, reference); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
	}
	if err := threadRepo.UpdateLastPost(ctx, thread.ID, repository.UpdateThreadLastPostInput{
		PostsCount:   3,
		LastPostID:   third.ID,
		LastPostedAt: third.CreatedAt,
	}); err != nil {
		t.Fatalf("UpdateLastPost() error = %v", err)
	}

	other, err := threadRepo.Create(ctx, repository.CreateThreadInput{BoardID: jazz.ID, Title: "別のスレッド"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := postRepo.Create(ctx, repository.CreatePostInput{ThreadID: other.ID, Number: 1, Body: "別の話"}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	return threadFixture{
		uc:        newGetThreadUsecaseForDB(db),
		thread:    thread,
		other:     other,
		author:    author,
		withdrawn: withdrawn,
	}
}

// newGetThreadUsecaseForDB builds the UseCase over the supplied application
// database.
//
// [Ja] newGetThreadUsecaseForDB は、渡されたアプリケーションデータベース上に UseCase を
// 構築します。
func newGetThreadUsecaseForDB(db *database.DB) *usecase.GetThreadUsecase {
	return usecase.NewGetThreadUsecase(
		repository.NewThreadRepository(db),
		repository.NewBoardRepository(db),
		repository.NewCategoryRepository(db),
		repository.NewPostRepository(db),
		repository.NewPostReferenceRepository(db),
		repository.NewUserRepository(db),
	)
}

// TestGetThreadUsecase_Execute verifies that Execute reads the thread, where it
// sits, and its posts in reply-number order, with each post carrying the account
// that wrote it and the reply numbers of the posts that answered it.
//
// [Ja] TestGetThreadUsecase_Execute は、Execute がスレッドとその在り処、そして投稿を
// レス番号順に読み、各投稿がそれを書いたアカウントと、それに答えた投稿のレス番号を運ぶ
// ことを検証します。
func TestGetThreadUsecase_Execute(t *testing.T) {
	t.Parallel()

	fixture := newGetThreadUsecase(t)
	ctx := context.Background()

	output, err := fixture.uc.Execute(ctx, usecase.GetThreadInput{ID: fixture.thread.ID})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if output.Thread.Title != "枯葉の名演" {
		t.Errorf("output.Thread.Title = %q, want %q", output.Thread.Title, "枯葉の名演")
	}
	if output.Board.Slug != "jazz" {
		t.Errorf("output.Board.Slug = %q, want %q", output.Board.Slug, "jazz")
	}
	if output.Category.Slug != "music" {
		t.Errorf("output.Category.Slug = %q, want %q", output.Category.Slug, "music")
	}

	if len(output.Posts) != 3 {
		t.Fatalf("len(output.Posts) = %d, want 3", len(output.Posts))
	}
	for i, post := range output.Posts {
		if post.Post.Number != i+1 {
			t.Errorf("output.Posts[%d].Post.Number = %d, want %d", i, post.Post.Number, i+1)
		}
	}

	if got := output.Posts[0].Author; got == nil || got.Atname != "alice" {
		t.Errorf("output.Posts[0].Author = %v, want alice", got)
	}

	// The reply numbers are what a >>N resolves back to, so the first post's
	// answers are the second and the third, and the second's is only the third.
	//
	// [Ja] レス番号は >>N が逆向きに解決する先である。したがって 1 つ目の投稿への答えは
	// 2 つ目と 3 つ目であり、2 つ目への答えは 3 つ目だけである。
	if got, want := output.Posts[0].ReplyNumbers, []int{2, 3}; !slices.Equal(got, want) {
		t.Errorf("output.Posts[0].ReplyNumbers = %v, want %v", got, want)
	}
	if got, want := output.Posts[1].ReplyNumbers, []int{3}; !slices.Equal(got, want) {
		t.Errorf("output.Posts[1].ReplyNumbers = %v, want %v", got, want)
	}
	if got := output.Posts[2].ReplyNumbers; len(got) != 0 {
		t.Errorf("output.Posts[2].ReplyNumbers = %v, want empty", got)
	}
}

// TestGetThreadUsecase_Execute_WithdrawnAuthor verifies that a post whose author
// has withdrawn comes back with no account resolved, while the post itself stays
// on the thread. A withdrawal takes the name off what was written, not the
// writing, so the reply quoting it keeps its context.
//
// [Ja] TestGetThreadUsecase_Execute_WithdrawnAuthor は、作者が退会した投稿が、
// アカウントの解決されない形で返り、投稿自身はスレッドに残ることを検証します。退会が外す
// のは書かれたものから名前であって書かれたものではないため、それを引用した返信は文脈を
// 保ちます。
func TestGetThreadUsecase_Execute_WithdrawnAuthor(t *testing.T) {
	t.Parallel()

	fixture := newGetThreadUsecase(t)
	ctx := context.Background()

	output, err := fixture.uc.Execute(ctx, usecase.GetThreadInput{ID: fixture.thread.ID})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	second := output.Posts[1]
	if second.Post.UserID == nil || *second.Post.UserID != fixture.withdrawn {
		t.Errorf("output.Posts[1].Post.UserID = %v, want %v", second.Post.UserID, fixture.withdrawn)
	}
	if second.Author != nil {
		t.Errorf("output.Posts[1].Author = %v, want nil (退会済みは解決しない)", second.Author)
	}
	if second.Post.Body != ">>1 Bill Evans" {
		t.Errorf("output.Posts[1].Post.Body = %q, want the body to survive the withdrawal", second.Post.Body)
	}
}

// TestGetThreadUsecase_Execute_OtherThreadsPostsAreExcluded verifies that a
// thread's posts are its own: the second thread in the same board contributes
// nothing to the first one's list.
//
// [Ja] TestGetThreadUsecase_Execute_OtherThreadsPostsAreExcluded は、スレッドの投稿が
// そのスレッド自身のものであること、すなわち同じ掲示板の 2 つ目のスレッドが 1 つ目の
// 一覧に何も足さないことを検証します。
func TestGetThreadUsecase_Execute_OtherThreadsPostsAreExcluded(t *testing.T) {
	t.Parallel()

	fixture := newGetThreadUsecase(t)
	ctx := context.Background()

	output, err := fixture.uc.Execute(ctx, usecase.GetThreadInput{ID: fixture.other.ID})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if len(output.Posts) != 1 {
		t.Fatalf("len(output.Posts) = %d, want 1", len(output.Posts))
	}
	if output.Posts[0].Post.Body != "別の話" {
		t.Errorf("output.Posts[0].Post.Body = %q, want %q", output.Posts[0].Post.Body, "別の話")
	}
}

// TestGetThreadUsecase_Execute_UnknownID verifies that an id naming no thread is
// reported as an AppError carrying AppErrCodeResourceNotFound, which is what lets
// the handler answer 404 with the shared not-found page rather than treating a
// guessed URL as a failure.
//
// [Ja] TestGetThreadUsecase_Execute_UnknownID は、どのスレッドも指さない id が
// AppErrCodeResourceNotFound を持つ AppError として報告されることを検証します。これに
// より、ハンドラーは推測された URL を失敗として扱うのではなく、共通の not-found ページで
// 404 を返せます。
func TestGetThreadUsecase_Execute_UnknownID(t *testing.T) {
	t.Parallel()

	fixture := newGetThreadUsecase(t)
	ctx := context.Background()

	output, err := fixture.uc.Execute(ctx, usecase.GetThreadInput{ID: fixture.thread.ID + 1000})
	if output != nil {
		t.Errorf("Execute() = %v, want nil", output)
	}

	ae := model.AsAppError(err)
	if ae == nil {
		t.Fatalf("Execute() error = %v, want *model.AppError", err)
	}
	if ae.Code != model.AppErrCodeResourceNotFound {
		t.Errorf("ae.Code = %v, want %v", ae.Code, model.AppErrCodeResourceNotFound)
	}
}

// TestGetThreadUsecase_Execute_LookupFailure verifies that a database that
// cannot be read is returned as a plain error rather than as the not-found
// AppError: an unreachable database does not mean the thread is gone, and
// answering 404 would tell a crawler to drop a page that still exists.
//
// [Ja] TestGetThreadUsecase_Execute_LookupFailure は、読めないデータベースが not-found の
// AppError ではなく素のエラーとして返ることを検証します。到達できないデータベースは
// スレッドが無くなったことを意味せず、404 で応答すればまだ存在するページを落とすよう
// クローラーに伝えてしまいます。
func TestGetThreadUsecase_Execute_LookupFailure(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	if err := db.Reader.Close(); err != nil {
		t.Fatalf("Reader の Close() error = %v", err)
	}

	output, err := newGetThreadUsecaseForDB(db).Execute(context.Background(), usecase.GetThreadInput{ID: 1})
	if output != nil {
		t.Errorf("Execute() = %v, want nil", output)
	}
	if err == nil {
		t.Fatal("Execute() error = nil, want error")
	}
	if ae := model.AsAppError(err); ae != nil {
		t.Errorf("Execute() error = %v, want a plain error rather than an AppError", ae)
	}
}
