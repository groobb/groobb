package usecase

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/validator"
)

// UnpublishPostUsecaseは投稿1件の本文を視界から外し、その周りのスレッドはそのまま
// にします。レス番号はその位置に残ってアンカーを保つため、それを引用した返信は、書かれた
// ときに指していた位置を今も指します。
//
// スレッド自身の集計には触れません。posts_countは表示されている本文の数ではなく発行した
// レス番号の数であるため、上限に達したスレッドは投稿の1つが非公開になっても閉じたまま
// であり、発行した番号が再び発行されることもありません。
type UnpublishPostUsecase struct {
	writer            *sql.DB
	reasonValidator   *validator.ModerationLogCreateValidator
	roleRepo          *repository.RoleRepository
	threadRepo        *repository.ThreadRepository
	postRepo          *repository.PostRepository
	moderationLogRepo *repository.ModerationLogRepository
}

// NewUnpublishPostUsecaseは書き込み用プール・validator・読み書きに使うリポジトリ
// からUnpublishPostUsecaseを構築します。
func NewUnpublishPostUsecase(
	writer *sql.DB,
	reasonValidator *validator.ModerationLogCreateValidator,
	roleRepo *repository.RoleRepository,
	threadRepo *repository.ThreadRepository,
	postRepo *repository.PostRepository,
	moderationLogRepo *repository.ModerationLogRepository,
) *UnpublishPostUsecase {
	return &UnpublishPostUsecase{
		writer:            writer,
		reasonValidator:   reasonValidator,
		roleRepo:          roleRepo,
		threadRepo:        threadRepo,
		postRepo:          postRepo,
		moderationLogRepo: moderationLogRepo,
	}
}

// UnpublishPostInputはExecuteの入力です。Actorは投稿を非公開にする側、ThreadIDと
// Numberは投稿が参照されるあらゆる場所でそれを名指す組、Reasonは履歴が保つ注記で、空でも
// 構いません。
type UnpublishPostInput struct {
	Actor    Actor
	ThreadID model.ThreadID
	Number   int
	Reason   string
}

// Executeは投稿を視界から外します。
//
// 権限を理由の検査より先に答えるのは、投稿を非公開にできない操作者に対して、どのみち
// 拒否される注記を短くするよう求めるのではなく、そのことを伝えるためです。
func (uc *UnpublishPostUsecase) Execute(ctx context.Context, input UnpublishPostInput) error {
	communityPolicy, err := resolveCommunityPolicy(ctx, uc.roleRepo, input.Actor)
	if err != nil {
		return err
	}
	if !communityPolicy.CanUnpublishPost() {
		return forbiddenThreadModeration(ctx, "投稿を非公開にする権限がない", input.ThreadID)
	}

	reason, err := uc.reasonValidator.Validate(ctx, validator.ModerationLogCreateValidatorInput{
		Reason: input.Reason,
	})
	if err != nil {
		return err
	}

	return uc.unpublish(ctx, input.Actor, input.ThreadID, input.Number, reason)
}

// unpublishは投稿に非公開の印を付け、操作を記録します。その前に、同じ
// トランザクションの中で、スレッドがまだそれを示していること、そして管理者が既に視界から
// 外してはいないことを確かめます。
//
// スレッドが非公開の投稿は、操作されるのではなく拒否されます。スレッドが丸ごとスレッド
// 自身の印で視界の外にあり、この操作が取り除くべき本文はコミュニティの中に立っていない
// ためです。既に非公開の投稿は、2度目のロックと同じく、記録を増やさずに成功とします。
func (uc *UnpublishPostUsecase) unpublish(ctx context.Context, actor Actor, threadID model.ThreadID, number int, reason string) error {
	tx, err := uc.writer.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("トランザクションの開始に失敗: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	thread, err := findModeratedThread(ctx, uc.threadRepo.WithTx(tx), threadID)
	if err != nil {
		return err
	}

	postRepo := uc.postRepo.WithTx(tx)
	post, err := findExistingPost(ctx, postRepo, thread.ID, number)
	if err != nil {
		return err
	}
	if post.UnpublishedAt != nil {
		return nil
	}

	if err := postRepo.Unpublish(ctx, post.ID); err != nil {
		return fmt.Errorf("投稿の非公開に失敗: %w", err)
	}

	if err := recordModerationLog(ctx, uc.moderationLogRepo.WithTx(tx), actor, moderationLogEntry{
		Action:   model.ModerationActionPostUnpublish,
		ThreadID: &thread.ID,
		PostID:   &post.ID,
		Reason:   reason,
	}); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("トランザクションのコミットに失敗: %w", err)
	}
	return nil
}
