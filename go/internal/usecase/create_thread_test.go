package usecase_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/validator"
)

// createThreadFixture is the UseCase under test together with the board threads
// are started in, the account starting them, and the repositories the assertions
// read the saved rows back through.
//
// [Ja] createThreadFixture は、テスト対象の UseCase と、スレッドを立てる先の掲示板、
// それを立てるアカウント、そして検証が保存された行を読み戻すためのリポジトリです。
type createThreadFixture struct {
	uc         *usecase.CreateThreadUsecase
	db         *database.DB
	board      *model.Board
	author     model.UserID
	threadRepo *repository.ThreadRepository
	postRepo   *repository.PostRepository
}

// newCreateThreadUsecase builds the UseCase over a database holding one board,
// "jazz", with nothing in it yet, and one account to write in it.
//
// [Ja] newCreateThreadUsecase は、"jazz" 掲示板が 1 つあってまだ何も無く、そこに書く
// アカウントが 1 つあるデータベース上に UseCase を構築します。
func newCreateThreadUsecase(t *testing.T) (createThreadFixture, context.Context) {
	t.Helper()

	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)
	db := testutil.SetupDB(t)

	categoryRepo := repository.NewCategoryRepository(db)
	boardRepo := repository.NewBoardRepository(db)
	threadRepo := repository.NewThreadRepository(db)
	postRepo := repository.NewPostRepository(db)
	userRepo := repository.NewUserRepository(db)

	category, err := categoryRepo.Create(ctx, repository.CreateCategoryInput{Slug: "music", Name: "音楽"})
	if err != nil {
		t.Fatalf("テスト用カテゴリーの作成に失敗: %v", err)
	}
	board, err := boardRepo.Create(ctx, repository.CreateBoardInput{CategoryID: &category.ID, Slug: "jazz", Name: "ジャズ"})
	if err != nil {
		t.Fatalf("テスト用掲示板の作成に失敗: %v", err)
	}

	uc := usecase.NewCreateThreadUsecase(
		db.Writer,
		validator.NewThreadCreateValidator(),
		boardRepo,
		threadRepo,
		postRepo,
		userRepo,
	)

	return createThreadFixture{
		uc:         uc,
		db:         db,
		board:      board,
		author:     testutil.NewUserBuilder(t, db).Build(),
		threadRepo: threadRepo,
		postRepo:   postRepo,
	}, ctx
}

// validCreateThreadInput is a submission that passes validation, for tests whose
// subject is what happens around a well-formed form rather than the form itself.
//
// [Ja] validCreateThreadInput は検証を通る送信であり、フォームそのものではなく、整った
// フォームの周りで何が起きるかを問うテストのためのものです。
func validCreateThreadInput(f createThreadFixture) usecase.CreateThreadInput {
	return usecase.CreateThreadInput{
		BoardSlug: f.board.Slug,
		UserID:    f.author,
		Title:     "枯葉の名演",
		Language:  string(model.LocaleJa.ThreadLanguage()),
		Body:      "好きな演奏は?",
	}
}

// countThreads and countPosts read how many rows the whole database holds, so a
// test can say that a refused submission left none behind rather than that a
// particular row is missing.
//
// [Ja] countThreads と countPosts は、データベース全体が持つ行数を読みます。拒否された
// 送信が特定の行を欠いていることではなく、1 行も残していないことをテストが述べられる
// ようにするためです。
func countThreads(t *testing.T, db *database.DB) int {
	t.Helper()

	var count int
	if err := db.Reader.QueryRowContext(context.Background(), "SELECT count(*) FROM threads").Scan(&count); err != nil {
		t.Fatalf("スレッドの件数の取得に失敗: %v", err)
	}
	return count
}

func countPosts(t *testing.T, db *database.DB) int {
	t.Helper()

	var count int
	if err := db.Reader.QueryRowContext(context.Background(), "SELECT count(*) FROM posts").Scan(&count); err != nil {
		t.Fatalf("投稿の件数の取得に失敗: %v", err)
	}
	return count
}

