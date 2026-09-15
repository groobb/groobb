package usecase

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/validator"
)

// UnpublishThreadUsecaseはスレッドをコミュニティの視界から外します。スレッドは
// タイトルも投稿もレス番号も保ち、変わるのは一覧がそれを運ばなくなり /t/{id} がそれを
// 示さなくなることです。
//
// これは削除ではなく印です。何も取り除かないため、印を外せばスレッドは立っていたとおりに
// 戻り、このスレッドが発行したレス番号が別のスレッドへ渡ることはありません。
type UnpublishThreadUsecase struct {
	writer            *sql.DB
	reasonValidator   *validator.ModerationLogCreateValidator
	roleRepo          *repository.RoleRepository
	threadRepo        *repository.ThreadRepository
	boardRepo         *repository.BoardRepository
	moderationLogRepo *repository.ModerationLogRepository
}

// NewUnpublishThreadUsecaseは書き込み用プール・validator・読み書きに使うリポジトリ
// からUnpublishThreadUsecaseを構築します。
func NewUnpublishThreadUsecase(
	writer *sql.DB,
	reasonValidator *validator.ModerationLogCreateValidator,
	roleRepo *repository.RoleRepository,
	threadRepo *repository.ThreadRepository,
	boardRepo *repository.BoardRepository,
	moderationLogRepo *repository.ModerationLogRepository,
) *UnpublishThreadUsecase {
	return &UnpublishThreadUsecase{
		writer:            writer,
		reasonValidator:   reasonValidator,
		roleRepo:          roleRepo,
		threadRepo:        threadRepo,
		boardRepo:         boardRepo,
		moderationLogRepo: moderationLogRepo,
	}
}

// UnpublishThreadInputはExecuteの入力です。Actorはスレッドを非公開にする側、
// ThreadIDは視界から外される/t/{id}、Reasonは履歴が保つ注記で、空でも構いません。
type UnpublishThreadInput struct {
	Actor    Actor
	ThreadID model.ThreadID
	Reason   string
}

// UnpublishThreadOutputは、スレッドが立っていた場所を、それを並べていた掲示板のslugで
// 名指します。その後に管理者が送られる先がこの一覧です。スレッド自身はもう降り立つ場所では
// ないためです。そしてslugを知るのはここだけです。要求が名指すのはスレッドであり、スレッドが
// 自身の掲示板を名指すのは、どのアドレスもそこから組み立てられないidであるためです。
type UnpublishThreadOutput struct {
	BoardSlug string
}

// Executeはスレッドを視界から外します。
//
// 権限を理由の検査より先に答えるのは、スレッドを非公開にできない操作者に対して、どのみち
// 拒否される注記を短くするよう求めるのではなく、そのことを伝えるためです。
//
// 掲示板を読むのはトランザクションの中ではなく、コミットされた後です。トランザクションが
// 読むのは書き込むかどうかを決めるもの (スレッドとその状態) であり、スレッドがどこに立って
// いたかは何も決めません。そこで読めば、操作が依存しないルックアップの間ずっと書き込みロックを
// 開いたままにすることになります。
func (uc *UnpublishThreadUsecase) Execute(ctx context.Context, input UnpublishThreadInput) (*UnpublishThreadOutput, error) {
	communityPolicy, err := resolveCommunityPolicy(ctx, uc.roleRepo, input.Actor)
	if err != nil {
		return nil, err
	}
	if !communityPolicy.CanUnpublishThread() {
		return nil, forbiddenThreadModeration(ctx, "スレッドを非公開にする権限がない", input.ThreadID)
	}

	reason, err := uc.reasonValidator.Validate(ctx, validator.ModerationLogCreateValidatorInput{
		Reason: input.Reason,
	})
	if err != nil {
		return nil, err
	}

	boardID, err := uc.unpublish(ctx, input.Actor, input.ThreadID, reason)
	if err != nil {
		return nil, err
	}

	board, err := uc.boardRepo.FindByID(ctx, boardID)
	if err != nil {
		return nil, fmt.Errorf("スレッドの掲示板の取得に失敗: %w", err)
	}
	if board == nil {
		return nil, fmt.Errorf("スレッドの掲示板が見つからない: thread_id=%s board_id=%s", input.ThreadID, boardID)
	}

	return &UnpublishThreadOutput{BoardSlug: board.Slug}, nil
}

// unpublishはスレッドに非公開の印を付け、操作を記録します。その前に、同じ
// トランザクションの中で、スレッドが存在すること、そして既に非公開ではないことを
// 確かめます。返すのはスレッドが立っていた掲示板で、呼び出し元が一覧を名指すのにこれを
// 使います。
//
// 既に非公開のスレッドは、2度目のロックと同じく、記録を増やさずに成功とします。要求が
// 求めたのはコミュニティにこのスレッドが示されないことであり、実際に示されていないため
// です。対象自身が非公開であってよいモデレーションの操作はこれだけです。スレッドをその
// 状態に置く操作そのものであるためです。
func (uc *UnpublishThreadUsecase) unpublish(ctx context.Context, actor Actor, threadID model.ThreadID, reason string) (model.BoardID, error) {
	tx, err := uc.writer.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("トランザクションの開始に失敗: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	threadRepo := uc.threadRepo.WithTx(tx)

	thread, err := findExistingThread(ctx, threadRepo, threadID)
	if err != nil {
		return 0, err
	}
	if thread.UnpublishedAt != nil {
		return thread.BoardID, nil
	}

	if err := threadRepo.Unpublish(ctx, thread.ID); err != nil {
		return 0, fmt.Errorf("スレッドの非公開に失敗: %w", err)
	}

	if err := recordModerationLog(ctx, uc.moderationLogRepo.WithTx(tx), actor, moderationLogEntry{
		Action:   model.ModerationActionThreadUnpublish,
		ThreadID: &thread.ID,
		Reason:   reason,
	}); err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("トランザクションのコミットに失敗: %w", err)
	}
	return thread.BoardID, nil
}
