package worker

import (
	"context"

	"github.com/riverqueue/river"

	"github.com/groobb/groobb/go/internal/dispatcher"
	"github.com/groobb/groobb/go/internal/usecase"
)

// PurgeWithdrawnUsersWorker is the River worker for the purge_withdrawn_users job.
// Like a Handler it is a thin adapter: the job carries no arguments, so it just
// delegates to the UseCase and returns its error unchanged, leaving logging and
// retries to River.
//
// [Ja] PurgeWithdrawnUsersWorker は purge_withdrawn_users ジョブの River ワーカーです。
// Handler と同様に薄い Adapter で、ジョブは引数を持たないため UseCase に委譲してそのエラーを
// そのまま返すだけです。ログ出力とリトライは River に任せます。
type PurgeWithdrawnUsersWorker struct {
	river.WorkerDefaults[dispatcher.PurgeWithdrawnUsersArgs]
	uc *usecase.PurgeWithdrawnUsersUsecase
}

// NewPurgeWithdrawnUsersWorker builds a PurgeWithdrawnUsersWorker backed by the
// given UseCase.
//
// [Ja] NewPurgeWithdrawnUsersWorker は与えられた UseCase を背後に持つ
// PurgeWithdrawnUsersWorker を生成します。
func NewPurgeWithdrawnUsersWorker(uc *usecase.PurgeWithdrawnUsersUsecase) *PurgeWithdrawnUsersWorker {
	return &PurgeWithdrawnUsersWorker{uc: uc}
}

// Work delegates to the UseCase and returns its error verbatim so River can record
// and retry a failed job.
//
// [Ja] Work は UseCase に委譲し、そのエラーをそのまま返します。これにより River が
// 失敗ジョブを記録・リトライできます。
func (w *PurgeWithdrawnUsersWorker) Work(ctx context.Context, _ *river.Job[dispatcher.PurgeWithdrawnUsersArgs]) error {
	return w.uc.Execute(ctx)
}
