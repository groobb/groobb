package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/groobb/groobb/go/internal/repository"
)

// withdrawnUserRetentionは、論理削除された (退会した) ユーザーをパージジョブが
// 物理削除するまで保持する期間です。この期間は、行とそのCASCADEする子データが完全に
// 消える前の運用上のバッファ (例: 誤った / 不正な退会の調査) であり、退会はemail / atnameを
// 即座に匿名化するためユーザー向けの取り消し (undo) ではありません。当面は定数として置き、
// 必要になれば後で設定化します。
const withdrawnUserRetention = 30 * 24 * time.Hour

// PurgeWithdrawnUsersUsecaseは、退会の猶予期間を過ぎたユーザーを物理削除します。
// アカウント退会の非同期な第2段階です。退会リクエストはアカウントの論理削除と匿名化だけを
// 行い、本UseCaseが (定期バックグラウンドジョブに駆動されて) 保持期間より前に論理削除された
// ユーザーのストレージを後から回収します (子行はON DELETE CASCADEで一緒に消えます)。
type PurgeWithdrawnUsersUsecase struct {
	userRepo *repository.UserRepository
}

// NewPurgeWithdrawnUsersUsecaseは与えられたuserリポジトリを使う
// PurgeWithdrawnUsersUsecaseを生成します。
func NewPurgeWithdrawnUsersUsecase(userRepo *repository.UserRepository) *PurgeWithdrawnUsersUsecase {
	return &PurgeWithdrawnUsersUsecase{userRepo: userRepo}
}

// Executeはcutoff (現在時刻から保持期間を引いた時刻) を計算し、それより前に
// 論理削除された全ユーザーを物理削除します。単一の永続化呼び出しのためトランザクションは
// 不要で、Executeに直接書きます。ワーカーが結果をそのまま返せるようerrorのみを返し、
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
