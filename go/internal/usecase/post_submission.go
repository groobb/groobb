package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// verifyPostAuthor decides whether userID is still an account that may write a
// post at all. Starting a thread and replying to one both ask it, so an account
// that has left is refused however the post was written.
//
// It is asked separately from verifyPostInterval, which the same callers ask
// afterwards, because a caller may have something of its own to ask in between.
// Replying reads the thread's own refusals there, so an account that can post
// nowhere is told that rather than being told about the thread it happened to
// write to.
//
// The repository must be enlisted in the caller's transaction (WithTx), because
// the answer holds only while the write lock it was read under is still held.
//
// [Ja] verifyPostAuthor は、userID がまだ投稿を書けるアカウントであるかを判断します。
// スレッドを立てる場合も返信する場合もここを通るため、去ったアカウントはどう書いても
// 拒否されます。
//
// 同じ呼び出し元がこの後に問う verifyPostInterval と分けているのは、呼び出し元がその間に
// 自身の問いを持ちうるためです。返信はそこでスレッド自身の拒否理由を読みます。どこにも
// 投稿できないアカウントに、たまたま書き込んだスレッドの話ではなく、そのことを伝えるため
// です。
//
// リポジトリは呼び出し元のトランザクションに参加したもの (WithTx) でなければなりません。
// ここでの答えが有効なのは、それを読んだ書き込みロックを保持している間だけであるためです。
func verifyPostAuthor(ctx context.Context, userRepo *repository.UserRepository, userID model.UserID) error {
	poster, err := userRepo.FindByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("投稿者の取得に失敗: %w", err)
	}
	// The account is gone from the lookup once it has withdrawn. A session for it
	// is refused the same way, so this is reached by one that withdrew while it
	// was signed in, and the post is refused rather than being attributed to an
	// account that has left.
	//
	// [Ja] アカウントは退会するとルックアップから外れる。そのアカウントのセッションも
	// 同じく拒否されるため、ここに至るのはサインイン中に退会した場合であり、去った
	// アカウントに投稿を帰属させずに拒否する。
	if poster == nil {
		return &model.AppError{
			Code:     model.AppErrCodeForbidden,
			UserMsg:  i18n.T(ctx, "validation_post_account_unavailable"),
			Internal: fmt.Errorf("投稿できるアカウントが見つからない: user_id=%s", userID),
			Metadata: map[string]string{"user_id": userID.String()},
		}
	}

	return nil
}

// verifyPostInterval decides whether the interval since userID's last post has
// run out at now. Starting a thread and replying to one both ask it, so a
// person's posts are spaced the same however they are written, and a thread just
// started cannot be replied to any sooner than any other.
//
// The repository must be enlisted in the caller's transaction (WithTx), because
// the answer holds only while the write lock it was read under is still held.
// Read outside it, the latest post is what it was before the waiting writer got
// its turn, and two submissions racing for the same interval would both be told
// to go ahead.
//
// [Ja] verifyPostInterval は、userID の最後の投稿からの間隔が now の時点で尽きているかを
// 判断します。スレッドを立てる場合も返信する場合もここを通るため、1 人の投稿はどう書かれても
// 同じだけ間隔が空き、立てたばかりのスレッドへの返信も他と同じだけ待つことになります。
//
// リポジトリは呼び出し元のトランザクションに参加したもの (WithTx) でなければなりません。
// ここでの答えが有効なのは、それを読んだ書き込みロックを保持している間だけであるためです。
// その外で読めば、最新の投稿は待っていた書き手が順番を得る前の姿であり、同じ間隔を奪い合う
// 2 つの送信の両方が「進んでよい」と告げられます。
func verifyPostInterval(
	ctx context.Context,
	postRepo *repository.PostRepository,
	userID model.UserID,
	now time.Time,
) error {
	latest, err := postRepo.FindLatestByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("投稿者の最新投稿の取得に失敗: %w", err)
	}
	if latest == nil {
		return nil
	}

	wait := model.PostIntervalWait(latest.CreatedAt, now)
	if wait == 0 {
		return nil
	}

	return &model.AppError{
		Code:       model.AppErrCodeRateLimited,
		UserMsg:    i18n.T(ctx, "validation_post_interval_too_short", map[string]any{"Count": int(wait.Seconds())}),
		Internal:   fmt.Errorf("前の投稿からの間隔が足りない: user_id=%s wait=%s", userID, wait),
		Metadata:   map[string]string{"user_id": userID.String()},
		RetryAfter: wait,
	}
}
