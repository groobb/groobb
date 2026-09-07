package usecase

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/validator"
)

// firstPostNumber is the reply number the post a thread starts with carries.
// The numbering starts here rather than being read from the thread, because a
// thread is created together with this post and holds none before it.
//
// [Ja] firstPostNumber は、スレッドが始まるときの投稿が持つレス番号です。番号付けを
// スレッドから読まずここから始めるのは、スレッドがこの投稿と同時に作られ、それ以前には
// 1 件も持たないためです。
const firstPostNumber = 1

// CreateThreadUsecase orchestrates starting a thread: it validates the submitted
// title, primary language and body, then writes the thread, its first post and
// the thread's view of that post in one transaction.
//
// The three are written together because a thread without its first post is not
// a state the application has: the board's list draws a row from the count and
// the last-post time, and the thread page shows a conversation. A failure
// anywhere in the sequence leaves none of them behind.
//
// [Ja] CreateThreadUsecase はスレッドを立てる処理を統括します。送信されたタイトル・
// 主言語・本文を検証し、スレッド・その最初の投稿・スレッドが持つその投稿の姿を 1 つの
// トランザクションで書き込みます。
//
// 3 つをまとめて書くのは、最初の投稿を持たないスレッドがアプリケーションの取りうる状態では
// ないためです。掲示板の一覧は件数と最終投稿時刻から 1 行を描き、スレッドのページは会話を
// 見せます。途中のどこで失敗しても、いずれも残しません。
type CreateThreadUsecase struct {
	writer          *sql.DB
	threadValidator *validator.ThreadCreateValidator
	boardRepo       *repository.BoardRepository
	threadRepo      *repository.ThreadRepository
	postRepo        *repository.PostRepository
	userRepo        *repository.UserRepository
}

// NewCreateThreadUsecase builds a CreateThreadUsecase from the write pool, the
// validator, and the repositories it reads and persists through.
//
// [Ja] NewCreateThreadUsecase は書き込み用プール・validator・読み書きに使うリポジトリから
// CreateThreadUsecase を構築します。
func NewCreateThreadUsecase(
	writer *sql.DB,
	threadValidator *validator.ThreadCreateValidator,
	boardRepo *repository.BoardRepository,
	threadRepo *repository.ThreadRepository,
	postRepo *repository.PostRepository,
	userRepo *repository.UserRepository,
) *CreateThreadUsecase {
	return &CreateThreadUsecase{
		writer:          writer,
		threadValidator: threadValidator,
		boardRepo:       boardRepo,
		threadRepo:      threadRepo,
		postRepo:        postRepo,
		userRepo:        userRepo,
	}
}

// CreateThreadInput is the input to Execute. BoardSlug is the /b/{slug} the form
// was opened from, UserID the account the session names, and Title / Language /
// Body the submitted fields. Language arrives as the raw submitted value because
// what a select carries is not yet known to name a language.
//
// [Ja] CreateThreadInput は Execute の入力です。BoardSlug はフォームを開いた /b/{slug}、
// UserID はセッションが名指すアカウント、Title / Language / Body は送信されたフィールド
// です。Language が送信された生の値なのは、select が運ぶ値が言語を名指すものだとまだ
// 分かっていないためです。
type CreateThreadInput struct {
	BoardSlug string
	UserID    model.UserID
	Title     string
	Language  string
	Body      string
}

// CreateThreadOutput is the address of what was created: the thread's id and the
// reply number its first post was saved with, which together make the /t/{id}#p{number}
// the caller sends the author to.
//
// The saved rows are not returned. The thread read back from its insert describes
// the row before its posts were counted, so handing it over would say a thread
// with one post holds none.
//
// [Ja] CreateThreadOutput は作られたものの在り処です。スレッドの id と、その最初の投稿が
// 保存されたときのレス番号であり、この 2 つが、呼び出し元が書き手を送る
// /t/{id}#p{number} になります。
//
// 保存された行そのものは返しません。挿入から読み戻したスレッドは、投稿が数えられる前の
// 行を表すため、それを渡せば、1 件の投稿を持つスレッドについて 1 件も持たないと述べる
// ことになります。
type CreateThreadOutput struct {
	ThreadID model.ThreadID
	Number   int
}

// Execute validates the form and, if it holds, starts the thread.
//
// Validation runs before the transaction so that a form with something to fix
// costs no write lock: a body of up to ten thousand code points is measured and
// normalized while nobody is waiting behind it.
//
// [Ja] Execute はフォームを検証し、通ればスレッドを立てます。
//
// 検証をトランザクションの前で行うのは、直すところのあるフォームが書き込みロックを
// 消費しないようにするためです。最大 1 万コードポイントの本文の計測と正規化は、後ろで
// 誰も待っていない間に済みます。
func (uc *CreateThreadUsecase) Execute(ctx context.Context, input CreateThreadInput) (*CreateThreadOutput, error) {
	validated, err := uc.threadValidator.Validate(ctx, validator.ThreadCreateValidatorInput{
		Title:    input.Title,
		Language: input.Language,
		Body:     input.Body,
	})
	if err != nil {
		return nil, err
	}

	return uc.createThread(ctx, input.BoardSlug, input.UserID, validated)
}

