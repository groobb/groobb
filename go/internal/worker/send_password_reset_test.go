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

// TestSendPasswordResetWorker_WorkはNoopSenderでワーカーの処理経路
// (Args -> UseCase -> Sender) を駆動し、Riverや実際のResend呼び出し無しに全体の連鎖を
// 検証する。確認ワーカーのテストと同様、NewClientが内部で実ResendSenderを構築するため
// Riverのループ全体はここでは駆動しない。本テストはWorkアダプタを担う。
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
		t.Fatalf("Work()のエラー = %v", err)
	}

	if len(noop.SentEmails) != 1 {
		t.Fatalf("len(SentEmails) = %d、期待値 = 1", len(noop.SentEmails))
	}
	sent := noop.SentEmails[0]
	if sent.To != "user@example.dev" {
		t.Errorf("To = %q、期待値 = %q", sent.To, "user@example.dev")
	}
	if sent.Subject != "[Groobb] パスワードの再設定" {
		t.Errorf("Subject = %q、期待値 = %q", sent.Subject, "[Groobb] パスワードの再設定")
	}

	var sb strings.Builder
	if err := sent.HTMLBody.Render(context.Background(), &sb); err != nil {
		t.Fatalf("HTMLBody.Render()のエラー = %v", err)
	}
	if !strings.Contains(sb.String(), resetURL) {
		t.Error("HTML本文にリセットURLが含まれていない")
	}
}
