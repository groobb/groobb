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

// createThreadFixtureは、テスト対象のUseCaseと、スレッドを立てる先の掲示板、
// それを立てるアカウント、そして検証が保存された行を読み戻すためのリポジトリです。
type createThreadFixture struct {
	uc         *usecase.CreateThreadUsecase
	db         *database.DB
	board      *model.Board
	author     model.UserID
	threadRepo *repository.ThreadRepository
	postRepo   *repository.PostRepository
}

// newCreateThreadUsecaseは、"jazz" 掲示板が1つあってまだ何も無く、そこに書く
// アカウントが1つあるデータベース上にUseCaseを構築します。
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

// validCreateThreadInputは検証を通る送信であり、フォームそのものではなく、整った
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

// countThreadsとcountPostsは、データベース全体が持つ行数を読みます。拒否された
// 送信が特定の行を欠いていることではなく、1行も残していないことをテストが述べられる
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

// TestCreateThreadUsecase_Execute_Successは、妥当な送信が、選ばれたタイトルと主言語を
// 持つスレッド、1番が付いて作者に帰属する最初の投稿、そしてスレッドが持つその投稿の姿を
// 残すことを検証します。
func TestCreateThreadUsecase_Execute_Success(t *testing.T) {
	t.Parallel()

	f, ctx := newCreateThreadUsecase(t)

	input := validCreateThreadInput(f)
	input.Title = "  枯葉の名演  "
	input.Language = string(model.LocaleEn.ThreadLanguage())

	out, err := f.uc.Execute(ctx, input)
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	if out == nil {
		t.Fatal("Execute()のoutput = nil")
	}
	if out.ThreadID == 0 {
		t.Error("out.ThreadIDはDB採番で空でないはず")
	}
	if out.Number != 1 {
		t.Errorf("out.Number = %d、期待値 = %d", out.Number, 1)
	}

	thread, err := f.threadRepo.FindByID(ctx, out.ThreadID)
	if err != nil {
		t.Fatalf("FindByID()のエラー = %v", err)
	}
	if thread == nil {
		t.Fatal("作成したスレッドをidで引けない")
	}
	if thread.BoardID != f.board.ID {
		t.Errorf("thread.BoardID = %v、期待値 = %v", thread.BoardID, f.board.ID)
	}
	if thread.UserID == nil || *thread.UserID != f.author {
		t.Errorf("thread.UserID = %v、期待値 = %v", thread.UserID, f.author)
	}
	// タイトルはvalidatorが正規化した形で保存される。長さを測った対象そのもので
	// ある。
	if thread.Title != "枯葉の名演" {
		t.Errorf("thread.Title = %q、期待値 = %q", thread.Title, "枯葉の名演")
	}
	if thread.Language != model.LocaleEn.ThreadLanguage() {
		t.Errorf("thread.Language = %q、期待値 = %q", thread.Language, model.LocaleEn.ThreadLanguage())
	}

	posts, err := f.postRepo.ListByThreadID(ctx, thread.ID)
	if err != nil {
		t.Fatalf("ListByThreadID()のエラー = %v", err)
	}
	if len(posts) != 1 {
		t.Fatalf("len(posts) = %d、期待値 = %d", len(posts), 1)
	}
	post := posts[0]
	if post.Number != 1 {
		t.Errorf("post.Number = %d、期待値 = %d", post.Number, 1)
	}
	if post.Body != "好きな演奏は?" {
		t.Errorf("post.Body = %q、期待値 = %q", post.Body, "好きな演奏は?")
	}
	if post.UserID == nil || *post.UserID != f.author {
		t.Errorf("post.UserID = %v、期待値 = %v", post.UserID, f.author)
	}

	// スレッドの非正規化された姿は、いま書かれた投稿を表す。掲示板のスレッド一覧が
	// postsを読まずに1行を描けるようにするためである。
	if thread.PostsCount != 1 {
		t.Errorf("thread.PostsCount = %d、期待値 = %d", thread.PostsCount, 1)
	}
	if thread.LastPostID == nil || *thread.LastPostID != post.ID {
		t.Errorf("thread.LastPostID = %v、期待値 = %v", thread.LastPostID, post.ID)
	}
	if !thread.LastPostedAt.Equal(post.CreatedAt) {
		t.Errorf("thread.LastPostedAt = %v、期待値 = %v", thread.LastPostedAt, post.CreatedAt)
	}
}

