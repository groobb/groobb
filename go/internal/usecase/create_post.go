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

// CreatePostUsecase orchestrates replying to a thread: it validates the
// submitted body, then writes the post, the thread's view of it and what the
// body refers to in one transaction.
//
// The three are written together because a reply that is counted but not stored,
// or stored without being counted, is not a state the application has: the reply
// number a post is addressed by comes from the thread's count, so a count that
// disagrees with the posts would hand out a number another post already holds.
//
// [Ja] CreatePostUsecase はスレッドへの返信を統括します。送信された本文を検証し、投稿・
// スレッドが持つその投稿の姿・本文が参照するものを 1 つのトランザクションで書き込みます。
//
// 3 つをまとめて書くのは、数えられたのに保存されない返信も、保存されたのに数えられない
// 返信も、アプリケーションの取りうる状態ではないためです。投稿を名指すレス番号はスレッドの
// 件数から採るため、件数が投稿と食い違えば、既に別の投稿が持つ番号を配ることになります。
type CreatePostUsecase struct {
	writer            *sql.DB
	postValidator     *validator.PostCreateValidator
	threadRepo        *repository.ThreadRepository
	postRepo          *repository.PostRepository
	postReferenceRepo *repository.PostReferenceRepository
	userRepo          *repository.UserRepository
}

// NewCreatePostUsecase builds a CreatePostUsecase from the write pool, the
// validator, and the repositories it reads and persists through.
//
// [Ja] NewCreatePostUsecase は書き込み用プール・validator・読み書きに使うリポジトリから
// CreatePostUsecase を構築します。
func NewCreatePostUsecase(
	writer *sql.DB,
	postValidator *validator.PostCreateValidator,
	threadRepo *repository.ThreadRepository,
	postRepo *repository.PostRepository,
	postReferenceRepo *repository.PostReferenceRepository,
	userRepo *repository.UserRepository,
) *CreatePostUsecase {
	return &CreatePostUsecase{
		writer:            writer,
		postValidator:     postValidator,
		threadRepo:        threadRepo,
		postRepo:          postRepo,
		postReferenceRepo: postReferenceRepo,
		userRepo:          userRepo,
	}
}

// CreatePostInput is the input to Execute. ThreadID is the /t/{id} the reply was
// written under, UserID the account the session names, and Body the submitted
// field. No language accompanies it: a language belongs to the thread, and a
// reply written in another one is taken.
//
// [Ja] CreatePostInput は Execute の入力です。ThreadID は返信が書かれた /t/{id}、UserID は
// セッションが名指すアカウント、Body は送信されたフィールドです。言語は伴いません。言語は
// スレッドに属するもので、別の言語で書かれた返信も受け付けます。
type CreatePostInput struct {
	ThreadID model.ThreadID
	UserID   model.UserID
	Body     string
}

// CreatePostOutput is the reply number the post was saved with, which the caller
// appends to the thread it already knows to reach /t/{id}#p{number}.
//
// The thread's id is not returned with it, unlike when a thread is started: the
// caller named the thread in the request, so handing it back would say only what
// it was already told.
//
// [Ja] CreatePostOutput は投稿が保存されたときのレス番号です。呼び出し元は、既に知って
// いるスレッドにこれを繋いで /t/{id}#p{number} に至ります。
//
// スレッドを立てるときと違い、スレッドの id を添えません。呼び出し元はリクエストの中で
// スレッドを名指しているため、返しても、伝えられたことをそのまま述べるだけになります。
type CreatePostOutput struct {
	Number int
}

// Execute validates the form and, if it holds, saves the reply.
//
// Validation and the reading of the body run before the transaction, so a form
// with something to fix costs no write lock: measuring up to ten thousand code
// points and scanning them for >>N happens while nobody is waiting behind it.
//
// [Ja] Execute はフォームを検証し、通れば返信を保存します。
//
// 検証と本文の読み取りをトランザクションの前で行うのは、直すところのあるフォームが書き込み
// ロックを消費しないようにするためです。最大 1 万コードポイントの計測と、そこから >>N を
// 拾う走査は、後ろで誰も待っていない間に済みます。
func (uc *CreatePostUsecase) Execute(ctx context.Context, input CreatePostInput) (*CreatePostOutput, error) {
	body, err := uc.postValidator.Validate(ctx, validator.PostCreateValidatorInput{
		Body: input.Body,
	})
	if err != nil {
		return nil, err
	}

	return uc.createPost(ctx, input.ThreadID, input.UserID, body, model.ReferencedPostNumbers(body))
}

