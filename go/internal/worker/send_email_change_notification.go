package worker

import (
	"context"

	"github.com/riverqueue/river"

	"github.com/groobb/groobb/go/internal/dispatcher"
	"github.com/groobb/groobb/go/internal/usecase"
)

// SendEmailChangeNotificationWorker is the River worker for the
// send_email_change_notification job. Like a Handler it is a thin adapter: it
// converts the job arguments into a UseCase input and returns the UseCase's result
// unchanged, leaving logging and retries to River.
//
// [Ja] SendEmailChangeNotificationWorker は send_email_change_notification ジョブの
// River ワーカーです。Handler と同様に薄い Adapter で、ジョブ引数を UseCase の入力に
// 変換し、UseCase の戻り値をそのまま返します。ログ出力とリトライは River に任せます。
type SendEmailChangeNotificationWorker struct {
	river.WorkerDefaults[dispatcher.SendEmailChangeNotificationArgs]
	uc *usecase.SendEmailChangeNotificationUsecase
}

// NewSendEmailChangeNotificationWorker builds a SendEmailChangeNotificationWorker
// backed by the given UseCase.
//
// [Ja] NewSendEmailChangeNotificationWorker は与えられた UseCase を背後に持つ
// SendEmailChangeNotificationWorker を生成します。
func NewSendEmailChangeNotificationWorker(uc *usecase.SendEmailChangeNotificationUsecase) *SendEmailChangeNotificationWorker {
	return &SendEmailChangeNotificationWorker{uc: uc}
}

// Work converts the job arguments and delegates to the UseCase, returning its
// error verbatim so River can record and retry a failed job.
//
// [Ja] Work はジョブ引数を変換して UseCase に委譲し、そのエラーをそのまま返します。
// これにより River が失敗ジョブを記録・リトライできます。
func (w *SendEmailChangeNotificationWorker) Work(ctx context.Context, job *river.Job[dispatcher.SendEmailChangeNotificationArgs]) error {
	return w.uc.Execute(ctx, usecase.SendEmailChangeNotificationInput{
		Email:    job.Args.Email,
		NewEmail: job.Args.NewEmail,
		Locale:   parseLocale(ctx, job.Args.Locale),
	})
}
