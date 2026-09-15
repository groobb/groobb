package worker

import (
	"context"

	"github.com/riverqueue/river"

	"github.com/groobb/groobb/go/internal/dispatcher"
	"github.com/groobb/groobb/go/internal/usecase"
)

// SendPasswordResetWorkerはsend_password_resetジョブのRiverワーカーです。
// Handlerと同様に薄いAdapterで、ジョブ引数をUseCaseの入力に変換し、UseCaseの
// 戻り値をそのまま返します。ログ出力とリトライはRiverに任せます。
type SendPasswordResetWorker struct {
	river.WorkerDefaults[dispatcher.SendPasswordResetArgs]
	uc *usecase.SendPasswordResetUsecase
}

// NewSendPasswordResetWorkerは与えられたUseCaseを背後に持つ
// SendPasswordResetWorkerを生成します。
func NewSendPasswordResetWorker(uc *usecase.SendPasswordResetUsecase) *SendPasswordResetWorker {
	return &SendPasswordResetWorker{uc: uc}
}

// Workはジョブ引数を変換してUseCaseに委譲し、そのエラーをそのまま返します。
// これによりRiverが失敗ジョブを記録・リトライできます。
func (w *SendPasswordResetWorker) Work(ctx context.Context, job *river.Job[dispatcher.SendPasswordResetArgs]) error {
	return w.uc.Execute(ctx, usecase.SendPasswordResetInput{
		Email:    job.Args.Email,
		ResetURL: job.Args.ResetURL,
		Locale:   parseLocale(ctx, job.Args.Locale),
	})
}
