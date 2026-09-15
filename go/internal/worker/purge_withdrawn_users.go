package worker

import (
	"context"

	"github.com/riverqueue/river"

	"github.com/groobb/groobb/go/internal/dispatcher"
	"github.com/groobb/groobb/go/internal/usecase"
)

// PurgeWithdrawnUsersWorkerはpurge_withdrawn_usersジョブのRiverワーカーです。
// Handlerと同様に薄いAdapterで、ジョブは引数を持たないためUseCaseに委譲してそのエラーを
// そのまま返すだけです。ログ出力とリトライはRiverに任せます。
type PurgeWithdrawnUsersWorker struct {
	river.WorkerDefaults[dispatcher.PurgeWithdrawnUsersArgs]
	uc *usecase.PurgeWithdrawnUsersUsecase
}

// NewPurgeWithdrawnUsersWorkerは与えられたUseCaseを背後に持つ
// PurgeWithdrawnUsersWorkerを生成します。
func NewPurgeWithdrawnUsersWorker(uc *usecase.PurgeWithdrawnUsersUsecase) *PurgeWithdrawnUsersWorker {
	return &PurgeWithdrawnUsersWorker{uc: uc}
}

// WorkはUseCaseに委譲し、そのエラーをそのまま返します。これによりRiverが
// 失敗ジョブを記録・リトライできます。
func (w *PurgeWithdrawnUsersWorker) Work(ctx context.Context, _ *river.Job[dispatcher.PurgeWithdrawnUsersArgs]) error {
	return w.uc.Execute(ctx)
}