// createPost writes the post, the thread's view of it and the references its
// body makes, having first confirmed inside the same transaction that the thread
// is there, that the account is still one that may write, that the thread still
// takes posts, and that the author has waited out their interval.
//
// Those confirmations are read here rather than before the transaction, which is
// where a write UseCase otherwise reads: the thread's count decides both the
// reply number and whether the cap has been reached, and _txlock=immediate means
// it is read once this writer holds the lock. Read outside it, two replies
// arriving at once would both be numbered from the same count, and the thousandth
// post would let a thousand-and-first through behind it.
//
// [Ja] createPost は投稿・スレッドが持つその投稿の姿・本文が作る参照を書き込みます。その
// 前に、同じトランザクションの中で、スレッドが存在すること、アカウントがまだ書けるもので
// あること、スレッドがまだ投稿を受け付けること、そして書き手が間隔を空け終えたことを確認
// します。
//
// この確認を、書き込み UseCase が普段読むトランザクションの前ではなくここで読むのは、
// スレッドの件数がレス番号と上限到達の両方を決めるためです。_txlock=immediate により、
// その読み取りはこの書き手がロックを保持した後に起こります。その外で読めば、同時に届いた
// 2 つの返信が同じ件数から番号を採り、1000 件目の投稿はその後ろに 1001 件目を通してしまい
// ます。
func (uc *CreatePostUsecase) createPost(
	ctx context.Context,
	threadID model.ThreadID,
	userID model.UserID,
	body string,
	referencedNumbers []int,
) (*CreatePostOutput, error) {
	tx, err := uc.writer.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("トランザクションの開始に失敗: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	threadRepo := uc.threadRepo.WithTx(tx)
	postRepo := uc.postRepo.WithTx(tx)
	postReferenceRepo := uc.postReferenceRepo.WithTx(tx)
	userRepo := uc.userRepo.WithTx(tx)

	thread, err := threadRepo.FindByID(ctx, threadID)
	if err != nil {
		return nil, fmt.Errorf("スレッドの取得に失敗: %w", err)
	}
	if thread == nil {
		return nil, &model.AppError{
			Code:     model.AppErrCodeResourceNotFound,
			UserMsg:  i18n.T(ctx, "error_not_found_message"),
			Internal: fmt.Errorf("スレッドが見つからない: thread_id=%s", threadID),
			Metadata: map[string]string{"thread_id": threadID.String()},
		}
	}

	// Whether this account may write at all is asked before the thread's own
	// refusals. An account that has left can post nowhere, so it hears that
	// rather than hearing about the thread it happened to write to: the guidance
	// a locked thread carries offers the next thread, which this account could
	// not start either.
	//
	// [Ja] このアカウントがそもそも書けるかを、スレッド自身の拒否理由より先に問う。去った
	// アカウントはどこにも投稿できないため、たまたま書き込んだスレッドの話ではなくそのことを
	// 伝える。ロック中のスレッドが運ぶ案内は次のスレッドを差し出すが、このアカウントはそれを
	// 立てることもできない。
	if err := verifyPostAuthor(ctx, userRepo, userID); err != nil {
		return nil, err
	}

	// Whether the thread takes a post is asked before whether this person has
	// waited. A lock holds for everyone and is not waited out, so a reply
	// refused for both reasons is told the one that will still hold in ten
	// seconds.
	//
	// [Ja] スレッドが投稿を受け付けるかを、この人が待ち終えたかより先に問う。ロックは全員に
	// 対して成立し、待っても解けないため、両方の理由で拒否される返信には、10 秒後にもなお
	// 成立しているほうを伝える。
	if reasons := thread.LockReasons(); len(reasons) > 0 {
		return nil, &model.AppError{
			Code:        model.AppErrCodeThreadLocked,
			UserMsg:     i18n.T(ctx, "validation_post_thread_locked"),
			Internal:    fmt.Errorf("ロック中のスレッドへの投稿: thread_id=%s reasons=%v", threadID, reasons),
			Metadata:    map[string]string{"thread_id": threadID.String()},
			LockReasons: reasons,
		}
	}

	if err := verifyPostInterval(ctx, postRepo, userID, time.Now()); err != nil {
		return nil, err
	}

	// The reply number comes from the count the thread carries rather than from
	// the numbers its posts hold. Nothing is deleted, so the two agree, and the
	// count is the value the cap was just read against.
	//
	// [Ja] レス番号は、投稿が持つ番号ではなくスレッドが持つ件数から採る。削除が無いため
	// 両者は一致し、件数はいま上限と突き合わせた値そのものである。
	post, err := postRepo.Create(ctx, repository.CreatePostInput{
		ThreadID: thread.ID,
		UserID:   &userID,
		Number:   thread.PostsCount + 1,
		Body:     body,
	})
	if err != nil {
		return nil, fmt.Errorf("投稿の作成に失敗: %w", err)
	}

	if err := threadRepo.UpdateLastPost(ctx, thread.ID, repository.UpdateThreadLastPostInput{
		PostsCount:   post.Number,
		LastPostID:   post.ID,
		LastPostedAt: post.CreatedAt,
	}); err != nil {
		return nil, fmt.Errorf("スレッドの最終投稿の更新に失敗: %w", err)
	}

	if err := postReferenceRepo.CreateAllByReferencedNumbers(ctx, repository.CreatePostReferencesInput{
		PostID:            post.ID,
		ThreadID:          thread.ID,
		Number:            post.Number,
		ReferencedNumbers: referencedNumbers,
	}); err != nil {
		return nil, fmt.Errorf("投稿の参照の作成に失敗: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("トランザクションのコミットに失敗: %w", err)
	}

	return &CreatePostOutput{Number: post.Number}, nil
}
