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

// TestSendPasswordResetWorker_Work drives the worker's job-processing path
// (Args -> UseCase -> Sender) with a NoopSender, so it exercises the whole chain
// without River or a real Resend call. Like the confirmation worker test, the
// full River loop is not driven here because NewClient builds a real ResendSender
// internally; this test covers the Work adapter.
//
// [Ja] TestSendPasswordResetWorker_Work は NoopSender でワーカーの処理経路
// (Args -> UseCase -> Sender) を駆動し、River や実際の Resend 呼び出し無しに全体の連鎖を
// 検証する。確認ワーカーのテストと同様、NewClient が内部で実 ResendSender を構築するため
// River のループ全体はここでは駆動しない。本テストは Work アダプタを担う。
func TestSendPasswordResetWorker_Work(t *testing.T) {
	t.Parallel()

	const resetURL = "https://groobb.example.dev/password/edit?token=opaque-token"

	noop := email.NewNoopSender()
	uc := usecase.NewSendPasswordResetUsecase(email.NewPasswordResetSender(noop))
	w := worker.NewSendPasswordResetWorker(uc)

	job := &river.Job[dispatcher.SendPasswordResetArgs]{
		Args: dispatcher.SendPasswordResetArgs{
			Email:    "user@example.dev",
			ResetURL: resetURL,
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
	if sent.To != "user@example.dev" {
		t.Errorf("To = %q, want %q", sent.To, "user@example.dev")
	}
	if sent.Subject != "[Groobb] パスワードの再設定" {
		t.Errorf("Subject = %q, want %q", sent.Subject, "[Groobb] パスワードの再設定")
	}

	var sb strings.Builder
	if err := sent.HTMLBody.Render(context.Background(), &sb); err != nil {
		t.Fatalf("HTMLBody.Render() error = %v", err)
	}
	if !strings.Contains(sb.String(), resetURL) {
		t.Error("HTML body missing the reset URL")
	}
}
