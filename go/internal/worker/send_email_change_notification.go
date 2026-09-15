package worker

import (
	"context"

	"github.com/riverqueue/river"

	"github.com/groobb/groobb/go/internal/dispatcher"
	"github.com/groobb/groobb/go/internal/usecase"
)

// SendEmailChangeNotificationWorkerはsend_email_change_notificationジョブの
// Riverワーカーです。Handlerと同様に薄いAdapterで、ジョブ引数をUseCaseの入力に
// 変換し、UseCaseの戻り値をそのまま返します。ログ出力とリトライはRiverに任せます。
type SendEmailChangeNotificationWorker struct {
	river.WorkerDefaults[dispatcher.SendEmailChangeNotificationArgs]
	uc *usecase.SendEmailChangeNotificationUsecase
}

// NewSendEmailChangeNotificationWorkerは与えられたUseCaseを背後に持つ
// SendEmailChangeNotificationWorkerを生成します。
func NewSendEmailChangeNotificationWorker(uc *usecase.SendEmailChangeNotificationUsecase) *SendEmailChangeNotificationWorker {
	return &SendEmailChangeNotificationWorker{uc: uc}
}

// Workはジョブ引数を変換してUseCaseに委譲し、そのエラーをそのまま返します。
// これによりRiverが失敗ジョブを記録・リトライできます。
func (w *SendEmailChangeNotificationWorker) Work(ctx context.Context, job *river.Job[dispatcher.SendEmailChangeNotificationArgs]) error {
	return w.uc.Execute(ctx, usecase.SendEmailChangeNotificationInput{
		Email:    job.Args.Email,
		NewEmail: job.Args.NewEmail,
		Locale:   parseLocale(ctx, job.Args.Locale),
	})
}