// TestCreateThreadUsecase_Execute_Success verifies that a valid submission
// leaves a thread carrying the chosen title and primary language, its first post
// numbered 1 and attributed to the author, and the thread's view of that post.
//
// [Ja] TestCreateThreadUsecase_Execute_Success は、妥当な送信が、選ばれたタイトルと主言語を
// 持つスレッド、1 番が付いて作者に帰属する最初の投稿、そしてスレッドが持つその投稿の姿を
// 残すことを検証します。
func TestCreateThreadUsecase_Execute_Success(t *testing.T) {
	t.Parallel()

	f, ctx := newCreateThreadUsecase(t)

	input := validCreateThreadInput(f)
	input.Title = "  枯葉の名演  "
	input.Language = string(model.LocaleEn.ThreadLanguage())

	out, err := f.uc.Execute(ctx, input)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if out == nil {
		t.Fatal("Execute() output = nil")
	}
	if out.ThreadID == 0 {
		t.Error("out.ThreadID は DB 採番で空でないはず")
	}
	if out.Number != 1 {
		t.Errorf("out.Number = %d, want %d", out.Number, 1)
	}

	thread, err := f.threadRepo.FindByID(ctx, out.ThreadID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}
	if thread == nil {
		t.Fatal("作成したスレッドを id で引けない")
	}
	if thread.BoardID != f.board.ID {
		t.Errorf("thread.BoardID = %v, want %v", thread.BoardID, f.board.ID)
	}
	if thread.UserID == nil || *thread.UserID != f.author {
		t.Errorf("thread.UserID = %v, want %v", thread.UserID, f.author)
	}
	// The title is stored as the validator normalized it, which is the value its
	// length was measured against.
	//
	// [Ja] タイトルは validator が正規化した形で保存される。長さを測った対象そのもので
	// ある。
	if thread.Title != "枯葉の名演" {
		t.Errorf("thread.Title = %q, want %q", thread.Title, "枯葉の名演")
	}
	if thread.Language != model.LocaleEn.ThreadLanguage() {
		t.Errorf("thread.Language = %q, want %q", thread.Language, model.LocaleEn.ThreadLanguage())
	}

	posts, err := f.postRepo.ListByThreadID(ctx, thread.ID)
	if err != nil {
		t.Fatalf("ListByThreadID() error = %v", err)
	}
	if len(posts) != 1 {
		t.Fatalf("len(posts) = %d, want %d", len(posts), 1)
	}
	post := posts[0]
	if post.Number != 1 {
		t.Errorf("post.Number = %d, want %d", post.Number, 1)
	}
	if post.Body != "好きな演奏は?" {
		t.Errorf("post.Body = %q, want %q", post.Body, "好きな演奏は?")
	}
	if post.UserID == nil || *post.UserID != f.author {
		t.Errorf("post.UserID = %v, want %v", post.UserID, f.author)
	}

	// The thread's denormalized view describes the post that was just written, so
	// the board's thread list draws a row without reading posts.
	//
	// [Ja] スレッドの非正規化された姿は、いま書かれた投稿を表す。掲示板のスレッド一覧が
	// posts を読まずに 1 行を描けるようにするためである。
	if thread.PostsCount != 1 {
		t.Errorf("thread.PostsCount = %d, want %d", thread.PostsCount, 1)
	}
	if thread.LastPostID == nil || *thread.LastPostID != post.ID {
		t.Errorf("thread.LastPostID = %v, want %v", thread.LastPostID, post.ID)
	}
	if !thread.LastPostedAt.Equal(post.CreatedAt) {
		t.Errorf("thread.LastPostedAt = %v, want %v", thread.LastPostedAt, post.CreatedAt)
	}
}

