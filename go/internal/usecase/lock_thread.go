package usecase

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/validator"
)

// LockThreadUsecaseはスレッドへの新しい返信を締め切ります。閲覧には影響しません。
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

// NewLockThreadUsecaseは書き込み用プール・validator・読み書きに使うリポジトリから
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

// LockThreadInputはExecuteの入力です。Actorはスレッドをロックする側、ThreadIDは
// ロックされる/t/{id}、Reasonは履歴が保つ注記で、空でも構いません。
type LockThreadInput struct {
	Actor    Actor
	ThreadID model.ThreadID
	Reason   string
}

// Executeはスレッドをロックします。
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

// lockはスレッドにロックの印を付け、操作を記録します。その前に、同じトランザクション
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
