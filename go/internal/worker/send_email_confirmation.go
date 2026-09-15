package worker

import (
	"context"

	"github.com/riverqueue/river"

	"github.com/groobb/groobb/go/internal/dispatcher"
	"github.com/groobb/groobb/go/internal/usecase"
)

// SendEmailConfirmationWorkerはsend_email_confirmationジョブのRiverワーカー
// です。Handlerと同様に薄いAdapterで、ジョブ引数をUseCaseの入力に変換し、UseCaseの
// 戻り値をそのまま返します。ログ出力とリトライはRiverに任せます。
type SendEmailConfirmationWorker struct {
	river.WorkerDefaults[dispatcher.SendEmailConfirmationArgs]
	uc *usecase.SendEmailConfirmationUsecase
}

// NewSendEmailConfirmationWorkerは与えられたUseCaseを背後に持つ
// SendEmailConfirmationWorkerを生成します。
func NewSendEmailConfirmationWorker(uc *usecase.SendEmailConfirmationUsecase) *SendEmailConfirmationWorker {
	return &SendEmailConfirmationWorker{uc: uc}
}

// Workはジョブ引数を変換してUseCaseに委譲し、そのエラーをそのまま返します。
// これによりRiverが失敗ジョブを記録・リトライできます。
func (w *SendEmailConfirmationWorker) Work(ctx context.Context, job *river.Job[dispatcher.SendEmailConfirmationArgs]) error {
	return w.uc.Execute(ctx, usecase.SendEmailConfirmationInput{
		Email:  job.Args.Email,
		Code:   job.Args.Code,
		Locale: parseLocale(ctx, job.Args.Locale),
	})
}
