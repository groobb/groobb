package usecase

import (
	"context"
	"fmt"
	"strconv"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// findExistingThread resolves the thread an operation names, answering a thread
// the community does not have as a missing resource.
//
// A caller that writes must pass a repository enlisted in its write
// transaction (WithTx). What it reads decides whether to write, and a thread
// that goes away in between would otherwise let an operation land on a row
// that is no longer there. A caller that only reads, such as the confirmation
// page resolving the thread it is about, passes the repository as it stands,
// so that the screen and the submission it leads to answer a thread the
// community does not have in the same way.
//
// It says nothing about whether the thread is published. Unpublishing a thread
// that is already unpublished is success rather than a refusal, so that
// operation resolves its target here, while the operations acting on what the
// community still shows go through findModeratedThread.
//
// [Ja] findExistingThreadは、操作が名指すスレッドを解決し、コミュニティが持たない
// スレッドをリソースの不在として答えます。
//
// 書き込む呼び出し元は、自身の書き込みトランザクションに参加したリポジトリ (WithTx) を
// 渡さなければなりません。これが読むものが書き込むかどうかを決めるため、その間に
// スレッドが失われれば、もう存在しない行に操作が着いてしまいます。読むだけの呼び出し元
// (対象のスレッドを解決する確認ページなど) はリポジトリをそのまま渡します。画面と、
// そこから続く送信とが、コミュニティの持たないスレッドに同じ答えを返すようにするため
// です。
//
// 公開されているかどうかについては何も述べません。既に非公開のスレッドの非公開は拒否では
// なく成功であるため、その操作は対象をここで解決し、コミュニティのまだ示しているものに
// 対して行われる操作はfindModeratedThreadを通ります。
func findExistingThread(
	ctx context.Context,
	threadRepo *repository.ThreadRepository,
	threadID model.ThreadID,
) (*model.Thread, error) {
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

	return thread, nil
}

// findModeratedThread resolves the thread an operation names and confirms that
// it is one the community still shows.
//
// A caller that writes must pass a repository enlisted in its write
// transaction (WithTx), for the reason findExistingThread gives and because an
// unpublication arriving in between would otherwise let an operation land on a
// thread that was taken out of view after it was read.
//
// An unpublished thread is AppErrCodeResourceUnpublished rather than a missing
// one, because the operations that come through here act on what the community
// sees, and an unpublished thread shows nothing to lock or to close. Its posts
// are hidden by the thread's own mark, so nothing is left standing for a further
// operation to take away.
//
// [Ja] findModeratedThreadは、操作が名指すスレッドを解決し、それがコミュニティのまだ
// 示しているスレッドであることを確かめます。
//
// 書き込む呼び出し元は、自身の書き込みトランザクションに参加したリポジトリ (WithTx) を
// 渡さなければなりません。findExistingThreadが述べる理由に加え、その間に非公開が届けば、
// 読んだ後に見えない場所へ移されたスレッドに操作が着いてしまうためです。
//
// 非公開のスレッドを不在ではなくAppErrCodeResourceUnpublishedとするのは、ここを通る操作が
// コミュニティの見ているものに対して行われるためです。非公開のスレッドは、ロックする対象も
// 閉じる対象も示していません。配下の投稿もスレッド自身の印で隠れており、さらに何かを
// 取り除くために立っているものは残っていません。
func findModeratedThread(
	ctx context.Context,
	threadRepo *repository.ThreadRepository,
	threadID model.ThreadID,
) (*model.Thread, error) {
	thread, err := findExistingThread(ctx, threadRepo, threadID)
	if err != nil {
		return nil, err
	}
	if thread.UnpublishedAt != nil {
		return nil, &model.AppError{
			Code:     model.AppErrCodeResourceUnpublished,
			UserMsg:  i18n.T(ctx, "error_unpublished_message"),
			Internal: fmt.Errorf("非公開のスレッドへの操作: thread_id=%s", threadID),
			Metadata: map[string]string{"thread_id": threadID.String()},
		}
	}

	return thread, nil
}

// findExistingPost resolves the post an operation names by the pair that
// addresses it inside its thread, answering a number the thread never issued as
// a missing resource.
//
// A post is named by its thread and its reply number rather than by an id of its
// own, because that is how it is addressed everywhere it is referred to: in a
// >>N, in a shared link, and in the row of the history that records what was
// done to it (ADR 0009).
//
// It says nothing about whether the post is published. What an unpublished post
// means differs between the callers — the operation that takes it out of view
// is already done, while the screen that would name it as a target has nothing
// to show — so each answers that for itself.
//
// [Ja] findExistingPostは、操作が名指す投稿を、スレッドの中でそれを指す組で解決し、
// スレッドが一度も発行していない番号をリソースの不在として答えます。
//
// 投稿を自身のidではなくスレッドとレス番号で名指すのは、それが参照されるあらゆる場所で
// そう名指されるためです。>>Nの中でも、共有されたリンクの中でも、それに対して何が行われたか
// を記録する履歴の行の中でもです (ADR 0009)。
//
// 公開されているかどうかについては何も述べません。非公開の投稿が何を意味するかは呼び出し元に
// よって異なる (それを視界から外す操作にとっては既に済んでいることであり、それを対象として
// 名指す画面にとっては示すものが無いということである) ため、それぞれが自身で答えます。
func findExistingPost(
	ctx context.Context,
	postRepo *repository.PostRepository,
	threadID model.ThreadID,
	number int,
) (*model.Post, error) {
	post, err := postRepo.FindByThreadIDAndNumber(ctx, threadID, number)
	if err != nil {
		return nil, fmt.Errorf("投稿の取得に失敗: %w", err)
	}
	if post == nil {
		return nil, missingPost(ctx, threadID, number)
	}

	return post, nil
}

// missingPost builds the answer given for a post the thread does not show under
// the named number. A caller reaches it either because the number was never
// issued or because what stood there is no longer shown, and the two are
// answered alike: a visitor who may not be told a post exists is not told it by
// the difference between the two answers either.
//
// [Ja] missingPostは、名指された番号でスレッドが示す投稿が無いときの答えを組み立てます。
// 呼び出し元がここに至るのは、その番号が一度も発行されていない場合か、そこに立っていたものが
// もう示されていない場合であり、両者には同じ答えを返します。投稿の存在を知らされるべきでない
// 訪問者は、2つの答えの違いによってもそれを知らされません。
func missingPost(ctx context.Context, threadID model.ThreadID, number int) error {
	return &model.AppError{
		Code:     model.AppErrCodeResourceNotFound,
		UserMsg:  i18n.T(ctx, "error_not_found_message"),
		Internal: fmt.Errorf("投稿が見つからない: thread_id=%s number=%d", threadID, number),
		Metadata: map[string]string{"thread_id": threadID.String(), "number": strconv.Itoa(number)},
	}
}

// forbiddenThreadModeration builds the refusal a moderation UseCase returns when
// the actor's scopes do not admit what it names. The message a visitor sees is
// the same for every operation, while what was refused is said in the log.
//
// [Ja] forbiddenThreadModerationは、操作者のスコープが名指された操作を許さないときに
// モデレーションのUseCaseが返す拒否を組み立てます。訪問者が見る文言はどの操作でも同じで、
// 何が拒否されたのかはログの側で述べます。
func forbiddenThreadModeration(ctx context.Context, reason string, threadID model.ThreadID) error {
	return &model.AppError{
		Code:     model.AppErrCodeForbidden,
		UserMsg:  i18n.T(ctx, "error_forbidden_message"),
		Internal: fmt.Errorf("%s: thread_id=%s", reason, threadID),
		Metadata: map[string]string{"thread_id": threadID.String()},
	}
}
