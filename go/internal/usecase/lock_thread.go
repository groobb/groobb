package usecase

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/validator"
)

// LockThreadUsecase closes a thread to new replies. Reading is not affected: a
// locked thread is still read, quoted and linked to, and the posts it already
// holds stay where they are.
//
// Locking says something about this thread rather than about the people writing
// to it, so it holds for everyone at once, the administrator who locked it
// included.
//
// [Ja] LockThreadUsecaseはスレッドへの新しい返信を締め切ります。閲覧には影響しません。
// ロック中のスレッドも読まれ、引用され、リンクされ、既にある投稿もそのままです。
//
// ロックが述べるのは、そこへ書き込む人々についてではなくこのスレッドについてであるため、
// ロックした管理者自身を含め、全員に対して同時に成立します。
type LockThreadUsecase struct {
	writer            *sql.DB
	reasonValidator   *validator.ModerationLogCreateValidator
	roleRepo          *repository.RoleRepository
	threadRepo        *repository.ThreadRepository
	moderationLogRepo *repository.ModerationLogRepository
}

// NewLockThreadUsecase builds a LockThreadUsecase from the write pool, the
// validator, and the repositories it reads and persists through.
//
// [Ja] NewLockThreadUsecaseは書き込み用プール・validator・読み書きに使うリポジトリから
// LockThreadUsecaseを構築します。
func NewLockThreadUsecase(
	writer *sql.DB,
	reasonValidator *validator.ModerationLogCreateValidator,
	roleRepo *repository.RoleRepository,
	threadRepo *repository.ThreadRepository,
	moderationLogRepo *repository.ModerationLogRepository,
) *LockThreadUsecase {
	return &LockThreadUsecase{
		writer:            writer,
		reasonValidator:   reasonValidator,
		roleRepo:          roleRepo,
		threadRepo:        threadRepo,
		moderationLogRepo: moderationLogRepo,
	}
}

// LockThreadInput is the input to Execute. Actor is who is locking the thread,
// ThreadID the /t/{id} being locked, and Reason the note the history keeps,
// which may be empty.
//
// [Ja] LockThreadInputはExecuteの入力です。Actorはスレッドをロックする側、ThreadIDは
// ロックされる/t/{id}、Reasonは履歴が保つ注記で、空でも構いません。
type LockThreadInput struct {
	Actor    Actor
	ThreadID model.ThreadID
	Reason   string
}

// Execute locks the thread.
//
// Permission is answered before the reason is examined, so an actor who may not
// lock threads hears that rather than being asked to shorten a note that would
// be refused either way.
//
// [Ja] Executeはスレッドをロックします。
//
// 権限を理由の検査より先に答えるのは、スレッドをロックできない操作者に対して、どのみち
// 拒否される注記を短くするよう求めるのではなく、そのことを伝えるためです。
func (uc *LockThreadUsecase) Execute(ctx context.Context, input LockThreadInput) error {
	communityPolicy, err := resolveCommunityPolicy(ctx, uc.roleRepo, input.Actor)
	if err != nil {
		return err
	}
	if !communityPolicy.CanLockThread() {
		return forbiddenThreadModeration(ctx, "スレッドをロックする権限がない", input.ThreadID)
	}

	reason, err := uc.reasonValidator.Validate(ctx, validator.ModerationLogCreateValidatorInput{
		Reason: input.Reason,
	})
	if err != nil {
		return err
	}

	return uc.lock(ctx, input.Actor, input.ThreadID, reason)
}

// lock marks the thread locked and records the operation, having first confirmed
// inside the same transaction that the thread is there, that it is still
// published, and that no administrator has locked it already.
//
// A thread that is already locked is success without a second entry: what the
// request asked for is that the thread take no further post, and it takes none.
// Recording it again would make the history say a thread was closed twice.
//
// [Ja] lockはスレッドにロックの印を付け、操作を記録します。その前に、同じトランザクション
// の中で、スレッドが存在すること、まだ公開されていること、そして管理者が既にロックしては
// いないことを確かめます。
//
// 既にロック中のスレッドは、記録を増やさずに成功とします。要求が求めたのはスレッドが
// それ以上の投稿を受け付けないことであり、実際に受け付けないためです。もう一度記録すれば、
// 履歴は1つのスレッドが2度閉じられたと述べることになります。
func (uc *LockThreadUsecase) lock(ctx context.Context, actor Actor, threadID model.ThreadID, reason string) error {
	tx, err := uc.writer.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("トランザクションの開始に失敗: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	threadRepo := uc.threadRepo.WithTx(tx)

	thread, err := findModeratedThread(ctx, threadRepo, threadID)
	if err != nil {
		return err
	}
	if thread.LockedAt != nil {
		return nil
	}

	if err := threadRepo.Lock(ctx, thread.ID); err != nil {
		return fmt.Errorf("スレッドのロックに失敗: %w", err)
	}

	if err := recordModerationLog(ctx, uc.moderationLogRepo.WithTx(tx), actor, moderationLogEntry{
		Action:   model.ModerationActionThreadLock,
		ThreadID: &thread.ID,
		Reason:   reason,
	}); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("トランザクションのコミットに失敗: %w", err)
	}
	return nil
}