// TestCreateThreadUsecase_Execute_InvalidInputは、直すところのあるフォームが、
// フィールドを名指す *model.ValidationErrorとして戻り、そのために何も書かれないことを
// 検証します。
func TestCreateThreadUsecase_Execute_InvalidInput(t *testing.T) {
	t.Parallel()

	f, ctx := newCreateThreadUsecase(t)

	input := validCreateThreadInput(f)
	input.Title = "   "
	input.Body = ""

	out, err := f.uc.Execute(ctx, input)
	if out != nil {
		t.Error("Execute()のoutputはnilのはず")
	}
	ve := model.AsValidationError(err)
	if ve == nil {
		t.Fatalf("Execute()のエラー = %v、期待値 = *model.ValidationError", err)
	}
	for _, field := range []string{"title", "body"} {
		if !ve.HasFieldError(field) {
			t.Errorf("veに %q のエラーが無い", field)
		}
	}

	if got := countThreads(t, f.db); got != 0 {
		t.Errorf("スレッドの件数 = %d、期待値 = %d", got, 0)
	}
	if got := countPosts(t, f.db); got != 0 {
		t.Errorf("投稿の件数 = %d、期待値 = %d", got, 0)
	}
}

// TestCreateThreadUsecase_Execute_UnknownBoardは、どの掲示板も指さないslugが
// リソース未存在として報告されることを検証します。これにより、どこにも並ばないスレッドを
// 作る代わりに、ハンドラーは404を返せます。
func TestCreateThreadUsecase_Execute_UnknownBoard(t *testing.T) {
	t.Parallel()

	f, ctx := newCreateThreadUsecase(t)

	input := validCreateThreadInput(f)
	input.BoardSlug = "unknown"

	out, err := f.uc.Execute(ctx, input)
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
	if got := countThreads(t, f.db); got != 0 {
		t.Errorf("スレッドの件数 = %d、期待値 = %d", got, 0)
	}
}

// TestCreateThreadUsecase_Execute_WithdrawnUserは、退会したアカウントがスレッドを
// 立てられないことを検証します。そのセッションが、まだ存在していたときに発行されたもので
// あってもです。
func TestCreateThreadUsecase_Execute_WithdrawnUser(t *testing.T) {
	t.Parallel()

	f, ctx := newCreateThreadUsecase(t)

	input := validCreateThreadInput(f)
	input.UserID = testutil.NewUserBuilder(t, f.db).WithDeletedAt(time.Now().Add(-24 * time.Hour)).Build()

	out, err := f.uc.Execute(ctx, input)
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
	if got := countThreads(t, f.db); got != 0 {
		t.Errorf("スレッドの件数 = %d、期待値 = %d", got, 0)
	}
}

// TestCreateThreadUsecase_Execute_WithinPostIntervalは、1つ目のすぐ後に立てた
// 2つ目のスレッドが、拒否の理由である待ち時間とともに拒否され、その拒否が何も書かないことを
// 検証します。
func TestCreateThreadUsecase_Execute_WithinPostInterval(t *testing.T) {
	t.Parallel()

	f, ctx := newCreateThreadUsecase(t)

	if _, err := f.uc.Execute(ctx, validCreateThreadInput(f)); err != nil {
		t.Fatalf("1つ目のExecute()のエラー = %v", err)
	}

	out, err := f.uc.Execute(ctx, validCreateThreadInput(f))
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
	// 待ち時間は、ハンドラーが数を読み取り直す必要のあるテキストではなく、そのまま
	// 使える時間として運ばれる。
	if ae.RetryAfter <= 0 || ae.RetryAfter > model.PostInterval {
		t.Errorf("ae.RetryAfter = %s、期待値 = (0, %s]", ae.RetryAfter, model.PostInterval)
	}

	if got := countThreads(t, f.db); got != 1 {
		t.Errorf("スレッドの件数 = %d、期待値 = %d", got, 1)
	}
	if got := countPosts(t, f.db); got != 1 {
		t.Errorf("投稿の件数 = %d、期待値 = %d", got, 1)
	}
}

