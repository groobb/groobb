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

// TestSendEmailChangeNotificationWorker_Work drives the worker's job-processing
// path (Args -> UseCase -> Sender) with a NoopSender, so it exercises the whole
// chain without River or a real Resend call. Like the other mail worker tests, the
// full River loop is not driven here because NewClient builds a real ResendSender
// internally; this test covers the Work adapter.
//
// [Ja] TestSendEmailChangeNotificationWorker_Work は NoopSender でワーカーの処理経路
// (Args -> UseCase -> Sender) を駆動し、River や実際の Resend 呼び出し無しに全体の連鎖を
// 検証する。他のメールワーカーのテストと同様、NewClient が内部で実 ResendSender を構築する
// ため River のループ全体はここでは駆動しない。本テストは Work アダプタを担う。
func TestSendEmailChangeNotificationWorker_Work(t *testing.T) {
	t.Parallel()

	noop := email.NewNoopSender()
	uc := usecase.NewSendEmailChangeNotificationUsecase(email.NewEmailChangeNotificationSender(noop))
	w := worker.NewSendEmailChangeNotificationWorker(uc)

	job := &river.Job[dispatcher.SendEmailChangeNotificationArgs]{
		Args: dispatcher.SendEmailChangeNotificationArgs{
			Email:    "old@example.dev",
			NewEmail: "new@example.dev",
			Locale:   "ja",
		},
	}

	if err := w.Work(context.Background(), job); err != nil {
		t.Fatalf("Work() error = %v", err)
	}

	if len(noop.SentEmails) != 1 {
		t.Fatalf("len(SentEmails) = %d, want 1", len(noop.SentEmails))
	}
	sent := noop.SentEmails[0]
	if sent.To != "old@example.dev" {
		t.Errorf("To = %q, want %q", sent.To, "old@example.dev")
	}
	if sent.Subject != "[Groobb] メールアドレスが変更されました" {
		t.Errorf("Subject = %q, want %q", sent.Subject, "[Groobb] メールアドレスが変更されました")
	}

	var sb strings.Builder
	if err := sent.HTMLBody.Render(context.Background(), &sb); err != nil {
		t.Fatalf("HTMLBody.Render() error = %v", err)
	}
	if !strings.Contains(sb.String(), "new@example.dev") {
		t.Error("HTML body missing the new address")
	}
}