// TestCreateThreadUsecase_Execute_InvalidInput verifies that a form with
// something to fix comes back as a *model.ValidationError naming the fields, and
// that nothing is written for it.
//
// [Ja] TestCreateThreadUsecase_Execute_InvalidInput は、直すところのあるフォームが、
// フィールドを名指す *model.ValidationError として戻り、そのために何も書かれないことを
// 検証します。
func TestCreateThreadUsecase_Execute_InvalidInput(t *testing.T) {
	t.Parallel()

	f, ctx := newCreateThreadUsecase(t)

	input := validCreateThreadInput(f)
	input.Title = "   "
	input.Body = ""

	out, err := f.uc.Execute(ctx, input)
	if out != nil {
		t.Error("Execute() output は nil のはず")
	}
	ve := model.AsValidationError(err)
	if ve == nil {
		t.Fatalf("Execute() error = %v, want *model.ValidationError", err)
	}
	for _, field := range []string{"title", "body"} {
		if !ve.HasFieldError(field) {
			t.Errorf("ve に %q のエラーが無い", field)
		}
	}

	if got := countThreads(t, f.db); got != 0 {
		t.Errorf("スレッドの件数 = %d, want %d", got, 0)
	}
	if got := countPosts(t, f.db); got != 0 {
		t.Errorf("投稿の件数 = %d, want %d", got, 0)
	}
}

// TestCreateThreadUsecase_Execute_UnknownBoard verifies that a slug naming no
// board is reported as a missing resource, which is what lets the handler answer
// 404 instead of creating a thread nothing lists.
//
// [Ja] TestCreateThreadUsecase_Execute_UnknownBoard は、どの掲示板も指さない slug が
// リソース未存在として報告されることを検証します。これにより、どこにも並ばないスレッドを
// 作る代わりに、ハンドラーは 404 を返せます。
func TestCreateThreadUsecase_Execute_UnknownBoard(t *testing.T) {
	t.Parallel()

	f, ctx := newCreateThreadUsecase(t)

	input := validCreateThreadInput(f)
	input.BoardSlug = "unknown"

	out, err := f.uc.Execute(ctx, input)
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
	if got := countThreads(t, f.db); got != 0 {
		t.Errorf("スレッドの件数 = %d, want %d", got, 0)
	}
}

// TestCreateThreadUsecase_Execute_WithdrawnUser verifies that an account that has
// withdrawn cannot start a thread, even though its session was issued while it
// was still there.
//
// [Ja] TestCreateThreadUsecase_Execute_WithdrawnUser は、退会したアカウントがスレッドを
// 立てられないことを検証します。そのセッションが、まだ存在していたときに発行されたもので
// あってもです。
func TestCreateThreadUsecase_Execute_WithdrawnUser(t *testing.T) {
	t.Parallel()

	f, ctx := newCreateThreadUsecase(t)

	input := validCreateThreadInput(f)
	input.UserID = testutil.NewUserBuilder(t, f.db).WithDeletedAt(time.Now().Add(-24 * time.Hour)).Build()

	out, err := f.uc.Execute(ctx, input)
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
	if got := countThreads(t, f.db); got != 0 {
		t.Errorf("スレッドの件数 = %d, want %d", got, 0)
	}
}

// TestCreateThreadUsecase_Execute_WithinPostInterval verifies that a second
// thread started right after the first is refused with the wait it is refused
// for, and that the refusal writes nothing.
//
// [Ja] TestCreateThreadUsecase_Execute_WithinPostInterval は、1 つ目のすぐ後に立てた
// 2 つ目のスレッドが、拒否の理由である待ち時間とともに拒否され、その拒否が何も書かないことを
// 検証します。
func TestCreateThreadUsecase_Execute_WithinPostInterval(t *testing.T) {
	t.Parallel()

	f, ctx := newCreateThreadUsecase(t)

	if _, err := f.uc.Execute(ctx, validCreateThreadInput(f)); err != nil {
		t.Fatalf("1 つ目の Execute() error = %v", err)
	}

	out, err := f.uc.Execute(ctx, validCreateThreadInput(f))
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
	// The wait is carried as a duration the handler can act on, rather than as
	// text it would have to read a number back out of.
	//
	// [Ja] 待ち時間は、ハンドラーが数を読み取り直す必要のあるテキストではなく、そのまま
	// 使える時間として運ばれる。
	if ae.RetryAfter <= 0 || ae.RetryAfter > model.PostInterval {
		t.Errorf("ae.RetryAfter = %s, want (0, %s]", ae.RetryAfter, model.PostInterval)
	}

	if got := countThreads(t, f.db); got != 1 {
		t.Errorf("スレッドの件数 = %d, want %d", got, 1)
	}
	if got := countPosts(t, f.db); got != 1 {
		t.Errorf("投稿の件数 = %d, want %d", got, 1)
	}
}

