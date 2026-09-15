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

// TestSendEmailChangeNotificationWorker_WorkはNoopSenderでワーカーの処理経路
// (Args -> UseCase -> Sender) を駆動し、Riverや実際のResend呼び出し無しに全体の連鎖を
// 検証する。他のメールワーカーのテストと同様、NewClientが内部で実ResendSenderを構築する
// ためRiverのループ全体はここでは駆動しない。本テストはWorkアダプタを担う。
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
		t.Fatalf("Work()のエラー = %v", err)
	}

	if len(noop.SentEmails) != 1 {
		t.Fatalf("len(SentEmails) = %d、期待値 = 1", len(noop.SentEmails))
	}
	sent := noop.SentEmails[0]
	if sent.To != "old@example.dev" {
		t.Errorf("To = %q、期待値 = %q", sent.To, "old@example.dev")
	}
	if sent.Subject != "[Groobb] メールアドレスが変更されました" {
		t.Errorf("Subject = %q、期待値 = %q", sent.Subject, "[Groobb] メールアドレスが変更されました")
	}

	var sb strings.Builder
	if err := sent.HTMLBody.Render(context.Background(), &sb); err != nil {
		t.Fatalf("HTMLBody.Render()のエラー = %v", err)
	}
	if !strings.Contains(sb.String(), "new@example.dev") {
		t.Error("HTML本文に新しいアドレスが含まれていない")
	}
}
