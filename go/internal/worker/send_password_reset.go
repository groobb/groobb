package worker

import (
	"context"

	"github.com/riverqueue/river"

	"github.com/groobb/groobb/go/internal/dispatcher"
	"github.com/groobb/groobb/go/internal/usecase"
)

// SendPasswordResetWorker is the River worker for the send_password_reset job.
// Like a Handler it is a thin adapter: it converts the job arguments into a
// UseCase input and returns the UseCase's result unchanged, leaving logging and
// retries to River.
//
// [Ja] SendPasswordResetWorker は send_password_reset ジョブの River ワーカーです。
// Handler と同様に薄い Adapter で、ジョブ引数を UseCase の入力に変換し、UseCase の
// 戻り値をそのまま返します。ログ出力とリトライは River に任せます。
type SendPasswordResetWorker struct {
	river.WorkerDefaults[dispatcher.SendPasswordResetArgs]
	uc *usecase.SendPasswordResetUsecase
}

// NewSendPasswordResetWorker builds a SendPasswordResetWorker backed by the given
// UseCase.
//
// [Ja] NewSendPasswordResetWorker は与えられた UseCase を背後に持つ
// SendPasswordResetWorker を生成します。
func NewSendPasswordResetWorker(uc *usecase.SendPasswordResetUsecase) *SendPasswordResetWorker {
	return &SendPasswordResetWorker{uc: uc}
}

// Work converts the job arguments and delegates to the UseCase, returning its
// error verbatim so River can record and retry a failed job.
//
// [Ja] Work はジョブ引数を変換して UseCase に委譲し、そのエラーをそのまま返します。
// これにより River が失敗ジョブを記録・リトライできます。
func (w *SendPasswordResetWorker) Work(ctx context.Context, job *river.Job[dispatcher.SendPasswordResetArgs]) error {
	return w.uc.Execute(ctx, usecase.SendPasswordResetInput{
		Email:    job.Args.Email,
		ResetURL: job.Args.ResetURL,
		Locale:   parseLocale(ctx, job.Args.Locale),
	})
}