// TestCreateThreadUsecase_Execute_AfterPostIntervalは、間隔が過ぎると既存の投稿が
// 投稿者の次の作成を妨げなくなることを確認します。保存済み投稿の時刻を過去へ移すことで、
// 実時間を待たずにデータベースからの取得を含めて検証します。
func TestCreateThreadUsecase_Execute_AfterPostInterval(t *testing.T) {
	t.Parallel()

	f, ctx := newCreateThreadUsecase(t)
	input := validCreateThreadInput(f)

	first, err := f.uc.Execute(ctx, input)
	if err != nil {
		t.Fatalf("1つ目のExecute()のエラー = %v", err)
	}
	if first == nil {
		t.Fatal("1つ目のExecute()のoutput = nil")
	}

	firstPosts, err := f.postRepo.ListByThreadID(ctx, first.ThreadID)
	if err != nil {
		t.Fatalf("ListByThreadID()のエラー = %v", err)
	}
	if len(firstPosts) != 1 {
		t.Fatalf("len(firstPosts) = %d、期待値 = 1", len(firstPosts))
	}
	testutil.BackdatePost(t, f.db, firstPosts[0].ID, time.Now().Add(-time.Hour))

	out, err := f.uc.Execute(ctx, input)
	if err != nil {
		t.Fatalf("間隔経過後のExecute()のエラー = %v", err)
	}
	if out == nil {
		t.Fatal("間隔経過後のExecute()のoutput = nil")
	}
	if out.ThreadID == first.ThreadID {
		t.Error("2つ目のスレッドに新しいIDが発行されていない")
	}
	if out.Number != 1 {
		t.Errorf("out.Number = %d、期待値 = 1", out.Number)
	}
	if got := countThreads(t, f.db); got != 2 {
		t.Errorf("スレッドの件数 = %d、期待値 = 2", got)
	}
	if got := countPosts(t, f.db); got != 2 {
		t.Errorf("投稿の件数 = %d、期待値 = 2", got)
	}

	thread, err := f.threadRepo.FindByID(ctx, out.ThreadID)
	if err != nil {
		t.Fatalf("FindByID()のエラー = %v", err)
	}
	if thread == nil {
		t.Fatal("2つ目のスレッドをidで引けない")
	}
	posts, err := f.postRepo.ListByThreadID(ctx, thread.ID)
	if err != nil {
		t.Fatalf("ListByThreadID()のエラー = %v", err)
	}
	if len(posts) != 1 {
		t.Fatalf("len(posts) = %d、期待値 = 1", len(posts))
	}
	post := posts[0]
	if post.Number != 1 {
		t.Errorf("post.Number = %d、期待値 = 1", post.Number)
	}
	if post.UserID == nil || *post.UserID != f.author {
		t.Errorf("post.UserID = %v、期待値 = %v", post.UserID, f.author)
	}
	if thread.PostsCount != 1 {
		t.Errorf("thread.PostsCount = %d、期待値 = 1", thread.PostsCount)
	}
	if thread.LastPostID == nil || *thread.LastPostID != post.ID {
		t.Errorf("thread.LastPostID = %v、期待値 = %v", thread.LastPostID, post.ID)
	}
	if !thread.LastPostedAt.Equal(post.CreatedAt) {
		t.Errorf("thread.LastPostedAt = %v、期待値 = %v", thread.LastPostedAt, post.CreatedAt)
	}
}

// TestCreateThreadUsecase_Execute_ConcurrentByTheSameUserは、1つのアカウントから
// 同時に送られた2つの送信が、1つのスレッドと1つの拒否になることを検証します。間隔は
// 書き込みロックの下で読むため、2つ目の送信が見るのは、1つ目がコミットした投稿であって、
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
			t.Errorf("Execute()のエラー = %v、期待値 = *model.AppError (%d)", err, model.AppErrCodeRateLimited)
		}
	}
	if succeeded != 1 {
		t.Errorf("成功した送信 = %d 件、期待値 = %d 件", succeeded, 1)
	}

	if got := countThreads(t, f.db); got != 1 {
		t.Errorf("スレッドの件数 = %d、期待値 = %d", got, 1)
	}
	if got := countPosts(t, f.db); got != 1 {
		t.Errorf("投稿の件数 = %d、期待値 = %d", got, 1)
	}
}

// TestCreateThreadUsecase_Execute_RollsBackOnFailureは、スレッドとその投稿を書いた
// 後の失敗がどちらも残さないことを検証します。失敗した送信が、掲示板に空のスレッドを残したり、
// 書き手の間隔を始めたりできないようにするためです。
//
// 失敗を起こすのにスレッドの更新を拒否するトリガーを使うのは、UseCaseが具象のリポジトリを
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

	if got := countThreads(t, f.db); got != 0 {
		t.Errorf("スレッドの件数 = %d、期待値 = %d", got, 0)
	}
	if got := countPosts(t, f.db); got != 0 {
		t.Errorf("投稿の件数 = %d、期待値 = %d", got, 0)
	}
}
