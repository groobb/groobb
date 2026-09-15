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

// firstPostNumberは、スレッドが始まるときの投稿が持つレス番号です。番号付けを
// スレッドから読まずここから始めるのは、スレッドがこの投稿と同時に作られ、それ以前には
// 1件も持たないためです。
const firstPostNumber = 1

// CreateThreadUsecaseはスレッドを立てる処理を統括します。送信されたタイトル・
// 主言語・本文を検証し、スレッド・その最初の投稿・スレッドが持つその投稿の姿を1つの
// トランザクションで書き込みます。
//
// 3つをまとめて書くのは、最初の投稿を持たないスレッドがアプリケーションの取りうる状態では
// ないためです。掲示板の一覧は件数と最終投稿時刻から1行を描き、スレッドのページは会話を
// 見せます。途中のどこで失敗しても、いずれも残しません。
type CreateThreadUsecase struct {
	writer          *sql.DB
	threadValidator *validator.ThreadCreateValidator
	boardRepo       *repository.BoardRepository
	threadRepo      *repository.ThreadRepository
	postRepo        *repository.PostRepository
	userRepo        *repository.UserRepository
}

// NewCreateThreadUsecaseは書き込み用プール・validator・読み書きに使うリポジトリから
// CreateThreadUsecaseを構築します。
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

// CreateThreadInputはExecuteの入力です。BoardSlugはフォームを開いた /b/{slug}、
// UserIDはセッションが名指すアカウント、Title / Language / Bodyは送信されたフィールド
// です。Languageが送信された生の値なのは、selectが運ぶ値が言語を名指すものだとまだ
// 分かっていないためです。
type CreateThreadInput struct {
	BoardSlug string
	UserID    model.UserID
	Title     string
	Language  string
	Body      string
}

// CreateThreadOutputは作られたものの在り処です。スレッドのidと、その最初の投稿が
// 保存されたときのレス番号であり、この2つが、呼び出し元が書き手を送る
// /t/{id}#p{number} になります。
//
// 保存された行そのものは返しません。挿入から読み戻したスレッドは、投稿が数えられる前の
// 行を表すため、それを渡せば、1件の投稿を持つスレッドについて1件も持たないと述べる
// ことになります。
type CreateThreadOutput struct {
	ThreadID model.ThreadID
	Number   int
}

// Executeはフォームを検証し、通ればスレッドを立てます。
//
// 検証をトランザクションの前で行うのは、直すところのあるフォームが書き込みロックを
// 消費しないようにするためです。最大1万コードポイントの本文の計測と正規化は、後ろで
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

// createThreadはスレッド・その最初の投稿・スレッドが持つその投稿の姿を書き込みます。
// その前に、同じトランザクションの中で、掲示板が存在することと、書き手が今投稿してよいことを
// 確認します。
//
// この確認を、書き込みUseCaseが普段読むトランザクションの前ではなくここで読むのは、
// 1人の投稿の間隔が、別の書き手がまさにコミットしようとしている行によって決まるためです。
// _txlock=immediateにより、この読み取りはこの書き手がロックを保持した後に起こります。
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

	// スレッドが作られたときの投稿は、その最新の投稿でもあり、数える唯一の投稿でもある。
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