// TestCreateThreadUsecase_Execute_AfterPostInterval checks that an existing post
// stops blocking its author once the interval has passed. Backdating the saved
// post exercises the database lookup without making the test wait in real time.
//
// [Ja] TestCreateThreadUsecase_Execute_AfterPostInterval は、間隔が過ぎると既存の投稿が
// 投稿者の次の作成を妨げなくなることを確認します。保存済み投稿の時刻を過去へ移すことで、
// 実時間を待たずにデータベースからの取得を含めて検証します。
func TestCreateThreadUsecase_Execute_AfterPostInterval(t *testing.T) {
	t.Parallel()

	f, ctx := newCreateThreadUsecase(t)
	input := validCreateThreadInput(f)

	first, err := f.uc.Execute(ctx, input)
	if err != nil {
		t.Fatalf("1 つ目の Execute() error = %v", err)
	}
	if first == nil {
		t.Fatal("1 つ目の Execute() output = nil")
	}

	firstPosts, err := f.postRepo.ListByThreadID(ctx, first.ThreadID)
	if err != nil {
		t.Fatalf("ListByThreadID() error = %v", err)
	}
	if len(firstPosts) != 1 {
		t.Fatalf("len(firstPosts) = %d, want 1", len(firstPosts))
	}
	testutil.BackdatePost(t, f.db, firstPosts[0].ID, time.Now().Add(-time.Hour))

	out, err := f.uc.Execute(ctx, input)
	if err != nil {
		t.Fatalf("間隔経過後の Execute() error = %v", err)
	}
	if out == nil {
		t.Fatal("間隔経過後の Execute() output = nil")
	}
	if out.ThreadID == first.ThreadID {
		t.Error("2 つ目のスレッドに新しい ID が発行されていない")
	}
	if out.Number != 1 {
		t.Errorf("out.Number = %d, want 1", out.Number)
	}
	if got := countThreads(t, f.db); got != 2 {
		t.Errorf("スレッドの件数 = %d, want 2", got)
	}
	if got := countPosts(t, f.db); got != 2 {
		t.Errorf("投稿の件数 = %d, want 2", got)
	}

	thread, err := f.threadRepo.FindByID(ctx, out.ThreadID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}
	if thread == nil {
		t.Fatal("2 つ目のスレッドを id で引けない")
	}
	posts, err := f.postRepo.ListByThreadID(ctx, thread.ID)
	if err != nil {
		t.Fatalf("ListByThreadID() error = %v", err)
	}
	if len(posts) != 1 {
		t.Fatalf("len(posts) = %d, want 1", len(posts))
	}
	post := posts[0]
	if post.Number != 1 {
		t.Errorf("post.Number = %d, want 1", post.Number)
	}
	if post.UserID == nil || *post.UserID != f.author {
		t.Errorf("post.UserID = %v, want %v", post.UserID, f.author)
	}
	if thread.PostsCount != 1 {
		t.Errorf("thread.PostsCount = %d, want 1", thread.PostsCount)
	}
	if thread.LastPostID == nil || *thread.LastPostID != post.ID {
		t.Errorf("thread.LastPostID = %v, want %v", thread.LastPostID, post.ID)
	}
	if !thread.LastPostedAt.Equal(post.CreatedAt) {
		t.Errorf("thread.LastPostedAt = %v, want %v", thread.LastPostedAt, post.CreatedAt)
	}
}