// createThread writes the thread, its first post and the thread's view of that
// post, having first confirmed inside the same transaction that the board is
// there and that the author may post now.
//
// Those confirmations are read here rather than before the transaction, which is
// where a write UseCase otherwise reads: the interval between one person's posts
// is decided by rows another writer may be about to commit, and _txlock=immediate
// means the read happens once this writer holds the lock. What the transaction
// does not hold is the work that does not depend on it — the form is validated
// before it opens.
//
// [Ja] createThread はスレッド・その最初の投稿・スレッドが持つその投稿の姿を書き込みます。
// その前に、同じトランザクションの中で、掲示板が存在することと、書き手が今投稿してよいことを
// 確認します。
//
// この確認を、書き込み UseCase が普段読むトランザクションの前ではなくここで読むのは、
// 1 人の投稿の間隔が、別の書き手がまさにコミットしようとしている行によって決まるためです。
// _txlock=immediate により、この読み取りはこの書き手がロックを保持した後に起こります。
// 一方、トランザクションに依存しない仕事は中に入れません。フォームの検証はそれを開く前に
// 済ませています。
func (uc *CreateThreadUsecase) createThread(
	ctx context.Context,
	boardSlug string,
	userID model.UserID,
	validated *validator.ThreadCreateValidateOutput,
) (*CreateThreadOutput, error) {
	tx, err := uc.writer.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("トランザクションの開始に失敗: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	boardRepo := uc.boardRepo.WithTx(tx)
	threadRepo := uc.threadRepo.WithTx(tx)
	postRepo := uc.postRepo.WithTx(tx)
	userRepo := uc.userRepo.WithTx(tx)

	board, err := boardRepo.FindBySlug(ctx, boardSlug)
	if err != nil {
		return nil, fmt.Errorf("掲示板の取得に失敗: %w", err)
	}
	if board == nil {
		return nil, &model.AppError{
			Code:     model.AppErrCodeResourceNotFound,
			UserMsg:  i18n.T(ctx, "error_not_found_message"),
			Internal: fmt.Errorf("掲示板が見つからない: slug=%s", boardSlug),
			Metadata: map[string]string{"board_slug": boardSlug},
		}
	}

	if err := verifyPostAuthor(ctx, userRepo, userID); err != nil {
		return nil, err
	}

	if err := verifyPostInterval(ctx, postRepo, userID, time.Now()); err != nil {
		return nil, err
	}

	thread, err := threadRepo.Create(ctx, repository.CreateThreadInput{
		BoardID:  board.ID,
		UserID:   &userID,
		Title:    validated.Title,
		Language: validated.Language,
	})
	if err != nil {
		return nil, fmt.Errorf("スレッドの作成に失敗: %w", err)
	}

	post, err := postRepo.Create(ctx, repository.CreatePostInput{
		ThreadID: thread.ID,
		UserID:   &userID,
		Number:   firstPostNumber,
		Body:     validated.Body,
	})
	if err != nil {
		return nil, fmt.Errorf("最初の投稿の作成に失敗: %w", err)
	}

	// The post the thread was created with is also its latest, and the only one it
	// counts. Its stored timestamp is what the board's list orders by, so the time
	// written here is the post's own rather than one taken again.
	//
	// No reference is written: a reply number points at a post below the one
	// writing it, and there is nothing below the first.
	//
	// [Ja] スレッドが作られたときの投稿は、その最新の投稿でもあり、数える唯一の投稿でもある。
	// 掲示板の一覧が並べ替えに使うのは保存されたその時刻であるため、ここで書くのは取り直した
	// 時刻ではなく投稿自身の時刻とする。
	//
	// 参照は書かない。レス番号はそれを書いた投稿より下の投稿を指すものであり、最初の投稿の
	// 下には何も無い。
	if err := threadRepo.UpdateLastPost(ctx, thread.ID, repository.UpdateThreadLastPostInput{
		PostsCount:   1,
		LastPostID:   post.ID,
		LastPostedAt: post.CreatedAt,
	}); err != nil {
		return nil, fmt.Errorf("スレッドの最終投稿の更新に失敗: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("トランザクションのコミットに失敗: %w", err)
	}

	return &CreateThreadOutput{ThreadID: thread.ID, Number: post.Number}, nil
}
