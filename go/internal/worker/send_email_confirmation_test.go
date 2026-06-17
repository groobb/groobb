package worker_test

import (
	"context"
	"strings"
	"testing"

	"github.com/riverqueue/river"

	"github.com/groobb/groobb/go/internal/dispatcher"
	"github.com/groobb/groobb/go/internal/email"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/worker"
)

// TestSendEmailConfirmationWorker_Work drives the worker's job-processing path
// (Args -> UseCase -> Sender) with a NoopSender, so it exercises the whole chain
// without River or a real Resend call. The full River Start -> fetch -> Work loop
// is not driven here because NewClient builds a real ResendSender internally,
// which would attempt a network send; this test covers the Work adapter, and the
// client test covers Start/Stop lifecycle.
//
// [Ja] TestSendEmailConfirmationWorker_Work は NoopSender でワーカーの処理経路
// (Args -> UseCase -> Sender) を駆動し、River や実際の Resend 呼び出し無しに全体の連鎖を
// 検証する。River の Start -> 取得 -> Work ループ全体はここでは駆動しない。NewClient は
// 内部で実 ResendSender を構築し、ネットワーク送信を試みてしまうためである。本テストは
// Work アダプタを、クライアントテストが Start/Stop のライフサイクルをそれぞれ担う。
func TestSendEmailConfirmationWorker_Work(t *testing.T) {
	t.Parallel()

	noop := email.NewNoopSender()
	uc := usecase.NewSendEmailConfirmationUsecase(email.NewConfirmationSender(noop))
	w := worker.NewSendEmailConfirmationWorker(uc)

	job := &river.Job[dispatcher.SendEmailConfirmationArgs]{
		Args: dispatcher.SendEmailConfirmationArgs{
			Email:  "user@example.dev",
			Code:   "135790",
			Locale: "ja",
		},
	}

	if err := w.Work(context.Background(), job); err != nil {
		t.Fatalf("Work() error = %v", err)
	}

	if len(noop.SentEmails) != 1 {
		t.Fatalf("len(SentEmails) = %d, want 1", len(noop.SentEmails))
	}
	sent := noop.SentEmails[0]
	if sent.To != "user@example.dev" {
		t.Errorf("To = %q, want %q", sent.To, "user@example.dev")
	}
	if sent.Subject != "[Groobb] 確認用コード" {
		t.Errorf("Subject = %q, want %q", sent.Subject, "[Groobb] 確認用コード")
	}

	var sb strings.Builder
	if err := sent.HTMLBody.Render(context.Background(), &sb); err != nil {
		t.Fatalf("HTMLBody.Render() error = %v", err)
	}
	if !strings.Contains(sb.String(), "135790") {
		t.Error("HTML body missing the code")
	}
}
