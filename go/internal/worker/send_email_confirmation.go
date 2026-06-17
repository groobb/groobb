package worker

import (
	"context"

	"github.com/riverqueue/river"

	"github.com/groobb/groobb/go/internal/dispatcher"
	"github.com/groobb/groobb/go/internal/usecase"
)

// SendEmailConfirmationWorker is the River worker for the send_email_confirmation
// job. Like a Handler it is a thin adapter: it converts the job arguments into a
// UseCase input and returns the UseCase's result unchanged, leaving logging and
// retries to River.
//
// [Ja] SendEmailConfirmationWorker は send_email_confirmation ジョブの River ワーカー
// です。Handler と同様に薄い Adapter で、ジョブ引数を UseCase の入力に変換し、UseCase の
// 戻り値をそのまま返します。ログ出力とリトライは River に任せます。
type SendEmailConfirmationWorker struct {
	river.WorkerDefaults[dispatcher.SendEmailConfirmationArgs]
	uc *usecase.SendEmailConfirmationUsecase
}

// NewSendEmailConfirmationWorker builds a SendEmailConfirmationWorker backed by
// the given UseCase.
//
// [Ja] NewSendEmailConfirmationWorker は与えられた UseCase を背後に持つ
// SendEmailConfirmationWorker を生成します。
func NewSendEmailConfirmationWorker(uc *usecase.SendEmailConfirmationUsecase) *SendEmailConfirmationWorker {
	return &SendEmailConfirmationWorker{uc: uc}
}

// Work converts the job arguments and delegates to the UseCase, returning its
// error verbatim so River can record and retry a failed job.
//
// [Ja] Work はジョブ引数を変換して UseCase に委譲し、そのエラーをそのまま返します。
// これにより River が失敗ジョブを記録・リトライできます。
func (w *SendEmailConfirmationWorker) Work(ctx context.Context, job *river.Job[dispatcher.SendEmailConfirmationArgs]) error {
	return w.uc.Execute(ctx, usecase.SendEmailConfirmationInput{
		Email:  job.Args.Email,
		Code:   job.Args.Code,
		Locale: job.Args.Locale,
	})
}
