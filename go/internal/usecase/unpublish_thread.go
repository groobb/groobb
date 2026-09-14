package usecase

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/validator"
)

// UnpublishThreadUsecase takes a thread out of the community's view. The thread
// keeps its title, its posts and its reply numbers; what changes is that the
// listings no longer carry it and /t/{id} no longer shows it.
//
// It is a mark rather than a deletion. Nothing is removed, so taking the mark
// off would bring the thread back exactly as it stood, and the reply numbers
// this thread issued are never handed to another thread.
//
// [Ja] UnpublishThreadUsecaseはスレッドをコミュニティの視界から外します。スレッドは
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

// NewUnpublishThreadUsecase builds an UnpublishThreadUsecase from the write
// pool, the validator, and the repositories it reads and persists through.
//
// [Ja] NewUnpublishThreadUsecaseは書き込み用プール・validator・読み書きに使うリポジトリ
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

// UnpublishThreadInput is the input to Execute. Actor is who is unpublishing the
// thread, ThreadID the /t/{id} being taken out of view, and Reason the note the
// history keeps, which may be empty.
//
// [Ja] UnpublishThreadInputはExecuteの入力です。Actorはスレッドを非公開にする側、
// ThreadIDは視界から外される/t/{id}、Reasonは履歴が保つ注記で、空でも構いません。
type UnpublishThreadInput struct {
	Actor    Actor
	ThreadID model.ThreadID
	Reason   string
}

// UnpublishThreadOutput names where the thread stood, by the slug of the board
// that listed it. The listing is where the administrator is sent afterwards,
// the thread itself no longer being somewhere to land, and the slug is only
// known here: the request names the thread, and the thread names its board by
// an id that no address is built from.
//
// [Ja] UnpublishThreadOutputは、スレッドが立っていた場所を、それを並べていた掲示板のslugで
// 名指します。その後に管理者が送られる先がこの一覧です。スレッド自身はもう降り立つ場所では
// ないためです。そしてslugを知るのはここだけです。要求が名指すのはスレッドであり、スレッドが
// 自身の掲示板を名指すのは、どのアドレスもそこから組み立てられないidであるためです。
type UnpublishThreadOutput struct {
	BoardSlug string
}

// Execute takes the thread out of view.
//
// Permission is answered before the reason is examined, so an actor who may not
// unpublish threads hears that rather than being asked to shorten a note that
// would be refused either way.
//
// The board is read after the transaction has committed rather than inside it.
// What the transaction reads is what decides whether to write (the thread and
// its state); where the thread stood decides nothing, and reading it there
// would hold the write lock open across a lookup the operation does not depend
// on.
//
// [Ja] Executeはスレッドを視界から外します。
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

// unpublish marks the thread unpublished and records the operation, having first
// confirmed inside the same transaction that the thread is there and is not
// already unpublished. It returns the board the thread was posted in, which is
// what the caller names the listing with.
//
// A thread that is already unpublished is success without a second entry, as a
// second lock is: the request asked that the community not be shown this thread,
// and it is not shown it. This is the one moderation operation whose target may
// itself be unpublished, because it is the operation that puts a thread in that
// state.
//
// [Ja] unpublishはスレッドに非公開の印を付け、操作を記録します。その前に、同じ
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
