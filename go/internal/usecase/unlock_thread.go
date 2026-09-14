package usecase

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// UnlockThreadUsecase lifts the lock an administrator placed on a thread.
//
// Whether the thread then takes replies is a separate question: a thread that
// reached its post cap stays locked for that reason, which is derived from the
// count it carries rather than from a column anyone can clear.
//
// It takes no reason. Lifting a lock restores what the thread was before, so
// there is nothing for the history to explain beyond the operation itself,
// whereas closing a thread is the decision a later administrator has to read.
//
// [Ja] UnlockThreadUsecaseは、管理者がスレッドに掛けたロックを外します。
//
// そのスレッドが実際に返信を受け付けるかどうかは別の問いです。投稿数の上限に達した
// スレッドはその理由でロックされたままであり、それは誰かが空にできる列からではなく、
// スレッドが持つ件数から導かれます。
//
// 理由は受け取りません。ロックを外すことはスレッドを元の姿に戻すことであり、操作そのもの
// の他に履歴が説明することはありません。後の管理者が読む必要のある判断は、スレッドを
// 閉じたほうです。
type UnlockThreadUsecase struct {
	writer            *sql.DB
	roleRepo          *repository.RoleRepository
	threadRepo        *repository.ThreadRepository
	moderationLogRepo *repository.ModerationLogRepository
}

// NewUnlockThreadUsecase builds an UnlockThreadUsecase from the write pool and
// the repositories it reads and persists through.
//
// [Ja] NewUnlockThreadUsecaseは書き込み用プールと、読み書きに使うリポジトリから
// UnlockThreadUsecaseを構築します。
func NewUnlockThreadUsecase(
	writer *sql.DB,
	roleRepo *repository.RoleRepository,
	threadRepo *repository.ThreadRepository,
	moderationLogRepo *repository.ModerationLogRepository,
) *UnlockThreadUsecase {
	return &UnlockThreadUsecase{
		writer:            writer,
		roleRepo:          roleRepo,
		threadRepo:        threadRepo,
		moderationLogRepo: moderationLogRepo,
	}
}

// UnlockThreadInput is the input to Execute. Actor is who is lifting the lock
// and ThreadID the /t/{id} it is lifted from.
//
// [Ja] UnlockThreadInputはExecuteの入力です。Actorはロックを外す側、ThreadIDはそれが
// 外される/t/{id}です。
type UnlockThreadInput struct {
	Actor    Actor
	ThreadID model.ThreadID
}

// Execute lifts the lock from the thread.
//
// [Ja] Executeはスレッドからロックを外します。
func (uc *UnlockThreadUsecase) Execute(ctx context.Context, input UnlockThreadInput) error {
	communityPolicy, err := resolveCommunityPolicy(ctx, uc.roleRepo, input.Actor)
	if err != nil {
		return err
	}
	if !communityPolicy.CanUnlockThread() {
		return forbiddenThreadModeration(ctx, "スレッドのロックを解除する権限がない", input.ThreadID)
	}

	return uc.unlock(ctx, input.Actor, input.ThreadID)
}

// unlock takes the mark off and records the operation, having first confirmed
// inside the same transaction that the thread is there, that it is still
// published, and that an administrator's lock is what it carries.
//
// A thread no administrator locked is success without an entry, for the reason a
// second lock writes none: the request asked that the administrators' lock not
// hold, and it does not. A thread locked only by its post cap is one such thread,
// so lifting nothing is not recorded as having reopened it.
//
// [Ja] unlockは印を外して操作を記録します。その前に、同じトランザクションの中で、
// スレッドが存在すること、まだ公開されていること、そして管理者のロックを持っていることを
// 確かめます。
//
// どの管理者もロックしていないスレッドは、2度目のロックが記録を書かないのと同じ理由で、
// 記録を増やさずに成功とします。要求が求めたのは管理者のロックが成立していないことであり、
// 実際に成立していないためです。投稿数の上限だけでロックされたスレッドもそのようなスレッド
// であり、何も外さなかった操作が、それを開き直したものとして記録されることはありません。
func (uc *UnlockThreadUsecase) unlock(ctx context.Context, actor Actor, threadID model.ThreadID) error {
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
	if thread.LockedAt == nil {
		return nil
	}

	if err := threadRepo.Unlock(ctx, thread.ID); err != nil {
		return fmt.Errorf("スレッドのロックの解除に失敗: %w", err)
	}

	if err := recordModerationLog(ctx, uc.moderationLogRepo.WithTx(tx), actor, moderationLogEntry{
		Action:   model.ModerationActionThreadUnlock,
		ThreadID: &thread.ID,
	}); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("トランザクションのコミットに失敗: %w", err)
	}
	return nil
}