// TestCreateThreadUsecase_Execute_ConcurrentByTheSameUser verifies that two
// submissions sent at once by one account produce one thread and one refusal:
// the interval is read under the write lock, so the second submission sees the
// post the first committed rather than the state before it.
//
// [Ja] TestCreateThreadUsecase_Execute_ConcurrentByTheSameUser は、1 つのアカウントから
// 同時に送られた 2 つの送信が、1 つのスレッドと 1 つの拒否になることを検証します。間隔は
// 書き込みロックの下で読むため、2 つ目の送信が見るのは、1 つ目がコミットした投稿であって、
// その前の状態ではありません。
func TestCreateThreadUsecase_Execute_ConcurrentByTheSameUser(t *testing.T) {
	t.Parallel()

	f, ctx := newCreateThreadUsecase(t)

	const submissions = 2
	errs := make([]error, submissions)

	var wg sync.WaitGroup
	for i := range submissions {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = f.uc.Execute(ctx, validCreateThreadInput(f))
		}()
	}
	wg.Wait()

	succeeded := 0
	for _, err := range errs {
		if err == nil {
			succeeded++
			continue
		}
		ae := model.AsAppError(err)
		if ae == nil || ae.Code != model.AppErrCodeRateLimited {
			t.Errorf("Execute() error = %v, want *model.AppError (%d)", err, model.AppErrCodeRateLimited)
		}
	}
	if succeeded != 1 {
		t.Errorf("成功した送信 = %d 件, want %d 件", succeeded, 1)
	}

	if got := countThreads(t, f.db); got != 1 {
		t.Errorf("スレッドの件数 = %d, want %d", got, 1)
	}
	if got := countPosts(t, f.db); got != 1 {
		t.Errorf("投稿の件数 = %d, want %d", got, 1)
	}
}

// TestCreateThreadUsecase_Execute_RollsBackOnFailure verifies that a failure
// after the thread and its post are written leaves neither behind, so a failed
// submission cannot leave an empty thread on the board or start the author's
// interval.
//
// The failure is provoked by a trigger that refuses to update a thread, because
// the UseCase holds concrete repositories that a test cannot swap for failing
// ones. The refusal lands on the last write of the transaction, which is the
// point where something is already written to roll back.
//
// [Ja] TestCreateThreadUsecase_Execute_RollsBackOnFailure は、スレッドとその投稿を書いた
// 後の失敗がどちらも残さないことを検証します。失敗した送信が、掲示板に空のスレッドを残したり、
// 書き手の間隔を始めたりできないようにするためです。
//
// 失敗を起こすのにスレッドの更新を拒否するトリガーを使うのは、UseCase が具象のリポジトリを
// 保持していて、テストが失敗するものへ差し替えられないためです。拒否が当たるのは
// トランザクションの最後の書き込みであり、そこはロールバックすべきものが既に書かれている
// 地点です。
func TestCreateThreadUsecase_Execute_RollsBackOnFailure(t *testing.T) {
	t.Parallel()

	f, ctx := newCreateThreadUsecase(t)

	_, err := f.db.Writer.ExecContext(ctx, `
		CREATE TRIGGER refuse_thread_update BEFORE UPDATE ON threads
		BEGIN
			SELECT RAISE(ABORT, 'テストによるスレッド更新の拒否');
		END;
	`)
	if err != nil {
		t.Fatalf("テスト用トリガーの作成に失敗: %v", err)
	}

	out, err := f.uc.Execute(ctx, validCreateThreadInput(f))
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

	if got := countThreads(t, f.db); got != 0 {
		t.Errorf("スレッドの件数 = %d, want %d", got, 0)
	}
	if got := countPosts(t, f.db); got != 0 {
		t.Errorf("投稿の件数 = %d, want %d", got, 0)
	}
}
