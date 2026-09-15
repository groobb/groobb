package usecase

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// UnlockThreadUsecaseは、管理者がスレッドに掛けたロックを外します。
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

// NewUnlockThreadUsecaseは書き込み用プールと、読み書きに使うリポジトリから
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

// UnlockThreadInputはExecuteの入力です。Actorはロックを外す側、ThreadIDはそれが
// 外される/t/{id}です。
type UnlockThreadInput struct {
	Actor    Actor
	ThreadID model.ThreadID
}

// Executeはスレッドからロックを外します。
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

// unlockは印を外して操作を記録します。その前に、同じトランザクションの中で、
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
