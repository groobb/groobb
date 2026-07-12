package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/groobb/groobb/go/internal/repository"
)

// withdrawnUserRetention is how long a soft-deleted (withdrawn) user is kept
// before the purge job physically deletes it. The window is an operational buffer
// (e.g. to investigate a mistaken or fraudulent withdrawal) before the row and its
// cascading children are removed for good; it is not a user-facing undo, since
// withdrawal anonymizes email/atname immediately. Kept as a constant for now, to be
// made configurable later if needed.
//
// [Ja] withdrawnUserRetention は、論理削除された (退会した) ユーザーをパージジョブが
// 物理削除するまで保持する期間です。この期間は、行とその CASCADE する子データが完全に
// 消える前の運用上のバッファ (例: 誤った / 不正な退会の調査) であり、退会は email / atname を
// 即座に匿名化するためユーザー向けの取り消し (undo) ではありません。当面は定数として置き、
// 必要になれば後で設定化します。
const withdrawnUserRetention = 30 * 24 * time.Hour

// PurgeWithdrawnUsersUsecase physically deletes users whose withdrawal grace
// period has elapsed. It is the asynchronous second stage of account withdrawal:
// the withdrawal request only soft-deletes and anonymizes the account, and this
// UseCase — driven by a periodic background job — later reclaims the storage for
// users soft-deleted longer ago than the retention window (their child rows go via
// ON DELETE CASCADE).
//
// [Ja] PurgeWithdrawnUsersUsecase は、退会の猶予期間を過ぎたユーザーを物理削除します。
// アカウント退会の非同期な第 2 段階です。退会リクエストはアカウントの論理削除と匿名化だけを
// 行い、本 UseCase が (定期バックグラウンドジョブに駆動されて) 保持期間より前に論理削除された
// ユーザーのストレージを後から回収します (子行は ON DELETE CASCADE で一緒に消えます)。
type PurgeWithdrawnUsersUsecase struct {
	userRepo *repository.UserRepository
}

// NewPurgeWithdrawnUsersUsecase builds a PurgeWithdrawnUsersUsecase over the given
// user repository.
//
// [Ja] NewPurgeWithdrawnUsersUsecase は与えられた user リポジトリを使う
// PurgeWithdrawnUsersUsecase を生成します。
func NewPurgeWithdrawnUsersUsecase(userRepo *repository.UserRepository) *PurgeWithdrawnUsersUsecase {
	return &PurgeWithdrawnUsersUsecase{userRepo: userRepo}
}

// Execute computes the cutoff (now minus the retention window) and physically
// deletes every user soft-deleted before it. This is a single persistence call, so
// it needs no transaction and is written directly in Execute. It returns only an
// error so the worker can pass the result through verbatim; the deleted count is
// logged here for operability rather than returned.
//
// [Ja] Execute は cutoff (現在時刻から保持期間を引いた時刻) を計算し、それより前に
// 論理削除された全ユーザーを物理削除します。単一の永続化呼び出しのためトランザクションは
// 不要で、Execute に直接書きます。ワーカーが結果をそのまま返せるよう error のみを返し、
// 削除件数は返さずに運用のためここでログ出力します。
func (uc *PurgeWithdrawnUsersUsecase) Execute(ctx context.Context) error {
	cutoff := time.Now().Add(-withdrawnUserRetention)

	count, err := uc.userRepo.PurgeDeletedBefore(ctx, cutoff)
	if err != nil {
		return fmt.Errorf("退会済みユーザーの物理削除に失敗: %w", err)
	}

	slog.InfoContext(ctx, "退会済みユーザーを物理削除しました", "count", count)
	return nil
}
